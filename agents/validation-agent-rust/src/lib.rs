use serde::{Deserialize, Serialize};
use std::alloc::{alloc, dealloc, Layout};
use std::ptr;
use std::slice;

#[derive(Deserialize)]
struct Input {
    operation: String,
    value: String,
    #[serde(default)]
    pattern: Option<String>,
}

#[derive(Serialize)]
struct Output {
    valid: bool,
    result: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    details: Option<String>,
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
        "email" => validate_email(&input.value),
        "url" => validate_url(&input.value),
        "phone" => validate_phone(&input.value),
        "credit_card" => validate_credit_card(&input.value),
        "ipv4" => validate_ipv4(&input.value),
        "ipv6" => validate_ipv6(&input.value),
        "regex" => {
            if let Some(pattern) = &input.pattern {
                validate_regex(&input.value, pattern)
            } else {
                Output {
                    valid: false,
                    result: "failed".to_string(),
                    details: Some("Pattern required for regex validation".to_string()),
                }
            }
        },
        "not_empty" => {
            let valid = !input.value.trim().is_empty();
            Output {
                valid,
                result: if valid { "valid".to_string() } else { "empty".to_string() },
                details: None,
            }
        },
        "length" => {
            if let Some(pattern) = &input.pattern {
                validate_length(&input.value, pattern)
            } else {
                Output {
                    valid: false,
                    result: "failed".to_string(),
                    details: Some("Pattern required (e.g., 'min:5', 'max:100', 'range:5-100')".to_string()),
                }
            }
        },
        "numeric" => {
            let valid = input.value.parse::<f64>().is_ok();
            Output {
                valid,
                result: if valid { "numeric".to_string() } else { "not_numeric".to_string() },
                details: None,
            }
        },
        "alpha" => {
            let valid = input.value.chars().all(|c| c.is_alphabetic());
            Output {
                valid,
                result: if valid { "alphabetic".to_string() } else { "contains_non_alpha".to_string() },
                details: None,
            }
        },
        "alphanumeric" => {
            let valid = input.value.chars().all(|c| c.is_alphanumeric());
            Output {
                valid,
                result: if valid { "alphanumeric".to_string() } else { "contains_special_chars".to_string() },
                details: None,
            }
        },
        _ => return error_response(&format!("Unknown operation: {}", input.operation)),
    };

    let output_json = match serde_json::to_string(&output) {
        Ok(j) => j,
        Err(e) => return error_response(&format!("JSON serialize error: {}", e)),
    };

    store_result(&output_json);
    0
}

// Email validation (simplified RFC 5322)
fn validate_email(email: &str) -> Output {
    let email_regex = r"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$";
    
    let parts: Vec<&str> = email.split('@').collect();
    if parts.len() != 2 {
        return Output {
            valid: false,
            result: "invalid".to_string(),
            details: Some("Must contain exactly one @ symbol".to_string()),
        };
    }

    let (local, domain) = (parts[0], parts[1]);
    
    if local.is_empty() || domain.is_empty() {
        return Output {
            valid: false,
            result: "invalid".to_string(),
            details: Some("Local or domain part is empty".to_string()),
        };
    }

    if !domain.contains('.') {
        return Output {
            valid: false,
            result: "invalid".to_string(),
            details: Some("Domain must contain a period".to_string()),
        };
    }

    // Basic character validation
    let valid_chars = local.chars().all(|c| c.is_alphanumeric() || "._%+-".contains(c))
        && domain.chars().all(|c| c.is_alphanumeric() || ".-".contains(c));

    Output {
        valid: valid_chars,
        result: if valid_chars { "valid_email".to_string() } else { "invalid_characters".to_string() },
        details: if valid_chars { Some(format!("{}@{}", local, domain)) } else { None },
    }
}

// URL validation
fn validate_url(url: &str) -> Output {
    let has_protocol = url.starts_with("http://") || url.starts_with("https://");
    
    if !has_protocol {
        return Output {
            valid: false,
            result: "invalid".to_string(),
            details: Some("Must start with http:// or https://".to_string()),
        };
    }

    let without_protocol = if url.starts_with("https://") {
        &url[8..]
    } else {
        &url[7..]
    };

    let has_domain = without_protocol.contains('.') && !without_protocol.is_empty();

    Output {
        valid: has_domain,
        result: if has_domain { "valid_url".to_string() } else { "invalid_format".to_string() },
        details: if has_domain { Some(without_protocol.to_string()) } else { None },
    }
}

// Phone number validation (international)
fn validate_phone(phone: &str) -> Output {
    // Remove common formatting characters
    let digits: String = phone.chars().filter(|c| c.is_numeric() || *c == '+').collect();
    
    let valid = if digits.starts_with('+') {
        digits.len() >= 11 && digits.len() <= 15 // International: +1234567890
    } else {
        digits.len() >= 10 && digits.len() <= 15 // Domestic: 1234567890
    };

    Output {
        valid,
        result: if valid { "valid_phone".to_string() } else { "invalid_length".to_string() },
        details: Some(format!("{} digits", digits.len())),
    }
}

