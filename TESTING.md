# Testing Instructions for "[object Object]" Fix

## Build the Project

```bash
# From the project root:
cd /Users/avextk/Documents/Code\ Projects/AICoding/cc-modelrouter

# Build the binary:
go build -o bin/ccrouter ./cmd/ccrouter

# Or clean build:
go clean -cache
go build -a -o bin/ccrouter ./cmd/ccrouter
```

## Run Unit Tests

```bash
# Run all unit tests:
go test ./...

# Run specific proxy tests:
go test ./internal/proxy -v

# Run the new invalid JSON test:
go test ./internal/proxy -run TestTryStreamingTarget_InvalidJSONInStream -v
```

## Run Integration Tests

```bash
# Run integration tests (requires build tag):
go test -tags=integration ./test -v

# Run the SSE integration test:
go test -tags=integration ./test -run TestIntegrationStreamingSSE -v
```

## Manual Testing

### 1. Start the Router

```bash
# Using the test config:
./bin/ccrouter code --config .cc-modelrouter/test.config.json
```

### 2. Send a Test Request

In another terminal:

```bash
# Test streaming request:
curl -N -X POST http://localhost:18081/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: test-key" \
  -d '{
    "model": "glm-4.7",
    "max_tokens": 100,
    "stream": true,
    "messages": [{"role": "user", "content": "Say hello"}]
  }'
```

### 3. Verify the Fix

**Before Fix:**
```
[object Object]
[object Object]
[object Object]
```

**After Fix:**
```
event: message_start
data: {"type":"message_start","message":{...}}

event: content_block_start
data: {"type":"content_block_start",...}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}

event: content_block_stop
data: {"type":"content_block_stop",...}

event: message_stop
data: {"type":"message_stop"}
```

## Debug Logging

The fix adds detailed logging. You should see:

```
[STREAM] Emitting message_start event: {"type":"message_start",...}
[STREAM] Emitting content_block_start event: {"type":"content_block_start",...}
[STREAM] Starting stream to bigmodel/glm-4.7
[STREAM] Stream completed successfully
```

If invalid JSON is received:

```
[STREAM] Invalid JSON data from provider, skipping: [object Object]
[STREAM] Skipping invalid JSON event data, type: content_block_delta
```

## Test Coverage

The following tests verify the fix:

1. **TestTryStreamingTarget_InvalidJSONInStream** - New test
   - Verifies invalid JSON is handled gracefully
   - Verifies "[object Object]" doesn't appear in response
   - Verifies synthetic events are emitted correctly

2. **TestIntegrationStreamingSSE** - Existing test
   - Verifies complete SSE event sequence
   - Verifies message_start and content_block_start events
   - Verifies proper event ordering

## Expected Results

✅ No "[object Object]" responses
✅ Proper SSE event sequence
✅ Valid JSON in all event data
✅ Debug logs showing synthetic events
✅ Invalid JSON is logged and skipped
