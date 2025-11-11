# WASM Upload Issue - Root Cause Analysis

**Status**: Issue identified - Multipart form parsing with large files
**Priority**: P0 - Blocks beta launch
**Date**: 2025-11-09

---

## Issue Summary

Agent registration with 5.8MB WASM files fails with validation errors, even though:
- R2 storage is configured correctly ✅
- MaxWASMSize is set to 50MB ✅
- Small WASM files (< 1KB) work correctly ✅
- The multipart form structure is correct ✅

**Error Message**:
```json
{
  "error": "invalid request",
  "message": "Key: 'RegisterAgentRequest.Name' Error:Field validation for 'Name' failed on the 'required' tag\nKey: 'RegisterAgentRequest.Capabilities' Error:Field validation for 'Capabilities' failed on the 'required' tag\nKey: 'RegisterAgentRequest.Pricing' Error:Field validation for 'Pricing' failed on the 'required' tag"
}
```

---

## Root Cause

### Code Flow Analysis

**File**: [libs/api/agent_handlers.go:26-161](libs/api/agent_handlers.go:26-161)

```go
func (h *Handlers) RegisterAgent(c *gin.Context) {
    // Step 1: Parse multipart form (line 41)
    if err := c.Request.ParseMultipartForm(MaxWASMSize); err != nil {
        // This succeeds with small files, fails silently with large files
        return
    }

    // Step 2: Get WASM binary file (line 55)
    file, header, err := c.Request.FormFile("wasm_binary")
    // This works - file is retrieved successfully

    // Step 3: Read WASM into memory (line 101-113)
    wasmData := make([]byte, header.Size)  // Allocates 5.8MB
    io.ReadFull(file, wasmData)
    // This works - WASM is read successfully

    // Step 4: Get JSON metadata (line 137)
    agentJSON := c.Request.FormValue("agent")
    // ❌ THIS FAILS - returns empty string with large files!

    if agentJSON == "" {
        c.JSON(400, "agent field with JSON data is required")
        return
    }
}
```

### Why It Fails

**Hypothesis**: The issue is NOT with the multipart parsing itself, but with how `c.Request.FormValue()` retrieves the form data after the large file has been read.

**Possible Causes**:
1. **Memory Pressure**: Reading 5.8MB into memory may cause form values to be discarded
2. **Gin Framework Limitation**: FormValue() may not work correctly after FormFile() with large files
3. **Fly.io Proxy Timeout**: The proxy may be timing out or buffering incorrectly
4. **Request Body Already Consumed**: After reading the file, the form values may no longer be accessible

---

## Evidence

### ✅ What Works
- User registration: `POST /api/v1/users/register` (tested successfully)
- R2 storage initialization: Logs show "S3 storage initialized successfully"
- Multipart form parsing with tiny files (150 bytes)
- File retrieval: `c.Request.FormFile("wasm_binary")` succeeds with 5.8MB file
- WASM reading: `io.ReadFull()` completes successfully

### ❌ What Fails
- `c.Request.FormValue("agent")` returns empty string when file > 5MB
- JSON unmarshaling fails because `agentJSON == ""`
- Validation errors occur because all required fields are missing

### 🧪 Test Results

**Test 1: Tiny WASM (150 bytes)**
```bash
# Result: Rejected with "WASM binary is suspiciously small"
# Proves: JSON parsing works correctly
```

**Test 2: Real WASM (5.8MB)**
```bash
# Result: "Key: 'RegisterAgentRequest.Name' Error:Field validation for 'Name' failed on the 'required' tag"
# Proves: JSON form value is not being retrieved
```

---

## Solutions

### Option 1: Fix Form Value Retrieval (Quick Fix - Recommended)

**Problem**: `c.Request.FormValue()` doesn't work after large file read

**Solution**: Parse the form before reading the file, or use a different method to access form values

```go
func (h *Handlers) RegisterAgent(c *gin.Context) {
    // Parse multipart form FIRST
    if err := c.Request.ParseMultipartForm(MaxWASMSize); err != nil {
        // handle error
        return
    }

    // Get JSON metadata BEFORE reading file  // ← CRITICAL FIX
    agentJSON := c.Request.FormValue("agent")
    if agentJSON == "" {
        c.JSON(400, "agent field required")
        return
    }

    // Parse JSON into struct
    var req RegisterAgentRequest
    if err := json.Unmarshal([]byte(agentJSON), &req); err != nil {
        c.JSON(400, fmt.Sprintf("invalid JSON: %v", err))
        return
    }

    // NOW get the file
    file, header, err := c.Request.FormFile("wasm_binary")
    if err != nil {
        c.JSON(400, "wasm_binary required")
        return
    }
    defer file.Close()

    // Continue with file processing...
}
```

