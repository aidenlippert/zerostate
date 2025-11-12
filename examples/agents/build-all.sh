#!/bin/bash
# Build all test agents for ZeroState

set -e

echo "🔨 Building ZeroState Test Agents..."

# Create output directory
mkdir -p dist

# Build Data Collector Agent
echo ""
echo "📊 Building Data Collector Agent..."
cd data-collector
cargo build --target wasm32-unknown-unknown --release
cp target/wasm32-unknown-unknown/release/data_collector.wasm ../dist/
cd ..

# Build Report Writer Agent
echo ""
echo "📝 Building Report Writer Agent..."
cd report-writer
cargo build --target wasm32-unknown-unknown --release
cp target/wasm32-unknown-unknown/release/report_writer.wasm ../dist/
cd ..

echo ""
echo "✅ Build complete!"
echo ""
echo "📦 WASM Binaries:"
ls -lh dist/*.wasm

echo ""
echo "🎯 Next Steps:"
echo "1. Register user: curl -X POST https://zerostate-api.fly.dev/api/v1/users/register \\"
echo "     -H 'Content-Type: application/json' \\"
echo "     -d '{\"email\":\"test@example.com\",\"password\":\"test123\",\"full_name\":\"Test User\"}'"
echo ""
echo "2. Upload Data Collector:"
echo "   curl -X POST https://zerostate-api.fly.dev/api/v1/agents/upload \\"
echo "     -H 'Authorization: Bearer \$TOKEN' \\"
echo "     -F 'wasm_binary=@dist/data_collector.wasm' \\"
echo "     -F 'name=Data Collector' \\"
echo "     -F 'description=Collects and structures data for analysis' \\"
echo "     -F 'version=1.0.0' \\"
echo "     -F 'capabilities=[\"data_collection\",\"analysis\",\"extraction\"]' \\"
echo "     -F 'price=0.05'"
echo ""
echo "3. Upload Report Writer:"
echo "   curl -X POST https://zerostate-api.fly.dev/api/v1/agents/upload \\"
echo "     -H 'Authorization: Bearer \$TOKEN' \\"
echo "     -F 'wasm_binary=@dist/report_writer.wasm' \\"
echo "     -F 'name=Report Writer' \\"
echo "     -F 'description=Generates professional reports from structured data' \\"
echo "     -F 'version=1.0.0' \\"
echo "     -F 'capabilities=[\"report_generation\",\"summarization\",\"writing\"]' \\"
echo "     -F 'price=0.08'"
echo ""
echo "4. Test collaboration:"
echo "   curl -X POST https://zerostate-api.fly.dev/api/v1/tasks/submit \\"
echo "     -H 'Authorization: Bearer \$TOKEN' \\"
echo "     -d '{\"query\":\"Analyze Q4 sales performance and generate executive report\",\"budget\":0.50,\"timeout\":300}'"
