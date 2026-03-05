# Fluid Ginkgo/Gomega Migration Patterns and Gotchas

## Purpose and Scope

This is the canonical implementation guide for migrating Fluid unit tests from Testify or ad-hoc Go tests to Ginkgo v2 + Gomega with low review friction.

Use this document when:
- migrating existing UT files to Ginkgo/Gomega
- adding missing UT coverage in migrated packages
- fixing flaky tests while preserving behavior parity

Do not use this guide to justify broad refactors. Keep migrations package-scoped and behavior-preserving.

## Testify -> Ginkgo/Gomega Conversion Table

| Old style (Testify/stdlib) | Ginkgo/Gomega target | Notes |
|---|---|---|
| `func TestX(t *testing.T)` | `Describe("X", func() { ... })` | Group behavior by domain, not by helper function name. |
| `assert.Equal(t, a, b)` | `Expect(a).To(Equal(b))` | Prefer strict equality unless type/format flexibility is needed. |
| `require.NoError(t, err)` | `Expect(err).NotTo(HaveOccurred())` | Use early failure expectation at the step that produces error. |
| `assert.Error(t, err)` | `Expect(err).To(HaveOccurred())` | Pair with `MatchError` when message/type matters. |
| `assert.True(t, cond)` | `Expect(cond).To(BeTrue())` | For booleans only. For collections, prefer `HaveLen`, `ContainElement`, etc. |
| `assert.Nil(t, x)` | `Expect(x).To(BeNil())` | For interfaces/pointers; for error use `HaveOccurred`. |
| table loop + `t.Run` | `DescribeTable` / `Entry` | Keep each scenario visible in test output. |
| `time.Sleep(...)` wait | `Eventually(...)` / `Consistently(...)` | Never use blind sleeps for async assertions. |

## Code Comparison Examples

### 1) Basic assertion migration

Before (Testify):

```go
func TestParseRuntimeName(t *testing.T) {
    got, err := ParseRuntimeName("alluxio-default")
    require.NoError(t, err)
    assert.Equal(t, "alluxio", got)
}
```

After (Ginkgo/Gomega):

```go
var _ = Describe("ParseRuntimeName", func() {
    It("parses runtime prefix", func() {
        got, err := ParseRuntimeName("alluxio-default")
        Expect(err).NotTo(HaveOccurred())
        Expect(got).To(Equal("alluxio"))
    })
})
```

### 2) Table-driven style conversion

Before (`t.Run` loop):

```go
func TestIsSupportedEngine(t *testing.T) {
    tests := []struct {
        name string
        in   string
        ok   bool
    }{
        {name: "alluxio", in: "alluxio", ok: true},
        {name: "unknown", in: "x", ok: false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            assert.Equal(t, tt.ok, IsSupportedEngine(tt.in))
        })
    }
}
```

After (`DescribeTable`):

```go
var _ = Describe("IsSupportedEngine", func() {
    DescribeTable("engine support",
        func(in string, ok bool) {
            Expect(IsSupportedEngine(in)).To(Equal(ok))
        },
        Entry("alluxio", "alluxio", true),
        Entry("unknown", "x", false),
    )
})
```

### 3) Async/eventual checks vs sleep

Before (sleep-based):

```go
It("updates dataset phase", func() {
    triggerReconcile(obj)
    time.Sleep(2 * time.Second)
    got := readPhase(obj.Name)
    Expect(got).To(Equal(v1alpha1.DatasetBound))
})
```

After (eventually-based):

```go
It("updates dataset phase", func() {
    triggerReconcile(obj)

    Eventually(func() v1alpha1.DatasetPhase {
        return readPhase(obj.Name)
    }, "10s", "200ms").Should(Equal(v1alpha1.DatasetBound))
})
```

### 4) Setup/cleanup with BeforeEach/AfterEach/DeferCleanup

Before (manual cleanup risk):

```go
func TestNodeLabelPatch(t *testing.T) {
    old := os.Getenv("RUNTIME_NAMESPACE")
    _ = os.Setenv("RUNTIME_NAMESPACE", "fluid")
    defer os.Setenv("RUNTIME_NAMESPACE", old)

    monkey.Patch(getNodeName, func() string { return "n1" })
    defer monkey.UnpatchAll()

    // test body
}
```

After (structured cleanup):

