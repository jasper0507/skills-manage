## Agent skills

### Issue tracker

Issues live as GitHub Issues (via the `gh` CLI) on **`jasper0507/skills-manage`**. See `docs/agents/issue-tracker.md`.

### Triage labels

Canonical labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout — root `CONTEXT.md` + `docs/adr/` (ADRs created lazily). See `docs/agents/domain.md`.

### Current phase (for new sessions)

- **Done:** domain in `CONTEXT.md`; accepted throwaway UX in `prototypes/workbench-desktop/`; research under `docs/research/`; **Spec #1**; tickets **#2**–**#7** (inventory → desk/index → boxes → clipboard → recycle/quarantine → **HTTP + embedded 分类工作台 UI** via `skills-manage serve`).
- **Do next:** v1 workbench is implementable end-to-end; close/triage remaining open GitHub tickets as needed. Do **not** re-grill locked v1 workbench rules without updating `CONTEXT.md` first.
- **Do not implement:** `prototypes/tag-pick-flow/` (rejected path). Treat `prototypes/` as behavior oracle, not production source tree.