// Credit card validation (Luhn algorithm)
fn validate_credit_card(number: &str) -> Output {
    let digits: String = number.chars().filter(|c| c.is_numeric()).collect();
    
    if digits.len() < 13 || digits.len() > 19 {
        return Output {
            valid: false,
            result: "invalid_length".to_string(),
            details: Some(format!("Length: {} (expected 13-19)", digits.len())),
        };
    }

    // Luhn algorithm
    let mut sum = 0;
    let mut double = false;
    
    for ch in digits.chars().rev() {
        let mut digit = ch.to_digit(10).unwrap() as i32;
        
        if double {
            digit *= 2;
            if digit > 9 {
                digit -= 9;
            }
        }
        
        sum += digit;
        double = !double;
    }

    let valid = sum % 10 == 0;

    Output {
        valid,
        result: if valid { "valid_card".to_string() } else { "luhn_check_failed".to_string() },
        details: Some(format!("Length: {}, Checksum: {}", digits.len(), sum % 10)),
    }
}

// IPv4 validation
fn validate_ipv4(ip: &str) -> Output {
    let parts: Vec<&str> = ip.split('.').collect();
    
    if parts.len() != 4 {
        return Output {
            valid: false,
            result: "invalid".to_string(),
            details: Some("Must have 4 octets".to_string()),
        };
    }

    for part in &parts {
        match part.parse::<u8>() {
            Ok(_) => {},
            Err(_) => {
                return Output {
                    valid: false,
                    result: "invalid".to_string(),
                    details: Some(format!("Invalid octet: {}", part)),
                };
            }
        }
    }

    Output {
        valid: true,
        result: "valid_ipv4".to_string(),
        details: Some(ip.to_string()),
    }
}

// IPv6 validation (simplified)
fn validate_ipv6(ip: &str) -> Output {
    let has_colon = ip.contains(':');
    let parts: Vec<&str> = ip.split(':').collect();
    
    if !has_colon || parts.len() < 3 || parts.len() > 8 {
        return Output {
            valid: false,
            result: "invalid".to_string(),
            details: Some("Invalid IPv6 format".to_string()),
        };
    }

    // Check for valid hex characters
    for part in &parts {
        if !part.is_empty() && !part.chars().all(|c| c.is_ascii_hexdigit()) {
            return Output {
                valid: false,
                result: "invalid".to_string(),
                details: Some(format!("Invalid hex: {}", part)),
            };
        }
    }

    Output {
        valid: true,
        result: "valid_ipv6".to_string(),
        details: Some(format!("{} groups", parts.len())),
    }
}

// Regex validation
fn validate_regex(value: &str, pattern: &str) -> Output {
    // For WASM, we'll do a simple pattern match without full regex support
    // In production, you'd use a proper regex crate
    let matches = value.contains(pattern);
    
    Output {
        valid: matches,
        result: if matches { "matches".to_string() } else { "no_match".to_string() },
        details: Some(format!("Pattern: {}", pattern)),
    }
}

// Length validation
fn validate_length(value: &str, pattern: &str) -> Output {
    let len = value.len();
    
    if pattern.starts_with("min:") {
        let min: usize = pattern[4..].parse().unwrap_or(0);
        let valid = len >= min;
        return Output {
            valid,
            result: if valid { "valid_length".to_string() } else { "too_short".to_string() },
            details: Some(format!("Length: {}, Min: {}", len, min)),
        };
    }
    
    if pattern.starts_with("max:") {
        let max: usize = pattern[4..].parse().unwrap_or(usize::MAX);
        let valid = len <= max;
        return Output {
            valid,
            result: if valid { "valid_length".to_string() } else { "too_long".to_string() },
            details: Some(format!("Length: {}, Max: {}", len, max)),
        };
    }
    
    if pattern.starts_with("range:") {
        let range_parts: Vec<&str> = pattern[6..].split('-').collect();
        if range_parts.len() == 2 {
            let min: usize = range_parts[0].parse().unwrap_or(0);
            let max: usize = range_parts[1].parse().unwrap_or(usize::MAX);
            let valid = len >= min && len <= max;
            return Output {
                valid,
                result: if valid { "valid_length".to_string() } else { "out_of_range".to_string() },
                details: Some(format!("Length: {}, Range: {}-{}", len, min, max)),
            };
        }
    }

    Output {
        valid: false,
        result: "invalid_pattern".to_string(),
        details: Some("Use min:N, max:N, or range:N-M".to_string()),
    }
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
