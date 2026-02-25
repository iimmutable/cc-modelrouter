# GLM Fix Test Commands

## Quick Reference

### Run All GLM Tests
```bash
go test -tags=integration_real -v ./test/integration/real_api/ -run "^TestGLM"
```

### Run Only Fix Validation Tests
```bash
go test -tags=integration_real -v ./test/integration/real_api/ -run "TestGLM.*Multiple"
```

### Run Specific Test
```bash
# The core fix test
go test -tags=integration_real -v ./test/integration/real_api/ -run TestGLMMultipleTextBlocks

# Real-world Claude Code test
go test -tags=integration_real -v ./test/integration/real_api/ -run TestGLMRealWorldClaudeCodeRequest

# Streaming test
go test -tags=integration_real -v ./test/integration/real_api/ -run TestGLMStreamingMultipleTextBlocks

# Mixed content test
go test -tags=integration_real -v ./test/integration/real_api/ -run TestGLMMixedContentWithMultipleTextBlocks
```

### Run with Coverage
```bash
go test -tags=integration_real -cover ./test/integration/real_api/ -run "^TestGLM"
```

## Environment Setup

### Required Environment Variable
```bash
export BIGMODEL_API_KEY="your-api-key-here"
```

### Verify API Key is Set
```bash
echo $BIGMODEL_API_KEY | wc -c
# Should show > 1 (not just 1 for newline)
```

## Test Files

### Fix Validation Tests
- `test/integration/real_api/glm_fix_test.go` - New tests for this fix

### Existing GLM Tests
- `test/integration/real_api/bigmodel_real_test.go` - Original GLM test suite

### Helper Files
- `test/integration/real_api/provider_tests.go` - Test infrastructure
- `test/integration/real_api/helpers.go` - API key management

## What to Look For

### ✅ Success Indicators
```
✓ GLM multiple text blocks test PASSED - fix is working correctly
✓ GLM mixed content test PASSED
✓ GLM successfully processed the merged text blocks
✓ Real-world Claude Code request test PASSED
```

### ❌ Failure Indicators
```
Response status: 400
400 Bad Request
ZenZGA/2.3
```

## Expected Test Duration

| Test Suite | Duration |
|------------|----------|
| Fix validation (4 tests) | ~11s |
| All GLM tests (12 tests) | ~32s |

## Binary Location

The fixed binary is at:
```
bin/ccrouter
```

Built: 2026-02-28 18:34 HKT
Size: ~16MB

## Manual Testing

### Start the Router
```bash
cd /Users/avextk/Documents/Code Projects/AICoding/cc-modelrouter
bin/ccrouter start
```

### Check Logs
```bash
# Latest log
tail -100 ~/.cc-modelrouter/logs/inst_*.log

# Watch for GLM requests
tail -f ~/.cc-modelrouter/logs/inst_*.log | grep GLM
```

### Stop the Router
```bash
bin/ccrouter stop --all
```

## Test Results Summary

**Last Run:** 2026-03-02 09:39 HKT
**Status:** ✅ ALL PASSED
**Tests:** 12/12 passed
**Duration:** 32.2s

See `GLM_FIX_VALIDATION.md` for detailed results.
