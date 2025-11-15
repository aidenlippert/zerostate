use serde::{Deserialize, Serialize};
use std::alloc::{alloc, dealloc, Layout};
use std::ptr;
use std::slice;

#[derive(Deserialize)]
struct Input {
    operation: String,
    text: String,
    #[serde(default)]
    separator: Option<String>,
}

#[derive(Serialize)]
struct Output {
    result: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    parts: Option<Vec<String>>,
}

#[derive(Serialize)]
struct ErrorOutput {
    error: String,
}

// Memory management functions
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

// Main execution function
#[no_mangle]
pub extern "C" fn execute(input_ptr: *const u8, input_len: usize) -> i32 {
    // Read input
    let input_bytes = unsafe { slice::from_raw_parts(input_ptr, input_len) };
    let input_str = match std::str::from_utf8(input_bytes) {
        Ok(s) => s,
        Err(_) => return error_response("Invalid UTF-8 input"),
    };

    // Parse JSON input
    let input: Input = match serde_json::from_str(input_str) {
        Ok(i) => i,
        Err(e) => return error_response(&format!("JSON parse error: {}", e)),
    };

    // Execute operation
    let output = match input.operation.as_str() {
        "uppercase" => Output {
            result: input.text.to_uppercase(),
            parts: None,
        },
        "lowercase" => Output {
            result: input.text.to_lowercase(),
            parts: None,
        },
        "reverse" => Output {
            result: input.text.chars().rev().collect(),
            parts: None,
        },
        "trim" => Output {
            result: input.text.trim().to_string(),
            parts: None,
        },
        "length" => Output {
            result: input.text.len().to_string(),
            parts: None,
        },
        "split" => {
            let separator = input.separator.as_deref().unwrap_or(" ");
            let parts: Vec<String> = input.text.split(separator).map(|s| s.to_string()).collect();
            Output {
                result: format!("{} parts", parts.len()),
                parts: Some(parts),
            }
        },
        "capitalize" => {
            let mut chars = input.text.chars();
            let result = match chars.next() {
                None => String::new(),
                Some(first) => first.to_uppercase().collect::<String>() + chars.as_str(),
            };
            Output {
                result,
                parts: None,
            }
        },
        "title_case" => {
            let result = input.text
                .split_whitespace()
                .map(|word| {
                    let mut chars = word.chars();
                    match chars.next() {
                        None => String::new(),
                        Some(first) => first.to_uppercase().collect::<String>() + chars.as_str(),
                    }
                })
                .collect::<Vec<_>>()
                .join(" ");
            Output {
                result,
                parts: None,
            }
        },
        "remove_whitespace" => Output {
            result: input.text.chars().filter(|c| !c.is_whitespace()).collect(),
            parts: None,
        },
        "count_words" => Output {
            result: input.text.split_whitespace().count().to_string(),
            parts: None,
        },
        _ => return error_response(&format!("Unknown operation: {}", input.operation)),
    };

    // Serialize output
    let output_json = match serde_json::to_string(&output) {
        Ok(j) => j,
        Err(e) => return error_response(&format!("JSON serialize error: {}", e)),
    };

    // Store result in global memory
    let bytes = output_json.as_bytes();
    let len = bytes.len();
    let layout = Layout::array::<u8>(len).unwrap();
    let ptr = unsafe { alloc(layout) };

    unsafe {
        ptr::copy_nonoverlapping(bytes.as_ptr(), ptr, len);
        RESULT_PTR = ptr;
        RESULT_LEN = len;
    }

    0 // Success
}

fn error_response(message: &str) -> i32 {
    let error = ErrorOutput {
        error: message.to_string(),
    };
    let output_json = serde_json::to_string(&error).unwrap_or_else(|_| {
        r#"{"error":"Failed to serialize error"}"#.to_string()
    });

    let bytes = output_json.as_bytes();
    let len = bytes.len();
    let layout = Layout::array::<u8>(len).unwrap();
    let ptr = unsafe { alloc(layout) };

    unsafe {
        ptr::copy_nonoverlapping(bytes.as_ptr(), ptr, len);
        RESULT_PTR = ptr;
        RESULT_LEN = len;
    }

    1 // Error
}
