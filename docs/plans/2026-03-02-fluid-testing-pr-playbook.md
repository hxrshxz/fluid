# Fluid Testing PR Playbook

Use this after writing tests/migration changes to avoid review churn.

Canonical migration coding reference:
- `docs/plans/2026-03-03-fluid-ginkgo-gomega-migration-patterns-and-gotchas.md`

Current execution policy:
- Dual-track phase: do migration + required new-test additions together per package wave.
- If coverage or reliability needs a new test file now, add it in the same wave (do not defer by default).

## Title Patterns

- `test(<pkg>): migrate tests to ginkgo/gomega`
- `test(<pkg>): improve unit coverage for <behavior>`
- `fix(test): stabilize <pkg> flaky tests`
- `refactor(test/<pkg>): improve readability and suite structure`

## Scope Rule

- Preferred: one package/wave per PR
- Good size: 1-4 files
- If >4 files, split by package or engine wave

## Canonical Upstream PR Template (Primary)

Use this as the main PR body structure. Keep sections I-V exactly so it matches upstream reviewer expectations.

```markdown
### Ⅰ. Describe what this PR does
- <1-3 bullets: migrated package scope, behavior parity, why this change matters>

### Ⅱ. Does this pull request fix one issue?
- <No issue linked / Yes: Fixes #<>>

### Ⅲ. List the added test cases and test result summary
- Added/updated cases:
  - `<case 1>`
  - `<case 2>`
- Result summary:
  - `make fmt`
  - `go test ./<changed-package>/... -count=1`
  - `go test ./pkg/controllers/... ./pkg/controllers/v1alpha1/... -count=1` (if controller-related)
  - Coverage before/after for changed scope: `<x% -> y%, +z%>`

### Ⅳ. Describe how to verify it
1. Checkout branch `<branch>`
2. Run `make fmt`
3. Run package tests: `go test ./<changed-package>/... -count=1`
4. If controllers changed, run: `go test ./pkg/controllers/... ./pkg/controllers/v1alpha1/... -count=1`
5. Confirm no `testify` import in touched migrated tests

### Ⅴ. Special notes for reviews
- N/A
```

## Practical Evidence Add-On (Recommended)

After filling I-V, append this short evidence block when the PR is migration or coverage-heavy.

```markdown
## Migration Evidence
- Package: `<pkg/path>`
- Files:
  - `<file1>`
  - `<file2>`
- Ginkgo/Gomega migration details: `<details>`
- Testify removed in touched scope: `<yes/no>`

## Checklist
- [ ] package-scoped PR
- [ ] no unresolved conflicts
- [ ] no Testify in touched scope
- [ ] no unnecessary benchmarks
- [ ] maintainer comments resolved
- [ ] requested substantive bot comments resolved
```

## Required Pre-PR Checks

1. `make fmt`
2. Target package tests
3. Controller lane tests if relevant
4. Quick coverage snapshot for changed scope
5. Confirm no `testify` import in touched test files

## Fast Gotcha Checklist

- Did I rebase and resolve conflicts?
- Is scope too broad?
- Did I include real behavior validation, not syntax-only migration?
- Did I clean env/global/patch state in tests?
- Is PR body complete with testing evidence?

## AI Reviewer Triage (Post-PR)

For bot/AI review comments, classify before applying:

- `must-fix`: correctness issue, failing-test cause, or clear policy violation.
- `optional`: readability/style improvement with no correctness impact.
- `ignore`: out-of-scope request, duplicate, contradicted by repo pattern, or hallucinated claim.

Rule:
- Apply only valid in-scope `must-fix` items.
- For ignored comments, record a one-line rationale in review response.
