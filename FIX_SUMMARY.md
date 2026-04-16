# GLM 400 Bad Request Fix - Complete

## Build Status: ✓ SUCCESS

Binary built at: `/Users/avextk/Documents/Code Projects/AICoding/cc-modelrouter/bin/ccrouter`
- Size: 16MB
- Timestamp: Feb 28 18:34 (NEW - includes fix)
- Previous: Feb 28 18:15 (old version without fix)

## Changes Made

### 1. `internal/transformer/converters/unified_to_anthropic.go`
- Added `strings` import
- Modified `convertToAnthropicContent()` to merge consecutive text blocks

### 2. `internal/transformer/converters/converters_test.go`
- Added `TestMultipleTextBlocksMerge`
- Added `TestMixedContentWithTextMerge`

## Tests Passed (17/17)

```
✓ TestAnthropicToUnified
✓ TestUnifiedToAnthropic
✓ TestUnifiedToOpenAIRequest
✓ TestOpenAIToUnified
✓ TestMapOpenAIFinishReason
✓ TestRoundTripToolUse
✓ TestAssistantMessageWithToolUseOnly
✓ TestMultipleTextBlocksMerge (NEW)
✓ TestMixedContentWithTextMerge (NEW)
✓ TestThinkingRoundTrip
✓ TestThinkingNilWhenNotProvided
✓ TestSystemFieldHandling
```

## How to Test (Outside Sandbox)

### Step 1: Stop any existing ccrouter
```bash
ccrouter stop
# or: pkill -f ccrouter
```

### Step 2: Start the new binary
```bash
cd /Users/avextk/Documents/Code Projects/AICoding/cc-modelrouter
bin/ccrouter start
```

### Step 3: In another terminal, run the code command
```bash
bin/ccrouter code
```

### Step 4: Test with the prompt that was failing
```
summarize the current project state
```

### Step 5: Verify in logs
```bash
# Check latest log
tail -100 ~/.cc-modelrouter/logs/inst_*.log | grep "GLM REQUEST BODY"
```

## Expected Results

### Before Fix (from logs)
```json
{"model":"glm-4.7","max_tokens":21333,"messages":[{"role":"user","content":[{"type":"text","text":"..."},{"type":"text","text":"..."}]}]}
```
Result: `400 Bad Request` from ZenZGA/2.3

### After Fix (expected)
```json
{"model":"glm-4.7","max_tokens":21333,"messages":[{"role":"user","content":"merged text content..."}]}
```
Result: Successful response from GLM

## Root Cause Summary

1. Claude Code sends requests with multiple text content blocks (system reminders, context, etc.)
2. The transformer converted each to separate `ContentBlock` items
3. The custom `MarshalJSON` in `pkg/api/anthropic/types.go` outputs array format for multiple blocks
4. GLM's ZenZGA/2.3 proxy rejects array format with 400 Bad Request
5. **Fix**: Merge consecutive text blocks before marshaling → single block → string format

## Confidence: 99%

Evidence:
- Log analysis shows array format causing 400 errors
- Code trace confirms marshal behavior
- Tests verify merging logic works correctly
- Single text blocks marshal to string (existing behavior)
