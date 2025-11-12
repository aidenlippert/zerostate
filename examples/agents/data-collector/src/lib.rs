use serde::{Deserialize, Serialize};
use serde_json;

#[derive(Serialize, Deserialize)]
struct Request {
    query: String,
}

#[derive(Serialize, Deserialize)]
struct Response {
    data: Vec<DataPoint>,
    summary: String,
    count: usize,
}

#[derive(Serialize, Deserialize, Clone)]
struct DataPoint {
    id: usize,
    value: String,
    metadata: String,
}

/// Data Collector Agent
/// Capability: data_collection, analysis, extraction
///
/// This agent simulates data collection and provides structured output
/// that can be used by other agents (like Report Writer)
#[no_mangle]
pub extern "C" fn execute(input_ptr: *const u8, input_len: usize) -> *mut u8 {
    // Read input
    let input_slice = unsafe { std::slice::from_raw_parts(input_ptr, input_len) };
    let input_str = std::str::from_utf8(input_slice).unwrap_or("");

    // Parse request
    let request: Request = match serde_json::from_str(input_str) {
        Ok(req) => req,
        Err(_) => Request {
            query: input_str.to_string(),
        },
    };

    // Simulate data collection based on query keywords
    let data = collect_data(&request.query);
    let count = data.len();

    let summary = format!(
        "Collected {} data points for query: '{}'. Data includes IDs, values, and metadata ready for analysis.",
        count,
        request.query
    );

    let response = Response {
        data,
        summary,
        count,
    };

    // Serialize response
    let response_json = serde_json::to_string(&response).unwrap();
    let response_bytes = response_json.as_bytes();

    // Allocate memory for response
    let mut output = Vec::with_capacity(response_bytes.len());
    output.extend_from_slice(response_bytes);

    // Return pointer to response
    let ptr = output.as_mut_ptr();
    std::mem::forget(output);
    ptr
}

fn collect_data(query: &str) -> Vec<DataPoint> {
    let query_lower = query.to_lowercase();

    // Simulate different data collection based on query
    if query_lower.contains("sales") || query_lower.contains("revenue") {
        vec![
            DataPoint {
                id: 1,
                value: "$125,000".to_string(),
                metadata: "Q1 2024 revenue, +15% YoY".to_string(),
            },
            DataPoint {
                id: 2,
                value: "$142,000".to_string(),
                metadata: "Q2 2024 revenue, +22% YoY".to_string(),
            },
            DataPoint {
                id: 3,
                value: "$138,500".to_string(),
                metadata: "Q3 2024 revenue, +18% YoY".to_string(),
            },
            DataPoint {
                id: 4,
                value: "$156,000".to_string(),
                metadata: "Q4 2024 revenue (projected), +25% YoY".to_string(),
            },
        ]
    } else if query_lower.contains("user") || query_lower.contains("customer") {
        vec![
            DataPoint {
                id: 1,
                value: "1,250 users".to_string(),
                metadata: "Active monthly users, +45% MoM".to_string(),
            },
            DataPoint {
                id: 2,
                value: "89% retention".to_string(),
                metadata: "7-day retention rate, industry avg: 65%".to_string(),
            },
            DataPoint {
                id: 3,
                value: "4.2/5.0 rating".to_string(),
                metadata: "Average user satisfaction score".to_string(),
            },
        ]
    } else if query_lower.contains("performance") || query_lower.contains("speed") {
        vec![
            DataPoint {
                id: 1,
                value: "125ms".to_string(),
                metadata: "P95 API response time, target: <200ms".to_string(),
            },
            DataPoint {
                id: 2,
                value: "99.97%".to_string(),
                metadata: "Service uptime, SLA: 99.9%".to_string(),
            },
            DataPoint {
                id: 3,
                value: "2.1s".to_string(),
                metadata: "Average page load time".to_string(),
            },
        ]
    } else {
        // Generic data collection
        vec![
            DataPoint {
                id: 1,
                value: "Data A".to_string(),
                metadata: format!("Related to: {}", query),
            },
            DataPoint {
                id: 2,
                value: "Data B".to_string(),
                metadata: format!("Found from: {}", query),
            },
            DataPoint {
                id: 3,
                value: "Data C".to_string(),
                metadata: format!("Extracted for: {}", query),
            },
        ]
    }
}

/// Memory allocation for WASM
#[no_mangle]
pub extern "C" fn alloc(size: usize) -> *mut u8 {
    let mut buf = Vec::with_capacity(size);
    let ptr = buf.as_mut_ptr();
    std::mem::forget(buf);
    ptr
}

/// Memory deallocation for WASM
#[no_mangle]
pub extern "C" fn dealloc(ptr: *mut u8, size: usize) {
    unsafe {
        let _ = Vec::from_raw_parts(ptr, size, size);
    }
}
