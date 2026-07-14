## Agent skills

### Issue tracker

Issues live as GitHub Issues (via the `gh` CLI) on **`jasper0507/skills-manage`**. See `docs/agents/issue-tracker.md`.

### Triage labels

Canonical labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout — root `CONTEXT.md` + `docs/adr/` (ADRs created lazily). See `docs/agents/domain.md`.

### Current phase (for new sessions)

- **Done:** domain in `CONTEXT.md`; accepted throwaway UX in `prototypes/workbench-desktop/`; research under `docs/research/`; **Spec #1**; tickets **#2** (inventory), **#3** (persistent desktop + 中央索引 + rescan), **#4** (desktop boxes: collision, 普通/组合盒子, compose/eject/demote), **#5** (clipboard + multi-select + create box), and **#6** (回收站 last-占位 隔离 / restore / purge); remaining **#7** with blocking edges.
- **Do next:** `/implement` on frontier ticket **#7** only (fresh context per ticket). Do **not** re-triage #7; do **not** re-grill v1 workbench rules.
- **Do not implement:** `prototypes/tag-pick-flow/` (rejected path). Treat `prototypes/` as behavior oracle, not production source tree.