```go
var _ = Describe("node label patch", func() {
    BeforeEach(func() {
        old := os.Getenv("RUNTIME_NAMESPACE")
        Expect(os.Setenv("RUNTIME_NAMESPACE", "fluid")).To(Succeed())
        DeferCleanup(func() {
            Expect(os.Setenv("RUNTIME_NAMESPACE", old)).To(Succeed())
        })

        monkey.Patch(getNodeName, func() string { return "n1" })
        DeferCleanup(func() { monkey.UnpatchAll() })
    })

    It("applies labels", func() {
        // test body
    })
})
```

## Package-Specific Guidance

### Controllers (`pkg/controllers`, `pkg/controllers/v1alpha1`)

- Prefer behavior-level specs: reconcile inputs -> status/spec side effects.
- Isolate fake client setup in `BeforeEach`, not per assertion block.
- For phase/status transitions, use `Eventually` when reconcile is asynchronous.
- Keep controller lane verification command in PR evidence when touched.

### DDC Engines (`pkg/ddc/*`)

- Focus on transform/output correctness for engine-specific manifests and options.
- Keep table-driven coverage for engine permutations and invalid input paths.
- Assert on exact critical fields, not whole huge objects unless stable.

### Webhook/Plugins (`pkg/webhook/*`, `pkg/application/inject/fuse/*`)

- Test mutation idempotency and required field insertion explicitly.
- Validate plugin precedence/conflict behavior with clear scenario names.
- Avoid brittle full JSON string matches; assert key mutated fragments.

### DataOperation (`pkg/dataoperation/*`, related status handlers)

- Cover state machine transitions and terminal/error branches.
- Use explicit context setup in `BeforeEach` and cleanup with `DeferCleanup`.
- Ensure retries/time-based behavior uses controllable clocks or eventual assertions.

## Gotchas (Symptom -> Likely Cause -> Fix)

- Test passes locally, flakes in CI -> hidden global/env leakage -> move setup to `BeforeEach` and restore with `DeferCleanup`.
- Migration compiles but reviewer requests rewrite -> syntax-only conversion -> add behavior scenarios and failure-path assertions.
- Intermittent async failures -> `time.Sleep` guesswork -> replace with bounded `Eventually` and meaningful polling interval.
- Review flags unclear failure output -> vague `Expect(err).To(HaveOccurred())` only -> add `MatchError` or stronger state assertions.
- PR grows too large and stalls -> multi-package migration wave -> split by package/engine and keep 1-4 files when possible.
- Controller tests fail after migration -> suite setup drift -> align fake client/scheme registration with existing package suite conventions.

## Reviewer Checklist for Migrated Tests

- [ ] Scope is package-focused and migration intent is clear.
- [ ] No new `testify` imports in touched migration scope.
- [ ] Behavior parity preserved (success + critical failure paths covered).
- [ ] Async checks use `Eventually`/`Consistently`, not raw sleeps.
- [ ] Env/global/patch state is cleaned with `DeferCleanup` or equivalent.
- [ ] Assertions are specific enough to debug failures quickly.
- [ ] Controller lane tests included if controller code/tests are touched.

## LLM Execution Protocol 

Follow these steps exactly for safe migration.

1. Lock scope to one package (or one engine wave) and list touched test files.
2. For each file, preserve scenario intent before rewriting assertion framework.
3. Convert structure:
   - `TestX` -> `Describe/Context/It`
   - `t.Run` loops -> `DescribeTable/Entry` where useful
4. Convert assertions with direct mapping recipe:
   - `require.NoError` -> `Expect(err).NotTo(HaveOccurred())`
   - `assert.Equal` -> `Expect(actual).To(Equal(expected))`
   - `assert.Contains` -> `Expect(actual).To(ContainSubstring(...))` or collection matcher
5. Replace `time.Sleep` synchronization with `Eventually`/`Consistently`.
6. Normalize setup/teardown:
   - shared fixtures in `BeforeEach`
   - cleanup in `DeferCleanup`
7. Run formatter and package tests; only then adjust flaky timing bounds if needed.
8. Verify touched migrated files have no `testify` import.
9. Produce concise PR evidence with upstream I-V template and explicit test commands.

### Non Negotiable (Do/Don't Rules)

Do:
- keep behavior unchanged while migrating framework style
- keep diffs local and package-scoped
- prefer explicit matcher intent over generic bool checks

Do not:
- perform opportunistic refactors unrelated to failing/migrated tests
- add benchmarks in migration-only PRs
- replace deterministic checks with long sleeps
- hide assertion meaning in helper layers unless already established in package style

YOU MUST Treat this document as the primary coding-quality reference for migration work.
