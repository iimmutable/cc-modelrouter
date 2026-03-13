# Module Test Folder Enforcement Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce module test file placement in `<module>/test/` folders via pre-commit hook and documentation.

**Architecture:** A bash pre-commit hook validates staged test files against the convention, allowing test folders and white-box tests alongside source. CLAUDE.md is updated with clear rules.

**Tech Stack:** Bash, Git hooks

---

## File Structure

| File | Action | Purpose |
|------|--------|---------|
| `.githooks/pre-commit` | Create | Hook script to validate test file locations |
| `CLAUDE.md` | Modify | Add "Test File Organization" section |

---

## Chunk 1: Pre-commit Hook

### Task 1: Create Pre-commit Hook

**Files:**
- Create: `.githooks/pre-commit`

- [ ] **Step 1: Create .githooks directory**

```bash
mkdir -p .githooks
```

- [ ] **Step 2: Create the pre-commit hook script**

```bash
cat > .githooks/pre-commit << 'EOF'
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
EOF
```

- [ ] **Step 3: Make the hook executable**

```bash
chmod +x .githooks/pre-commit
```

- [ ] **Step 4: Configure git to use the hooks directory**

```bash
git config core.hooksPath .githooks
```

- [ ] **Step 5: Verify git configuration**

```bash
git config --get core.hooksPath
```

Expected output: `.githooks`

- [ ] **Step 6: Commit the hook**

```bash
git add .githooks/pre-commit
git commit -m "chore: add pre-commit hook for test file location validation"
```

---

## Chunk 2: Documentation Update

### Task 2: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md:212-217` (replace "Testing Patterns" section)

- [ ] **Step 1: Replace the Testing Patterns section**

Find the existing "Testing Patterns" section (lines 212-217):
```markdown
### Testing Patterns

- **Unit tests**: Alongside source files (`*_test.go`)
- **Table-driven tests**: For multiple scenarios
- **Integration tests**: In `test/integration/` with `-tags=integration`
- **Mock HTTP clients**: Use `httptest.NewServer` for provider testing
```

Replace with:
```markdown
### Testing Patterns

- **Table-driven tests**: For multiple scenarios
- **Mock HTTP clients**: Use `httptest.NewServer` for provider testing

### Test File Organization

Tests must follow Go's black-box/white-box testing patterns:

| Test Type | Location | Package | Tests |
|-----------|----------|---------|-------|
| **Black-box** | `<module>/test/` | `package <module>_test` | Exported members |
| **White-box** | `<module>/` (alongside source) | `package <module>` | Private + exported |
| **Cross-module** | `test/` at root | Varies | Multiple modules |

**Examples:**

| Location | Status | Reason |
|----------|--------|--------|
| `internal/proxy/test/handler_test.go` | ✓ Correct | Black-box test |
| `internal/proxy/handler_test.go` | ✓ Allowed | White-box test (private access needed) |
| `test/integration/provider_test.go` | ✓ Correct | Cross-module test |
| `internal/proxy/utils_test.go` (no `utils.go`) | ✗ Incorrect | Should be in `test/` subfolder |

**Pre-commit Hook:** A hook validates test file locations on commit. See `.githooks/pre-commit`.
```

- [ ] **Step 2: Verify the change**

```bash
git diff CLAUDE.md
```

- [ ] **Step 3: Commit the documentation update**

```bash
git add CLAUDE.md
git commit -m "docs: add test file organization rules to CLAUDE.md"
```

---

## Acceptance Criteria Verification

- [ ] **Verify hook allows test folder files**

Create a temp test file in a test folder and stage it:
```bash
mkdir -p internal/proxy/test
touch internal/proxy/test/dummy_test.go
git add internal/proxy/test/dummy_test.go
.githooks/pre-commit
```
Expected: No error (exit 0)
Cleanup: `git reset HEAD internal/proxy/test/dummy_test.go && rm -rf internal/proxy/test`

- [ ] **Verify hook allows root test folder files**

```bash
touch test/dummy_test.go
git add test/dummy_test.go
.githooks/pre-commit
```
Expected: No error (exit 0)
Cleanup: `git reset HEAD test/dummy_test.go && rm test/dummy_test.go`

- [ ] **Verify hook allows white-box tests (alongside source)**

```bash
# handler.go exists in internal/proxy/
touch internal/proxy/dummy_test.go
git add internal/proxy/dummy_test.go
.githooks/pre-commit
```
Expected: Info message "ℹ️ White-box test (allowed): internal/proxy/dummy_test.go" (exit 0)
Cleanup: `git reset HEAD internal/proxy/dummy_test.go && rm internal/proxy/dummy_test.go`

- [ ] **Verify hook rejects misplaced tests**

```bash
# Create test file with no corresponding source
mkdir -p internal/proxy
touch internal/proxy/orphan_test.go
git add internal/proxy/orphan_test.go
.githooks/pre-commit
```
Expected: Error message "❌ Test file misplaced" (exit 1)
Cleanup: `git reset HEAD internal/proxy/orphan_test.go && rm internal/proxy/orphan_test.go`

- [ ] **Verify git hooks path is configured**

```bash
git config --get core.hooksPath
```
Expected output: `.githooks`