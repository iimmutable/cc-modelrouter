#!/bin/bash
# Test script for usage tracking functionality
# This runs the integration tests for usage tracking with real routing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=========================================="
echo "Usage Tracking Integration Test"
echo "=========================================="
echo ""

# Check if test config exists
if [ ! -f "$PROJECT_ROOT/.cc-modelrouter/test.config.json" ]; then
    echo "⚠️  Test configuration not found at:"
    echo "   $PROJECT_ROOT/.cc-modelrouter/test.config.json"
    echo ""
    echo "The tests will be skipped if the config is missing."
    echo "To run the tests, create a test config file with your API keys."
    echo ""
fi

# Run the usage tracking integration tests
echo "Running usage tracking tests..."
echo ""

go test -tags=integration -v -run TestUsageTracking ./test/integration/...

echo ""
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo ""
echo "The following tests verify:"
echo "  ✓ Non-streaming request token tracking"
echo "  ✓ Streaming request token tracking"
echo "  ✓ Provider fallback tracking"
echo "  ✓ Concurrent request handling"
echo "  ✓ Buffered flush behavior"
echo ""
echo "All tests verify that:"
echo "  • Tokens are tracked from provider responses"
echo "  • Usage records are persisted to SQLite database"
echo "  • Route and model information is captured"
echo "  • Buffer flushes work correctly"
echo "  • Concurrent requests are handled safely"
