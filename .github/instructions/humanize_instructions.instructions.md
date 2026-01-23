---
applyTo: '**'
---
# Fluid LFX #5407 — LEGENDARY Test Migration & Coverage Guidelines

> **"YOU MUST" = Non-negotiable. These aren't suggestions. Follow them or the output is wrong.**

---

## MISSION BRIEF

You are an expert Go developer contributing to the Fluid project (LFX #5407). This is a **MIGRATION PROJECT**:

| Priority | Goal | Status |
|----------|------|--------|
| **PRIMARY** | Migrate from testify/Convey → Ginkgo + Gomega | Active |
| **SECONDARY** | Increase coverage from ~57% → 70% | Ongoing |

---

## ⚠️ CRITICAL REQUIREMENTS (YOU MUST FOLLOW)

### Framework Selection
```
YOU MUST use Ginkgo/Gomega for ALL new test code.
YOU MUST NOT use Convey, testify, or standard Go tests for new tests.
YOU MUST NOT mix multiple test frameworks within a single file.
```

### When Adding Tests to Existing Files
```
YOU MUST check what framework the existing file uses.
YOU MUST choose ONE of these approaches:
  - Option A (PREFERRED): Migrate the ENTIRE file to Ginkgo
  - Option B: Create a NEW separate `*_ginkgo_test.go` file
YOU MUST NOT add Convey/testify code to ANY file (even existing Convey files).
```

### Code Quality Non-Negotiables
```
YOU MUST NOT use underscores in test function names.
  ✅ TestValidateInput
  ❌ Test_Validate_Input
  
YOU MUST capture ALL return values. Never use _ to discard.
  ✅ result, err := Function()
  ❌ _, err := Function()
  
YOU MUST call defer patches.Reset() after EVERY gomonkey patch.

YOU MUST add comment inside empty mock functions:
  func (m *Mock) Method() { // No-op for test mock }
  
YOU MUST keep cognitive complexity ≤ 15 in test functions.

YOU MUST define const for any string literal used > 2 times.
```

### Test Coverage Requirements
```
YOU MUST test the happy path (success case).
YOU MUST test error cases (what happens when things fail).
YOU MUST test edge cases (nil, empty, zero values, invalid input).
YOU MUST verify cleanup happens in defer blocks.
YOU MUST verify all side effects (mocked methods called as expected).
```

### Naming Requirements
```
YOU MUST start test function names with "Test" prefix.
YOU MUST use CamelCase (no underscores).
YOU MUST NOT include redundant package prefix in function names.
  ✅ TestValidate
  ❌ TestJuiceFSEngineValidate (redundant)
  
YOU MUST use descriptive test case names in Describe/Context/It.
YOU MUST start test case descriptions with lowercase.
```

### Before Committing
```
YOU MUST run: make fmt
YOU MUST run: go test ./pkg/ddc/[package]/... -v
YOU MUST verify all tests pass with zero failures.
YOU MUST include DCO sign-off in commit message.
YOU MUST NOT include issue number (#5407) in commit message body.
```

### ⚠️ Ginkgo Suite Management (CRITICAL)
```
YOU MUST have ONLY ONE RunSpecs() call per Go package.
YOU MUST check if the package already has a suite_test.go or *_suite_test.go before adding RunSpecs().
YOU MUST NOT add func TestXxx(t *testing.T) { RunSpecs(...) } to individual test files if a suite file exists.
YOU MUST use var _ = Describe(...) for test specs - they register automatically with the suite.

CORRECT PATTERN:
  - Create ONE suite_test.go per package with RunSpecs()
  - All other *_test.go files use only var _ = Describe(...) blocks
  - No individual test files call RunSpecs()

WRONG PATTERN (causes CI failure "Rerunning Suite"):
  - types_test.go has func TestTypes(t *testing.T) { RunSpecs(...) }
  - node_test.go has func TestNode(t *testing.T) { RunSpecs(...) }
  - Both files run ALL specs, causing Ginkgo to detect duplicate suite execution

BEFORE MIGRATING A FILE:
  1. Check: ls pkg/ddc/[package]/*suite*.go
  2. If suite exists → DO NOT add RunSpecs() to your file
  3. If no suite exists → Create suite_test.go with RunSpecs()
  4. Your test file should ONLY contain var _ = Describe(...) blocks
```

---

## GINKGO/GOMEGA PATTERN (USE THIS ALWAYS)

```go
package pkgname_test

import (
    "testing"
    
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "github.com/agiledragon/gomonkey/v2"
    
    // Import the package you're testing
    "github.com/fluid-cloudnative/fluid/pkg/ddc/pkgname"
)

func TestPkgname(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Pkgname Suite")
}

var _ = Describe("FunctionName", func() {
    var (
        engine  *pkgname.Engine
        patches *gomonkey.Patches
    )
    
    BeforeEach(func() {
        engine = &pkgname.Engine{}
    })
    
    AfterEach(func() {
        if patches != nil {
            patches.Reset()
        }
    })
    
    Context("when operation succeeds", func() {
        BeforeEach(func() {
            patches = gomonkey.ApplyFunc(someFunc, func() error {
                return nil
            })
        })
        
        It("should return expected result", func() {
            result, err := engine.Method()
            Expect(err).NotTo(HaveOccurred())
            Expect(result).To(Equal(expected))
        })
    })
    
    Context("when dependency fails", func() {
        BeforeEach(func() {
            patches = gomonkey.ApplyFunc(someFunc, func() error {
                return errors.New("mock error")
            })
        })
        
        It("should return error", func() {
            result, err := engine.Method()
            Expect(err).To(HaveOccurred())
            Expect(err.Error()).To(ContainSubstring("mock error"))
        })
    })
    
    Context("when input is nil", func() {
        It("should handle gracefully", func() {
            result, err := engine.Method(nil)
            Expect(err).To(HaveOccurred())
        })
    })
    
    Context("when input is empty", func() {
        It("should return empty result", func() {
            result, err := engine.Method("")
            Expect(err).NotTo(HaveOccurred())
            Expect(result).To(BeEmpty())
        })
    })
})
```

---

## GOMEGA MATCHERS CHEAT SHEET

### Equality
```go
Expect(x).To(Equal(y))              // Deep equality
Expect(x).To(BeEquivalentTo(y))     // Type-converting equality
Expect(x).To(BeIdenticalTo(y))      // Same object (pointer)
```

### Nil/Empty/Zero
```go
Expect(x).To(BeNil())
Expect(x).To(BeEmpty())             // "", [], map{}
Expect(x).To(BeZero())              // Zero value for type
```

### Boolean
```go
Expect(x).To(BeTrue())
Expect(x).To(BeFalse())
```

### Errors
```go
Expect(err).To(HaveOccurred())      // err != nil
Expect(err).NotTo(HaveOccurred())   // err == nil
Expect(err).To(MatchError("msg"))   // Error message contains
```

### Strings
```go
Expect(s).To(ContainSubstring("x"))
Expect(s).To(HavePrefix("x"))
Expect(s).To(HaveSuffix("x"))
Expect(s).To(MatchRegexp("pattern"))
```

### Collections
```go
Expect(slice).To(HaveLen(3))
Expect(slice).To(ContainElement(x))
Expect(slice).To(ContainElements(x, y, z))
Expect(map).To(HaveKey("key"))
Expect(map).To(HaveKeyWithValue("key", "value"))
```

### Numeric
```go
Expect(x).To(BeNumerically(">", 5))
Expect(x).To(BeNumerically("~", 5, 0.1))  // Within tolerance
```

---

## GOMONKEY MOCKING PATTERNS

### Mock a Function
```go
patches := gomonkey.ApplyFunc(targetFunc, func(args...) returnType {
    return mockValue
})
defer patches.Reset()
```

### Mock a Method
```go
patches := gomonkey.ApplyMethod(reflect.TypeOf(&instance), "MethodName",
    func(_ *Type, args...) returnType {
        return mockValue
    })
defer patches.Reset()
```

### Mock Multiple Things
```go
patches := gomonkey.NewPatches()
patches.ApplyFunc(func1, mockFunc1)
patches.ApplyFunc(func2, mockFunc2)
patches.ApplyMethod(reflect.TypeOf(&obj), "Method", mockMethod)
defer patches.Reset()
```

### Sequential Return Values
```go
patches := gomonkey.ApplyFuncSeq(targetFunc, []gomonkey.OutputCell{
    {Values: gomonkey.Params{nil, errors.New("first call fails")}},
    {Values: gomonkey.Params{"success", nil}},  // Second call succeeds
})
defer patches.Reset()
```

---

## MIGRATION STRATEGY

### Decision Tree
```
Is this a NEW test file?
├── YES → YOU MUST use Ginkgo/Gomega
└── NO → Does existing file use Ginkgo?
    ├── YES → Add tests in Ginkgo format
    └── NO → How large is the file?
        ├── < 100 lines → Migrate ENTIRE file to Ginkgo
        └── ≥ 100 lines → Create NEW *_ginkgo_test.go file
```

### Migration Template (Converting Convey → Ginkgo)

**Before (Convey):**
```go
func TestSomething(t *testing.T) {
    Convey("Test something", t, func() {
        Convey("when X", func() {
            So(result, ShouldEqual, expected)
        })
    })
}
```

**After (Ginkgo):**
```go
var _ = Describe("Something", func() {
    Context("when X", func() {
        It("should equal expected", func() {
            Expect(result).To(Equal(expected))
        })
    })
})
```

---

## PR WORKFLOW

### 1. Branch Creation
```bash
git checkout -b test/[package]-[function]
# Example: test/juicefs-validate
```

### 2. Commit Message Format
```
test(pkg/ddc/[package]): add Ginkgo tests for [function]

Signed-off-by: <harshmastic@gmail.com>
```

**YOU MUST NOT include issue number in commit message.**

### 3. PR Description Template
```markdown
### Ⅰ. Describe what this PR does
test(pkg/ddc/[package]): add Ginkgo unit tests for [function]

Adds comprehensive test coverage using Ginkgo/Gomega as part of the 
framework migration effort. Covers success paths, error handling, 
and edge cases.

### Ⅱ. Does this pull request fix one issue?
part of #5407

### Ⅲ. List the added test cases
| Test Case | Description |
|-----------|-------------|
| success case | validates normal operation with valid input |
| error handling | verifies behavior when dependency fails |
| nil input | ensures graceful handling of nil values |
| empty input | confirms correct response to empty data |

### Ⅳ. Describe how to verify it
\`\`\`bash
go test ./pkg/ddc/[package]/... -v -cover
\`\`\`

### Ⅴ. Special notes for reviews
- Uses Ginkgo/Gomega per #5407 migration goals
- All return values captured and verified
- gomonkey patches properly cleaned up with defer Reset()
- Cognitive complexity kept below 15
```

---

## CODE REVIEW RESPONSES

### When Reviewer Suggests Ginkgo Migration
```markdown
✅ "Good point! I've migrated the tests to Ginkgo/Gomega to align with #5407."
```

### When Reviewer Suggests Naming Changes
```markdown
✅ "Updated the function names as suggested. Thanks for the catch!"
```

### When Reviewer Points Out Missing Error Cases
```markdown
✅ "Added error case coverage. Now testing: [list new cases]"
```

### When CI Fails (gofmt, golint)
```markdown
✅ "Fixed formatting issues. Ran `make fmt` and pushed the fix."
```

---

## FORBIDDEN ACTIONS (YOU MUST NOT)

| ❌ NEVER DO THIS | WHY |
|------------------|-----|
| Mix frameworks in one file | Creates maintenance nightmare |
| Use underscores in test names | SonarQube violation |
| Discard return values with `_` | Misses errors, reduces coverage |
| Forget `defer patches.Reset()` | Causes test pollution |
| Skip error case testing | Leaves bugs undetected |
| Add issue numbers to commits | Project convention |
| Refactor production code | Scope creep, separate PR |
| Add new dependencies | Review burden |
| Skip `make fmt` | CI will fail |
| Create order-dependent tests | Flaky tests |
| Add multiple RunSpecs() per package | CI fails with "Rerunning Suite" |

---

## PRE-SUBMIT CHECKLIST

```
YOU MUST verify each item before submitting:

[ ] Framework: Using Ginkgo/Gomega (not Convey/testify)
[ ] No mixed frameworks in any single file
[ ] Only ONE RunSpecs() per package (check for existing suite_test.go)
[ ] No underscores in test function names
[ ] All return values captured (no `_`)
[ ] Every gomonkey patch has `defer patches.Reset()`
[ ] Empty mock functions have comments inside
[ ] Happy path tested
[ ] Error cases tested  
[ ] Edge cases tested (nil, empty, zero, invalid)
[ ] Ran `make fmt` - no formatting issues
[ ] Ran `go test ./pkg/...` - all tests pass
[ ] Commit message has DCO sign-off
[ ] Commit message has NO issue number
[ ] PR description complete with all sections
[ ] Cognitive complexity ≤ 15 in all functions
```

---

## VERIFICATION COMMANDS

```bash
# Format code (YOU MUST run before commit)
make fmt

# Run tests for specific package
go test ./pkg/ddc/[package]/... -v

# Run tests with coverage
go test ./pkg/ddc/[package]/... -v -cover

# Generate coverage report
go test ./pkg/ddc/[package]/... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run all project tests
go test ./pkg/... -v

# Check for race conditions
go test ./pkg/ddc/[package]/... -race
```

---

## SUMMARY

**YOU MUST** use Ginkgo/Gomega for all new tests. The project is migrating away from Convey/testify. When you encounter legacy tests, migrate them or create separate Ginkgo files. Follow all "YOU MUST" requirements - they are not suggestions. Every PR should be perfect: zero formatting issues, zero test failures, comprehensive coverage, proper documentation.

**The goal:** Write tests so clean and thorough that no reviewer - human or AI - can find a single flaw.

