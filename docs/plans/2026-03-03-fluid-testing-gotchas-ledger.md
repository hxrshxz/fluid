# Fluid Testing Migration Gotchas Ledger

This is a living gotchas file for Fluid testing migration work.

Source baseline: merged PR patterns from `hxrshxz` and `adity1raut` in `fluid-cloudnative/fluid`.

## How to Use

- Read this before implementing or preparing a testing PR.
- Apply relevant prevention checks during coding and verification.
- If a new repeatable gotcha appears, append a new entry under `## New Gotchas`.
- Do not delete historical gotchas; append only.

## Stable Gotchas (From Merged PR History)

| ID | Symptom | Likely Cause | Fix Pattern | Prevention Check |
|---|---|---|---|---|
| G-01 | PR review stalls immediately | unresolved merge conflicts | rebase and resolve before final review ping | `git status` clean + no conflict markers |
| G-02 | Reviewer asks to split PR | scope too broad/multi-package | split into package/engine waves | keep one package wave unless user requests large PR |
| G-03 | Migration PR gets “not enough” feedback | syntax-only conversion | add behavior parity scenarios (success + error + edge) | explicit scenario checklist in PR body |
| G-04 | Async tests flaky | blind `time.Sleep` sync | replace with bounded `Eventually`/`Consistently` | grep for new `time.Sleep` in touched tests |
| G-05 | Works locally, flaky in CI | leaked env/global/patch state | `DeferCleanup`/After cleanup for env and patches | cleanup assertions in tests |
| G-06 | Migration incomplete feedback | Testify still present in touched scope | migrate assertions to Gomega | `rg 'testify'` on touched test files |
| G-07 | Reviewer asks for stronger proof | missing verification evidence | include exact test commands and results | required verification block in PR body |
| G-08 | Review churn on unrelated changes | opportunistic refactor/benchmark edits | keep migration-only diffs | no unrelated refactors/benchmarks |
| G-09 | Approval delayed after bot comments | substantive comments ignored | resolve requested bot/maintainer comments | checklist includes comment-resolution gate |
| G-10 | Style nits/re-request changes | formatting or local pattern mismatch | run fmt and match local naming/layout | `make fmt` + local style scan |

## High-Value Hotspots (Observed)

- `pkg/webhook/plugins`
- `pkg/application/inject/fuse`
- `pkg/ctrl/watch`
- `pkg/csi/plugins`
- `pkg/ddc/*`

Use tighter verification and behavior parity checks in these areas.

## New Gotchas

Append entries using this template:

```markdown
### YYYY-MM-DD | <package> | <short title>
- Symptom:
- Root cause:
- Fix applied:
- Prevention gate to add:
- Related PR/commit:
```
