use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::alloc::{alloc, dealloc, Layout};
use std::ptr;
use std::slice;

#[derive(Deserialize)]
struct Input {
    operation: String,
    data: String,
    #[serde(default)]
    path: Option<String>,
    #[serde(default)]
    value: Option<String>,
}

#[derive(Serialize)]
struct Output {
    result: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    valid: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    data: Option<Value>,
}

#[derive(Serialize)]
struct ErrorOutput {
    error: String,
}

static mut RESULT_PTR: *mut u8 = ptr::null_mut();
static mut RESULT_LEN: usize = 0;

#[no_mangle]
pub extern "C" fn alloc_memory(size: usize) -> *mut u8 {
    let layout = Layout::array::<u8>(size).unwrap();
    unsafe { alloc(layout) }
}

#[no_mangle]
pub extern "C" fn dealloc_memory(ptr: *mut u8, size: usize) {
    let layout = Layout::array::<u8>(size).unwrap();
    unsafe { dealloc(ptr, layout) }
}

#[no_mangle]
pub extern "C" fn get_result_ptr() -> *const u8 {
    unsafe { RESULT_PTR }
}

#[no_mangle]
pub extern "C" fn get_result_len() -> usize {
    unsafe { RESULT_LEN }
}

#[no_mangle]
pub extern "C" fn execute(input_ptr: *const u8, input_len: usize) -> i32 {
    let input_bytes = unsafe { slice::from_raw_parts(input_ptr, input_len) };
    let input_str = match std::str::from_utf8(input_bytes) {
        Ok(s) => s,
        Err(_) => return error_response("Invalid UTF-8 input"),
    };

    let input: Input = match serde_json::from_str(input_str) {
        Ok(i) => i,
        Err(e) => return error_response(&format!("JSON parse error: {}", e)),
    };

    let output = match input.operation.as_str() {
        "validate" => {
            // Validate JSON syntax
            match serde_json::from_str::<Value>(&input.data) {
                Ok(_) => Output {
                    result: "valid".to_string(),
                    valid: Some(true),
                    data: None,
                },
                Err(e) => Output {
                    result: format!("invalid: {}", e),
                    valid: Some(false),
                    data: None,
                },
            }
        },
        "parse" => {
            // Parse and return structured data
            match serde_json::from_str::<Value>(&input.data) {
                Ok(parsed) => Output {
                    result: "parsed".to_string(),
                    valid: Some(true),
                    data: Some(parsed),
                },
                Err(e) => return error_response(&format!("Parse error: {}", e)),
            }
        },
        "prettify" => {
            // Format JSON with indentation
            match serde_json::from_str::<Value>(&input.data) {
                Ok(parsed) => {
                    match serde_json::to_string_pretty(&parsed) {
                        Ok(pretty) => Output {
                            result: pretty,
                            valid: Some(true),
                            data: None,
                        },
                        Err(e) => return error_response(&format!("Prettify error: {}", e)),
                    }
                },
                Err(e) => return error_response(&format!("Parse error: {}", e)),
            }
        },
        "minify" => {
            // Minify JSON (remove whitespace)
            match serde_json::from_str::<Value>(&input.data) {
                Ok(parsed) => {
                    match serde_json::to_string(&parsed) {
                        Ok(minified) => Output {
                            result: minified,
                            valid: Some(true),
                            data: None,
                        },
                        Err(e) => return error_response(&format!("Minify error: {}", e)),
                    }
                },
                Err(e) => return error_response(&format!("Parse error: {}", e)),
            }
        },
        "get" => {
            // Get value at JSON path
            match serde_json::from_str::<Value>(&input.data) {
                Ok(mut parsed) => {
                    if let Some(path) = &input.path {
                        // Simple path navigation (e.g., "user.name" or "items[0]")
                        let result_value = navigate_path(&parsed, path);
                        match result_value {
                            Some(val) => Output {
                                result: val.to_string(),
                                valid: Some(true),
                                data: Some(val.clone()),
                            },
                            None => return error_response(&format!("Path not found: {}", path)),
                        }
                    } else {
                        return error_response("Path required for 'get' operation");
                    }
                },
                Err(e) => return error_response(&format!("Parse error: {}", e)),
            }
        },
        "keys" => {
            // Get all keys from JSON object
            match serde_json::from_str::<Value>(&input.data) {
                Ok(parsed) => {
                    if let Value::Object(map) = parsed {
                        let keys: Vec<String> = map.keys().cloned().collect();
                        Output {
                            result: keys.join(", "),
                            valid: Some(true),
                            data: Some(Value::Array(keys.into_iter().map(Value::String).collect())),
                        }
                    } else {
                        return error_response("Input must be a JSON object");
                    }
                },
                Err(e) => return error_response(&format!("Parse error: {}", e)),
            }
        },
        "type" => {
            // Get type of JSON value
            match serde_json::from_str::<Value>(&input.data) {
                Ok(parsed) => {
                    let type_name = match &parsed {
                        Value::Null => "null",
                        Value::Bool(_) => "boolean",
                        Value::Number(_) => "number",
                        Value::String(_) => "string",
                        Value::Array(_) => "array",
                        Value::Object(_) => "object",
                    };
                    Output {
                        result: type_name.to_string(),
                        valid: Some(true),
                        data: None,
                    }
                },
                Err(e) => return error_response(&format!("Parse error: {}", e)),
            }
        },
        _ => return error_response(&format!("Unknown operation: {}", input.operation)),
    };

    // Serialize output
    let output_json = match serde_json::to_string(&output) {
        Ok(j) => j,
        Err(e) => return error_response(&format!("JSON serialize error: {}", e)),
    };

    store_result(&output_json);
    0
}

// Navigate JSON path (simplified JSONPath)
fn navigate_path<'a>(value: &'a Value, path: &str) -> Option<&'a Value> {
    let mut current = value;
    for part in path.split('.') {
        // Handle array indices
        if part.contains('[') && part.ends_with(']') {
            let key = &part[..part.find('[').unwrap()];
            if !key.is_empty() {
                current = current.get(key)?;
            }
            // Parse index
            let index_str = &part[part.find('[').unwrap() + 1..part.len() - 1];
            let index: usize = index_str.parse().ok()?;
            current = current.get(index)?;
        } else {
            current = current.get(part)?;
        }
    }
    Some(current)
}

fn store_result(data: &str) {
    let bytes = data.as_bytes();
    let len = bytes.len();
    let layout = Layout::array::<u8>(len).unwrap();
    let ptr = unsafe { alloc(layout) };

    unsafe {
        ptr::copy_nonoverlapping(bytes.as_ptr(), ptr, len);
        RESULT_PTR = ptr;
        RESULT_LEN = len;
    }
}

fn error_response(message: &str) -> i32 {
    let error = ErrorOutput {
        error: message.to_string(),
    };
    let output_json = serde_json::to_string(&error).unwrap_or_else(|_| {
        r#"{"error":"Failed to serialize error"}"#.to_string()
    });

    store_result(&output_json);
    1
}
