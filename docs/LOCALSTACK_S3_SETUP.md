# LocalStack S3 Setup for ZeroState API - COMPLETE ✅

**Date**: January 2025
**Purpose**: Local S3 mock for WASM binary storage and testing
**Status**: Operational and Ready for Testing

---

## Overview

LocalStack is now running and configured for the ZeroState API, providing a local S3-compatible storage solution for WASM agent binaries. This enables full end-to-end testing without requiring AWS credentials or incurring cloud costs.

---

## LocalStack Configuration

### Container Status
```bash
CONTAINER ID: 0da3564893f3
IMAGE: localstack/localstack:latest
STATUS: Up and healthy
PORTS: 0.0.0.0:4566->4566/tcp
NAME: zerostate-localstack
```

### S3 Bucket Created
```bash
Bucket Name: zerostate-dev
Region: us-east-1
Endpoint: http://localhost:4566
Status: Active
```

### Test WASM Binary Uploaded
```bash
File: tests/wasm/hello.wasm
Size: 2.3 MiB
S3 Path: s3://zerostate-dev/agents/agent_001/308d956b7a5461d254a6340e878832f2.wasm
Hash: 308d956b7a5461d254a6340e878832f2
Status: Uploaded successfully
```

---

## API Server Configuration

### Environment Variables
```bash
S3_BUCKET=zerostate-dev
S3_ENDPOINT=http://localhost:4566
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
S3_REGION=us-east-1
```

### Server Status
```
✅ S3 storage initialized successfully
✅ Binary store initialized with S3 backend
✅ Server running on port 9000
✅ All components operational
```

### Startup Logs Confirmation
```
{"level":"info","msg":"initializing S3 storage"}
{"level":"info","msg":"S3 storage initialized","bucket":"zerostate-dev","region":"us-east-1"}
{"level":"info","msg":"S3 storage initialized successfully","bucket":"zerostate-dev","region":"us-east-1"}
{"level":"info","msg":"binary store initialized with S3 backend"}
```

---

## Quick Start Guide

### 1. Start LocalStack (Already Running)
```bash
docker run -d -p 4566:4566 --name zerostate-localstack localstack/localstack:latest
```

### 2. Create S3 Bucket (Already Created)
```bash
AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
  aws --endpoint-url=http://localhost:4566 s3 mb s3://zerostate-dev
```

### 3. Upload WASM Binary (Already Uploaded)
```bash
HASH=$(openssl rand -hex 16)
AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
  aws --endpoint-url=http://localhost:4566 s3 cp \
    tests/wasm/hello.wasm \
    s3://zerostate-dev/agents/agent_001/${HASH}.wasm
```

### 4. Start API Server with S3 (Currently Running)
```bash
S3_BUCKET=zerostate-dev \
S3_ENDPOINT=http://localhost:4566 \
AWS_ACCESS_KEY_ID=test \
AWS_SECRET_ACCESS_KEY=test \
S3_REGION=us-east-1 \
./api
```

### 5. Test WASM Execution

#### Get Authentication Token
```bash
TOKEN=$(curl -s -X POST http://localhost:9000/api/v1/users/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}' \
  | jq -r '.token')
```

#### Execute WASM Task
```bash
curl -X POST http://localhost:9000/api/v1/tasks/execute \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "agent_id": "agent_001",
    "input": "test input"
  }' | jq .
```

**Note**: The agent's `binary_url` and `binary_hash` fields in the database need to be updated to point to the uploaded WASM binary before execution will work.

---

## Database Setup (Manual Step Required)

The agent record needs to be updated with the S3 binary location:

```sql
UPDATE agents
SET binary_url = 's3://zerostate-dev/agents/agent_001/308d956b7a5461d254a6340e878832f2.wasm',
    binary_hash = '308d956b7a5461d254a6340e878832f2'
WHERE id = 'agent_001';
```

**Alternative via Go script**:
```go
package main

import (
	"database/sql"
	"fmt"
	"log"
	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "zerostate.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		UPDATE agents
		SET binary_url = ?, binary_hash = ?
		WHERE id = ?
	`, "s3://zerostate-dev/agents/agent_001/308d956b7a5461d254a6340e878832f2.wasm",
	   "308d956b7a5461d254a6340e878832f2",
	   "agent_001")

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Agent updated successfully!")
}
```

---

## Verification Steps

### 1. Check LocalStack Status
```bash
docker ps | grep localstack
# Should show: Up X minutes (healthy)
```

### 2. List S3 Buckets
```bash
AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
  aws --endpoint-url=http://localhost:4566 s3 ls
# Should show: zerostate-dev
```

### 3. List Files in Bucket
```bash
AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
  aws --endpoint-url=http://localhost:4566 s3 ls s3://zerostate-dev/agents/agent_001/
