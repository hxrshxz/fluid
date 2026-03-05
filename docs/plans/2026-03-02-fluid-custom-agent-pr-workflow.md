# Fluid Custom Agent Workflow (Testing PRs)

Use this as a concise implementation-first instruction source for a custom PR agent.

## Purpose

Implement migration/coverage changes first, then prepare PR artifacts with low review friction and explicit quality evidence.

## Agent Contract

The agent must:

1. Keep PR scope package-focused (one package/wave where possible).
2. Follow the canonical coding guide for touched tests.
3. Avoid Testify in touched migration scope.
4. Include deterministic test hygiene (env/global/patch cleanup where relevant).
5. Never open PR without explicit user confirmation.

## Required Inputs

- Current branch diff
- Target scope/package
- Testing and coverage outputs
- Canonical coding reference:
  - `docs/plans/2026-03-03-fluid-ginkgo-gomega-migration-patterns-and-gotchas.md`
- Supporting PR/process references:
  - `docs/plans/2026-03-02-fluid-testing-pr-playbook.md`
  - `docs/plans/2026-03-02-fluid-testing-pr-merged-analysis.md`
  - `docs/plans/2026-03-02-fluid-mentor-final-plan.md`

## Implementation-First Execution Steps

1. Analyze changed files and classify task type: `migration`, `coverage`, `fix`, or `mixed`.
2. Implement/migrate tests using the canonical pattern guide.
3. Validate scope size:
   - good: 1-4 files/package wave
   - if too broad: suggest split before PR prep
4. Run checks:
   - `make fmt`
   - `go test ./<changed-package>/... -count=1`
   - if controllers changed: `go test ./pkg/controllers/... ./pkg/controllers/v1alpha1/... -count=1`
5. Verify no Testify in touched migrated scope.
6. Build PR draft from playbook I-V template.
7. Ask for explicit user confirmation.
8. Only after confirmation, create PR.

## PR Title Patterns

- `test(<pkg>): migrate tests to ginkgo/gomega`
- `test(<pkg>): improve unit coverage for <behavior>`
- `fix(test): stabilize <pkg> tests`
- `refactor(test/<pkg>): improve suite structure and readability`

## PR Mechanics (Concise)

Use title patterns from this file and body template from:
- `docs/plans/2026-03-02-fluid-testing-pr-playbook.md`

Mandatory PR mechanics:
- include testing + coverage evidence
- keep scope package-focused
- confirm no Testify in touched migrated files
- do not create/open PR before explicit user confirmation

## Common Gotchas

- unresolved merge conflicts before review
- PR too broad (split by package wave)
- syntax-only migration without scenario validation
- missing controller lane tests when controllers are touched
- opening PR before explicit confirmation
