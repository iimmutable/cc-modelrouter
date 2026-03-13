# Module Test Folder Enforcement Design

**Date:** 2026-03-15
**Status:** Draft

## Problem

Module test files are inconsistently placed alongside source files rather than in dedicated test folders. This makes it harder to:
- Distinguish test files from source files at a glance
- Maintain a clean module structure
- Enforce consistent conventions across the codebase

## Solution

Enforce a convention where module tests are placed in `<module>/test/` folders, with pre-commit hook validation. White-box tests (testing private members) may remain alongside source files as an exception.

## Go Testing Context

This design follows Go's black-box/white-box testing patterns:

| Test Type | Package | Location | Tests |
|-----------|---------|----------|-------|
| **Black-box** | `package <module>_test` | `<module>/test/` | Exported members only |
| **White-box** | `package <module>` | `<module>/` (alongside source) | Private + exported members |

Tests in `<module>/test/` use an external test package and can only test exported members. If a test needs access to unexported members, it may remain alongside the source file.

## Convention

### Module Tests (Black-box)
Place black-box tests for a module in `<module>/test/`:

```
internal/proxy/test/           # proxy module black-box tests
internal/router/test/          # router module black-box tests
internal/transformer/test/     # transformer module black-box tests
pkg/api/anthropic/test/        # API types black-box tests
```

**Package declaration:** Use `package <module>_test` (external test package)

### White-box Tests (Exception)
Tests that need access to private members may remain alongside source files:

```
internal/proxy/handler_test.go    # White-box test (package proxy)
```

**Package declaration:** Use `package <module>` (same as source)

### Cross-Module Tests (Exception)
Use `test/` at project root for tests that span multiple modules:

```
test/integration/              # cross-module integration tests
test/security/                 # security tests
test/openrouter_test.go        # full-stack provider tests
```

### Examples

| Location | Status | Reason |
|----------|--------|--------|
| `internal/proxy/test/handler_test.go` | ✓ Correct | Black-box test in test folder |
| `internal/proxy/handler_test.go` | ✓ Allowed | White-box test (needs private access) |
| `test/integration/provider_quirks/` | ✓ Correct | Cross-module test at root |

## Implementation

### 1. Documentation Update (CLAUDE.md)

Add a new "Test File Organization" section with:
- Clear rules for black-box vs white-box vs cross-module tests
- Examples of correct and incorrect placements
- Reference to the pre-commit hook

### 2. Pre-commit Hook

**Location:** `.githooks/pre-commit`

**Script:**
```bash
#!/bin/bash

# Check for misplaced test files
# Allows: */test/*.go (module test folders)
# Allows: test/*.go (root test folder)
# Allows: *_test.go alongside source (white-box tests)
# Rejects: test files not following these patterns

MISPLACED=0

# Get all staged test files (excluding deleted)
STAGED_TESTS=$(git diff --cached --name-only --diff-filter=ACM | grep '_test\.go$' || true)

for FILE in $STAGED_TESTS; do
    DIR=$(dirname "$FILE")
    BASENAME=$(basename "$FILE")

    # Allowed locations:
    # 1. In a test/ subfolder (e.g., internal/proxy/test/)
    # 2. In root test/ folder
    # 3. Alongside source files (white-box tests) - but warn

    if [[ "$DIR" == "test" || "$DIR" == */test || "$DIR" == test/* ]]; then
        # In a test folder - allowed
        continue
    fi

    # Check if this is alongside source (white-box test)
    SOURCE_FILE="${FILE%_test.go}.go"
    if git ls-files --error-unmatch "$SOURCE_FILE" >/dev/null 2>&1; then
        # White-box test alongside source - warn but allow
        echo "ℹ️  White-box test (allowed): $FILE"
        continue
    fi

    # Misplaced test file
    echo "❌ Test file misplaced: $FILE"
    PARENT_DIR=$(dirname "$FILE")
    echo "   Module tests should be in: ${PARENT_DIR}/test/"
    MISPLACED=1
done

if [ $MISPLACED -eq 1 ]; then
    echo ""
    echo "Commit rejected. Move test files to appropriate test/ folders."
    echo "If you need to test private members, keep the test alongside the source."
    exit 1
fi

exit 0
```

**Configuration:**
```bash
git config core.hooksPath .githooks
chmod +x .githooks/pre-commit
```

### 3. Edge Cases

| Case | Rule | Location |
|------|------|----------|
| `testdata/` | Keep alongside tests | `<module>/test/testdata/` or `<module>/testdata/` |
| Benchmark tests (`*_bench_test.go`) | Same rules as unit tests | `<module>/test/` preferred |
| Fuzz tests (`func FuzzXxx`) | Same rules as unit tests | `<module>/test/` preferred |
| Example tests (`func ExampleXxx`) | Same rules as unit tests | `<module>/test/` preferred |
| Test helpers (`testing.go`) | In test folder | `<module>/test/testing.go` |
| Nested module tests | Follow same rules per subdirectory | `<module>/subdir/test/` or alongside source |
| Multi-file tests (no single source) | Must be in test folder | `<module>/test/` or `test/` |
| Package declaration | Must match location | `*/test/` → `package <module>_test`, alongside → `package <module>` |

### 4. File Structure

```
.githooks/
└── pre-commit          # Hook script

CLAUDE.md               # Updated with test organization rules
```

## Migration Plan

Existing tests are not automatically migrated. The enforcement applies to:
- New test files created after this design is implemented
- Modified test files should be evaluated for migration

**Decision checklist when modifying an existing test:**

```
1. Does it test only exported members?
   → Move to <module>/test/
   → Update package to <module>_test

2. Does it test private members?
   → Keep alongside source
   → Ensure package is <module>

3. Does it span multiple modules?
   → Move to test/ at root

4. No single corresponding source file?
   → Move to <module>/test/ or test/ at root
```

## Files to Create/Modify

| File | Action |
|------|--------|
| `CLAUDE.md` | Add "Test File Organization" section |
| `.githooks/pre-commit` | Create hook script |

## Acceptance Criteria

- [ ] CLAUDE.md contains clear test organization rules
- [ ] Pre-commit hook detects misplaced test files
- [ ] Hook allows files in `*/test/` folders
- [ ] Hook allows files in root `test/` folder
- [ ] Hook allows white-box tests alongside source files (with info message)
- [ ] Hook rejects misplaced test files (not in test/ and not alongside source)
- [ ] Git is configured to use `.githooks/` directory