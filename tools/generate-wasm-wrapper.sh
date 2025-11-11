#!/bin/bash
# Generate WASM Wrapper for HTTP-based Agents
# Usage: ./generate-wasm-wrapper.sh --name "Agent Name" --endpoint "https://api.com/execute" --capabilities "cap1,cap2"

set -e

# Parse arguments
NAME=""
ENDPOINT=""
CAPABILITIES=""
OUTPUT=""
API_KEY=""

while [[ $# -gt 0 ]]; do
  case $1 in
    --name)
      NAME="$2"
      shift 2
      ;;
    --endpoint)
      ENDPOINT="$2"
      shift 2
      ;;
    --capabilities)
      CAPABILITIES="$2"
      shift 2
      ;;
    --output)
      OUTPUT="$2"
      shift 2
      ;;
    --api-key)
      API_KEY="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# Validate required arguments
if [[ -z "$NAME" || -z "$ENDPOINT" || -z "$CAPABILITIES" ]]; then
  echo "Error: Missing required arguments"
  echo "Usage: $0 --name 'Agent Name' --endpoint 'https://api.com/execute' --capabilities 'cap1,cap2' [--output agent.wasm] [--api-key KEY]"
  exit 1
fi

# Set default output
if [[ -z "$OUTPUT" ]]; then
  OUTPUT="agent-wrapper.wasm"
fi

echo "🔧 Generating WASM wrapper for agent: $NAME"
echo "  Endpoint: $ENDPOINT"
echo "  Capabilities: $CAPABILITIES"

# Create temporary directory
TMPDIR=$(mktemp -d)
cd "$TMPDIR"

# Initialize Rust project
echo "📦 Creating Rust project..."
cargo init --lib agent-wrapper

cd agent-wrapper

# Add dependencies
cat > Cargo.toml << EOF
[package]
name = "agent-wrapper"
version = "1.0.0"
edition = "2021"

[lib]
crate-type = ["cdylib"]

[dependencies]
wasm-bindgen = "0.2"
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"

[profile.release]
opt-level = "z"
lto = true
codegen-units = 1
EOF

# Generate Rust source code
mkdir -p src
cat > src/lib.rs << 'RUST_EOF'
use wasm_bindgen::prelude::*;
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize)]
struct Request {
    input: String,
}

#[derive(Serialize, Deserialize)]
struct Response {
    output: String,
    #[serde(default)]
    error: Option<String>,
}

#[wasm_bindgen]
extern "C" {
    #[wasm_bindgen(js_namespace = console)]
    fn log(s: &str);
}

// This will be replaced by the actual endpoint
const AGENT_ENDPOINT: &str = "ENDPOINT_PLACEHOLDER";
const API_KEY: &str = "API_KEY_PLACEHOLDER";

#[wasm_bindgen]
pub fn execute(input: String) -> String {
    // In a real implementation, this would use fetch API
    // For now, return a stub response indicating the agent needs to be called
    let response = Response {
        output: format!(
            "Agent wrapper for: {}\nEndpoint: {}\nInput received: {}\n\nNote: This is a stub. Actual execution requires HTTP fetch support in WASM runtime.",
            "AGENT_NAME_PLACEHOLDER",
            AGENT_ENDPOINT,
            input
        ),
        error: None,
    };

    serde_json::to_string(&response).unwrap_or_else(|e| {
        format!(r#"{{"error": "Failed to serialize response: {}"}}"#, e)
    })
}

// Additional metadata exports
#[wasm_bindgen]
pub fn get_capabilities() -> String {
    "CAPABILITIES_PLACEHOLDER".to_string()
}

#[wasm_bindgen]
pub fn get_version() -> String {
    "1.0.0".to_string()
}
RUST_EOF

# Replace placeholders
sed -i "s|ENDPOINT_PLACEHOLDER|$ENDPOINT|g" src/lib.rs
sed -i "s|AGENT_NAME_PLACEHOLDER|$NAME|g" src/lib.rs
sed -i "s|CAPABILITIES_PLACEHOLDER|$CAPABILITIES|g" src/lib.rs
if [[ -n "$API_KEY" ]]; then
  sed -i "s|API_KEY_PLACEHOLDER|$API_KEY|g" src/lib.rs
else
  sed -i "s|API_KEY_PLACEHOLDER||g" src/lib.rs
fi

echo "🔨 Building WASM binary..."
cargo build --target wasm32-unknown-unknown --release

# Copy output to specified location
WASM_FILE="target/wasm32-unknown-unknown/release/agent_wrapper.wasm"
if [[ -f "$WASM_FILE" ]]; then
  # Get back to original directory
  cd - > /dev/null
  cp "$TMPDIR/agent-wrapper/$WASM_FILE" "$OUTPUT"

  # Optimize with wasm-opt if available
  if command -v wasm-opt &> /dev/null; then
    echo "⚡ Optimizing with wasm-opt..."
    wasm-opt -Oz "$OUTPUT" -o "${OUTPUT}.tmp"
    mv "${OUTPUT}.tmp" "$OUTPUT"
  fi

  # Get file size
  SIZE=$(du -h "$OUTPUT" | cut -f1)

  echo "✅ WASM wrapper generated successfully!"
  echo "  Output: $OUTPUT"
  echo "  Size: $SIZE"
  echo ""
  echo "📤 Upload with:"
  echo "  curl -X POST https://zerostate-api.fly.dev/api/v1/agents/upload \\"
  echo "    -H 'Authorization: Bearer YOUR_JWT_TOKEN' \\"
  echo "    -F 'wasm_binary=@$OUTPUT' \\"
  echo "    -F 'name=$NAME' \\"
  echo "    -F 'description=HTTP wrapper for $NAME' \\"
  echo "    -F 'version=1.0.0' \\"
  echo "    -F 'capabilities=[\"$(echo $CAPABILITIES | sed 's/,/","/g')\"]' \\"
  echo "    -F 'price=0.05'"

  # Cleanup
  rm -rf "$TMPDIR"
else
  echo "❌ Build failed"
  exit 1
fi
