## Agent skills

### Issue tracker

Issues live as GitHub Issues (via the `gh` CLI) on **`jasper0507/skills-manage`**. See `docs/agents/issue-tracker.md`.

### Triage labels

Canonical labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout — root `CONTEXT.md` + `docs/adr/` (ADRs created lazily). See `docs/agents/domain.md`.

### Current phase (for new sessions)

- **Done (v1 workbench backend + thin UI):** domain in `CONTEXT.md`; accepted throwaway UX in `prototypes/workbench-desktop/`; research under `docs/research/`; Spec **#1**; tickets **#2–#7** implemented (inventory → desk/index → boxes → clipboard → recycle/quarantine → `skills-manage serve` + embedded UI). Post-#7 fixes: draggable 回收站, one-screen row-major default layout.
- **Do next:** human soak-test; **frontend is open for design discussion only** (not re-implement domain). Any UI redesign → grill open questions → tickets if multi-session. Do **not** re-grill locked v1 workbench domain rules without updating `CONTEXT.md` first.
- **Do not implement:** `prototypes/tag-pick-flow/` (rejected). Treat `prototypes/` as behavior oracle, not production source tree. Do **not** re-implement closed #2–#7 unless a new bug ticket says so.