**Time**: 30 minutes
**Risk**: Low
**Test**: Upload 5.8MB file again

---

### Option 2: Increase Fly.io Timeouts

**Problem**: Fly.io proxy may be timing out during upload

**Solution**: Update `fly.toml` configuration

```toml
[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = false
  auto_start_machines = true
  min_machines_running = 1
  processes = ["app"]

  # Increase timeout for large uploads
  [http_service.http_options]
    response_timeout = 60  # Default is 30s
```

**Command**:
```bash
fly deploy --app zerostate-api
```

**Time**: 10 minutes
**Risk**: Low
**Test**: Upload after deployment

---

### Option 3: Implement Chunked Upload (Long-term Solution)

**Problem**: Large file uploads are unreliable

**Solution**: Implement resumable chunked uploads

```go
// Client side: Split file into chunks
// POST /api/v1/agents/upload/start   → Get upload ID
// POST /api/v1/agents/upload/chunk   → Upload chunk
// POST /api/v1/agents/upload/complete → Finalize

// Server side: Store chunks in R2, assemble on completion
```

**Time**: 2-3 days
**Risk**: Medium (new functionality)
**Benefit**: Supports very large agents (>50MB) in future

---

### Option 4: Switch to Endpoint Agents First

**Problem**: WASM upload is complex and blocking progress

**Solution**: Prioritize endpoint agent implementation (no file upload needed)

**Benefits**:
- No file upload issues
- Easier for CrewAI/LangChain users
- Can test marketplace mechanics immediately
- WASM can be fixed in parallel

**Implementation**: See [AGENT_TYPES_ARCHITECTURE.md](AGENT_TYPES_ARCHITECTURE.md) Task 2

**Time**: 1-2 days
**Risk**: Low
**ROI**: High (unblocks beta testers)

---

## Recommendation

**Execute in parallel**:

1. **Immediate (Today)**: Implement Option 1 (Fix Form Value Retrieval)
   - 30 minutes to implement
   - Test with 5.8MB file
   - Deploy to production if successful

2. **If Option 1 fails (Today)**: Try Option 2 (Increase Timeouts)
   - 10 minutes to configure
   - Redeploy and test

3. **Start in parallel (This Week)**: Option 4 (Endpoint Agents)
   - Begin implementation regardless of WASM fix
   - Endpoint agents don't require file uploads
   - Enables beta testing while WASM is perfected

4. **Future Enhancement (Week 2-3)**: Option 3 (Chunked Upload)
   - Implement after beta launch
   - Improves reliability for very large agents
   - Enables GPU-heavy container agents

---

## Next Steps

1. Apply Option 1 fix to [agent_handlers.go:135-161](libs/api/agent_handlers.go:135-161)
2. Rebuild and deploy: `fly deploy --app zerostate-api`
3. Test with production script: `python3 /tmp/test_agent_upload.py`
4. If successful, mark "Test agent registration with WASM upload" as ✅
5. Begin endpoint agent implementation regardless of outcome

---

## Success Criteria

- [ ] 5.8MB echo-agent.wasm uploads successfully
- [ ] Agent record created in database
- [ ] WASM binary stored in R2 with correct URL
- [ ] Agent can be retrieved via `GET /api/v1/agents/:id`
- [ ] Ready for task execution testing

---

## Files Involved

- [libs/api/agent_handlers.go](libs/api/agent_handlers.go) - Main handler (needs fix)
- [libs/api/server.go:177](libs/api/server.go:177) - Route registration
- [fly.toml](fly.toml) - Deployment configuration (optional timeout increase)
- [/tmp/test_agent_upload.py](/tmp/test_agent_upload.py) - Test script

---

## Timeline

**If Option 1 works**: WASM upload fixed today ✅
**If Option 1 + 2 work**: WASM upload fixed today ✅
**If all fail**: Pivot to endpoint agents, fix WASM in parallel

**Target**: Beta launch ready by end of Week 1 with either WASM OR endpoint agents working.
