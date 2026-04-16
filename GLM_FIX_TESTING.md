# Testing Instructions for GLM 400 Bad Request Fix

## What Was Fixed

Modified `internal/transformer/converters/unified_to_anthropic.go` to merge consecutive text blocks when converting from Unified to Anthropic format. This prevents GLM's ZenZGA/2.3 proxy from rejecting requests with array-format content.

## Files Modified

1. `internal/transformer/converters/unified_to_anthropic.go`
   - Added `strings` import
   - Modified `convertToAnthropicContent` to merge consecutive text blocks

2. `internal/transformer/converters/converters_test.go`
   - Added `TestMultipleTextBlocksMerge`
   - Added `TestMixedContentWithTextMerge`

## Build Command

```bash
cd /Users/avextk/Documents/Code Projects/AICoding/cc-modelrouter
go build -o bin/ccrouter ./cmd/ccrouter
```

## Test Command

```bash
# Start the router
bin/ccrouter start

# In another terminal, test with code command
bin/ccrouter code

# Then prompt with: "summarize the current project state"
```

## Expected Result

| Before Fix | After Fix |
|------------|-----------|
| 400 Bad Request error from GLM | Successful response from GLM |
| `"content":[{"type":"text","text":"..."},{"type":"text","text":"..."}]` | `"content":"merged text content..."` |

## Log Verification

Check `~/.cc-modelrouter/logs/` for the latest log file:
```bash
ls -lt ~/.cc-modelrouter/logs/ | head -5
tail -100 ~/.cc-modelrouter/logs/inst_*.log | grep "GLM REQUEST BODY"
```

## Tests Passed

All converter tests pass (17 tests):
- TestAnthropicToUnified ✓
- TestUnifiedToAnthropic ✓
- TestRoundTripToolUse ✓
- TestMultipleTextBlocksMerge ✓ (NEW)
- TestMixedContentWithTextMerge ✓ (NEW)
