## Agent skills

### Issue tracker

Issues live as GitHub Issues (via the `gh` CLI) on **`jasper0507/skills-manage`**. See `docs/agents/issue-tracker.md`.

### Triage labels

Canonical labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout — root `CONTEXT.md` + `docs/adr/` (ADRs created lazily). See `docs/agents/domain.md`.

**Read first:** `CONTEXT.md` (product language) and `docs/adr/0001-v1-icon-only-recycle-bin.md` (v1 recycle = icon-only soft trash; **no** skill body quarantine/rm).

### Current phase (for new sessions)

- **Product consensus (grilled):** v1 = taxonomy workbench with **R2 icon-level recycle bin** (placeholders in/out of bin; empty drops placeholder records only). **Forbidden:** last live placeholder for an identity entering the bin; any skill-package isolate/rename/`rm`. Multi-filing = copy placeholders. Backend package **E2 + E3 done** (R2, membership truth, placement projection). **UI out of E2** (future redesign).
- **Code today:** Workbench recycle is R2 only; box membership truth = `ItemIDs`; durable placement = desktop | recycle | empty (member-only). `LocBox` is Desk projection only. Write path uses placement primitives + `admitMember`; **rehome only on Open** (load repair). Mutations: document snapshot rollback + single persist. Do **not** reintroduce body-delete.
- **Done previously:** workbench/HTTP/thin UI skeleton; E2.1–E2.3; E3.1–E3.2 membership/placement; structural cleanup (placement primitives, unified desktop place, server mutateJSON).
- **Do next:** UI redesign tickets when cut. Do not re-implement domain in the thin UI.
- **Do not implement:** `prototypes/tag-pick-flow/` (rejected). Treat `prototypes/workbench-desktop/` as behavior oracle for desk/box **except** where it implies body-delete — product authority is `CONTEXT.md`. Keep **Workbench** as the sole primary product seam.