# Should show: 308d956b7a5461d254a6340e878832f2.wasm
```

### 4. Verify API Server S3 Integration
```bash
curl http://localhost:9000/health | jq .
# Should show: "status": "healthy"
```

### 5. Check Server Logs for S3 Confirmation
Look for these lines in server startup:
```
{"level":"info","msg":"S3 storage initialized","bucket":"zerostate-dev"}
{"level":"info","msg":"binary store initialized with S3 backend"}
```

---

## Common Operations

### Stop LocalStack
```bash
docker stop zerostate-localstack
```

### Start LocalStack (if stopped)
```bash
docker start zerostate-localstack
```

### Remove LocalStack Container
```bash
docker rm -f zerostate-localstack
```

### Reset S3 Data (Remove and Recreate)
```bash
# Stop and remove container
docker rm -f zerostate-localstack

# Start fresh
docker run -d -p 4566:4566 --name zerostate-localstack localstack/localstack:latest

# Wait for LocalStack to be ready
sleep 10

# Recreate bucket
AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
  aws --endpoint-url=http://localhost:4566 s3 mb s3://zerostate-dev

# Re-upload binaries
AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
  aws --endpoint-url=http://localhost:4566 s3 cp \
    tests/wasm/hello.wasm \
    s3://zerostate-dev/agents/agent_001/308d956b7a5461d254a6340e878832f2.wasm
```

### View LocalStack Logs
```bash
docker logs zerostate-localstack
```

### Upload New WASM Binary
```bash
AGENT_ID="agent_002"
HASH=$(openssl rand -hex 16)

AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
  aws --endpoint-url=http://localhost:4566 s3 cp \
    /path/to/binary.wasm \
    s3://zerostate-dev/agents/${AGENT_ID}/${HASH}.wasm
```

---

## Troubleshooting

### LocalStack Not Starting
```bash
# Check if port 4566 is already in use
lsof -i :4566

# If occupied, kill the process or use a different port
docker run -d -p 4567:4566 --name zerostate-localstack localstack/localstack:latest
# Then update S3_ENDPOINT to http://localhost:4567
```

### S3 Upload Fails
```bash
# Verify LocalStack is healthy
docker ps | grep localstack

# Check LocalStack logs
docker logs zerostate-localstack

# Test S3 endpoint
curl http://localhost:4566/_localstack/health | jq .
```

### API Server Can't Connect to S3
```bash
# Verify environment variables are set
env | grep S3

# Check S3_ENDPOINT is pointing to localhost
echo $S3_ENDPOINT
# Should be: http://localhost:4566

# Restart API server with correct env vars
S3_BUCKET=zerostate-dev S3_ENDPOINT=http://localhost:4566 \
AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
S3_REGION=us-east-1 ./api
```

### WASM Execution Fails
```bash
# 1. Verify binary is in S3
AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
  aws --endpoint-url=http://localhost:4566 s3 ls \
  s3://zerostate-dev/agents/agent_001/

# 2. Verify database has correct binary_url
# (Requires database access - see Database Setup section)

# 3. Check API server logs for errors
# Look for download errors or binary store issues

# 4. Test with a fresh token
TOKEN=$(curl -s -X POST http://localhost:9000/api/v1/users/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}' \
  | jq -r '.token')
```

---

## Benefits of LocalStack

✅ **No AWS Account Required**: Test S3 functionality without cloud credentials
✅ **Cost-Free**: Zero cloud costs during development
✅ **Fast Iteration**: Instant uploads and downloads on localhost
✅ **Complete S3 API**: Full compatibility with AWS S3 SDK
✅ **Easy Reset**: Simple container restart to wipe data
✅ **Offline Development**: Works without internet connection
✅ **CI/CD Friendly**: Perfect for automated testing pipelines

---

## Next Steps

1. **Update Database**: Run the SQL/Go script above to update agent_001's binary URL
2. **Test Execution**: Execute a test task via the API
3. **Verify Results**: Check task results are stored in database
4. **Upload More Binaries**: Add additional agent WASM files for testing
5. **Production Migration**: When ready, switch to real AWS S3 by changing environment variables

---

## Production AWS S3 Migration

When moving to production AWS S3:

```bash
# 1. Create production S3 bucket
aws s3 mb s3://zerostate-production
aws s3api put-bucket-versioning \
  --bucket zerostate-production \
  --versioning-configuration Status=Enabled

# 2. Update environment variables (remove S3_ENDPOINT)
S3_BUCKET=zerostate-production
AWS_ACCESS_KEY_ID=<real-access-key>
AWS_SECRET_ACCESS_KEY=<real-secret-key>
S3_REGION=us-east-1

# 3. Upload binaries to production
aws s3 cp tests/wasm/hello.wasm \
  s3://zerostate-production/agents/agent_001/308d956b7a5461d254a6340e878832f2.wasm

# 4. Update database with production S3 URLs
# 5. Restart API server with production credentials
```

---

## Summary

**LocalStack S3 Setup**: ✅ COMPLETE
**S3 Bucket Created**: ✅ zerostate-dev
**WASM Binary Uploaded**: ✅ hello.wasm (2.3 MiB)
**API Server Configured**: ✅ S3 backend operational
**Binary Store**: ✅ Initialized and ready

**System Status**: Fully operational and ready for end-to-end WASM execution testing!

The only remaining step is updating the database agent record with the binary URL, then you can test full WASM execution locally!
