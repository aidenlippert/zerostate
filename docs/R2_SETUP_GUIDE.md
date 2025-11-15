# 🪣 Cloudflare R2 Setup Guide for Ainur

## ✅ What We Have

- **R2 API Token**: Created ✅
- **Access Key ID**: `f95be90d0e652041cc3af9ffee4b7abc` ✅
- **Secret Access Key**: `858ba29...` ✅
- **Endpoint**: `https://d4affab848a8f8e47b7930147fe1b43a.r2.cloudflarestorage.com` ✅
- **Go SDK**: `libs/storage/r2.go` created ✅
- **Environment Variables**: Added to `.env` ✅

## 🔧 Step 1: Create Bucket (Manual - One Time)

The API token you created doesn't have `ListBuckets` permission. You need to create the bucket manually:

### Via Cloudflare Dashboard:

1. Go to: https://dash.cloudflare.com/
2. Click **"R2"** in left sidebar
3. Click **"Create bucket"**
4. Enter bucket name: **`ainur-agents`**
5. Location: **Automatic** (or choose closest region)
6. Click **"Create bucket"**

### Via Wrangler CLI (Alternative):

```bash
# Install wrangler
npm install -g wrangler

# Login to Cloudflare
wrangler login

# Create bucket
wrangler r2 bucket create ainur-agents
```

## 🧪 Step 2: Test R2 Upload (After Bucket Created)

Once you've created the bucket, test the upload:

```bash
# Test R2 connectivity and upload Math Agent
./test-r2-upload.sh
```

This will:
- ✅ Verify bucket exists
- ✅ Upload Math Agent WASM (986 bytes)
- ✅ Download and verify integrity
- ✅ Generate public URL

## 📚 Step 3: Using R2 in Go Code

```go
package main

import (
    "context"
    "fmt"
    "os"
    
    "github.com/yourusername/zerostate/libs/storage"
)

func main() {
    // Create R2 client from environment variables
    r2, err := storage.NewR2StorageFromEnv()
    if err != nil {
        panic(err)
    }
    
    // Upload WASM
    wasmData, _ := os.ReadFile("math_agent.wasm")
    err = r2.UploadWASM(context.Background(), "agents/math-agent-v1.0.wasm", wasmData)
    if err != nil {
        panic(err)
    }
    
    fmt.Println("✅ Uploaded to R2!")
    
    // Download WASM
    data, err := r2.DownloadWASM(context.Background(), "agents/math-agent-v1.0.wasm")
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("✅ Downloaded %d bytes from R2!\n", len(data))
}
```

## 🔐 Security Best Practices

### For Development:
- ✅ Use `.env` file (already configured)
- ✅ Never commit `.env` to git (already in `.gitignore`)

### For Production (Fly.io):
```bash
# Set R2 secrets in Fly.io
fly secrets set \
  R2_ACCESS_KEY_ID=f95be90d0e652041cc3af9ffee4b7abc \
  R2_SECRET_ACCESS_KEY=858ba29693ea0906a86f6a07aca10cc4f7185d0411b6ed7fc5ea1e6683a0e7c0 \
  R2_ENDPOINT=https://d4affab848a8f8e47b7930147fe1b43a.r2.cloudflarestorage.com \
  R2_BUCKET_NAME=ainur-agents
```

## 📊 R2 Storage Structure

```
ainur-agents/
├── agents/
│   ├── math-agent-v1.0.wasm (986 bytes)
│   ├── math-agent-v1.1.wasm
│   ├── text-agent-v1.0.wasm
│   └── ...
├── metadata/
│   └── agent-manifests.json
└── backups/
    └── 2025-11-12/
```

## 💰 R2 Pricing (Free Tier)

- **Storage**: 10 GB/month (free)
- **Class A Operations**: 1M/month (PUT, LIST) (free)
- **Class B Operations**: 10M/month (GET, HEAD) (free)
- **Egress**: Free (no data transfer fees!)

**Cost for Ainur**:
- 1,000 agents × 10 KB each = 10 MB storage ($0.00)
- 100K downloads/month = 100K Class B ops ($0.00)
- **Total: $0.00/month** 🎉

## 🚀 Ready for Wasmtime Integration!

Once bucket is created and `./test-r2-upload.sh` passes, we can:

1. ✅ Update agent upload handler to store WASM in R2
2. ✅ Update WASM executor to fetch from R2
3. ✅ Test full E2E: Upload → Execute → Result
4. ✅ Deploy to Fly.io

---

**Next Step**: Create the bucket, then run `./test-r2-upload.sh`!
