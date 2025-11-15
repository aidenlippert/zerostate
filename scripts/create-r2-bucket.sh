#!/bin/bash
# Create Cloudflare R2 bucket via wrangler CLI

echo "🪣 Creating Cloudflare R2 bucket: ainur-agents"
echo ""
echo "Since AWS CLI doesn't have bucket creation permissions,"
echo "you'll need to create the bucket via Cloudflare Dashboard:"
echo ""
echo "Steps:"
echo "1. Go to: https://dash.cloudflare.com/"
echo "2. Click 'R2' in left sidebar"
echo "3. Click 'Create bucket'"
echo "4. Name: ainur-agents"
echo "5. Location: Automatic"
echo "6. Click 'Create bucket'"
echo ""
echo "After creation, run: ./test-r2-upload.sh to verify"
