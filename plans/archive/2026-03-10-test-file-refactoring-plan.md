# Test File Refactoring Plan - ABANDONED

## Status: NOT VIABLE - Technical Incompatibility with Go

This refactoring approach has been abandoned due to fundamental incompatibility with Go's package system.

## Problem

In Go, **each directory is a separate package**. When tests are moved to `<module>/test/` subdirectories:

- Tests with `package <module>` cannot access the parent package's unexported members
- This breaks all white-box tests that test internal functions/types
- The only working pattern is `package <module>_test` which only allows testing exported APIs

## Evidence

After moving 41 test files to `<module>/test/` subdirectories, ALL tests failed with "undefined" errors for unexported types and functions:

```
internal/config/test/loader_test.go:37:14: undefined: Load
internal/interceptor/test/max_token_test.go:12:17: undefined: NewMaxTokenInterceptor
internal/transformer/test/base_test.go:11:8: undefined: NewBaseTransformer
```

## Go Testing Conventions

### White-Box Tests (Current Pattern - Recommended)
```
internal/config/
├── config.go
├── loader.go
├── loader_test.go      # package config - can test internals
└── types_test.go       # package config - can test internals
```

### Black-Box Tests (Alternative - Not Recommended)
```
internal/config/
├── config.go
└── test/
    └── api_test.go     # package config_test - only exported APIs
```

## Why This Refactoring Doesn't Work

1. **White-box tests need same package**: Tests accessing internal functions must be in the same directory as the source
2. **Go treats directories as packages**: `<module>/test/` is a different package than `<module>/`
3. **Mixing test types in one directory**: Can't have both `package <module>` and `package <module>_test` files in the same directory

## Recommendation: KEEP CURRENT STRUCTURE

The current test organization is actually correct for Go:
- Tests alongside source files (e.g., `loader_test.go` next to `loader.go`)
- This allows white-box testing of internal functions
- Go automatically discovers test files regardless of location

## Alternative Approaches (If Organization is Still Desired)

### Option 1: Category-Based Test Organization
Organize by test type instead of by module:
```
test/
├── unit/              # Run with go test ./internal/...
├── integration/       # Already exists
└── util/              # Already exists
```

### Option 2: Black-Box Testing Only
Convert all tests to black-box (requires significant rewrites):
- Rename all tests to `package <module>_test`
- Import parent packages
- Only test exported APIs
- Loss of internal testing coverage

### Option 3: Separate Exported Internal Packages
Create separate `internal` packages for testable internals:
- More complex architecture
- May expose too much internally
- Not recommended for this codebase

## Conclusion

**The current test file organization is correct and follows Go best practices.**

The documentation mentioning `<module>/test/` appears to be aspirational rather than reflective of actual Go conventions. The existing structure where tests are alongside source files (`*_test.go`) is the standard Go pattern and should be maintained.

## Action Taken

All changes have been reverted. Tests remain in their original locations alongside source files.
