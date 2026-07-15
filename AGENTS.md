## Agent skills

### Issue tracker

Issues live as GitHub Issues (via the `gh` CLI) on **`jasper0507/skills-manage`**. See `docs/agents/issue-tracker.md`.

### Triage labels

Canonical labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout — root `CONTEXT.md` + `docs/adr/` (ADRs created lazily). See `docs/agents/domain.md`.

**Read first:** `CONTEXT.md` (product language) and `docs/adr/0001-v1-icon-only-recycle-bin.md` (v1 recycle = icon-only soft trash; **no** skill body quarantine/rm).

### Current phase (for new sessions)

- **Product consensus (grilled):** v1 = taxonomy workbench with **R2 icon-level recycle bin** (placeholders in/out of bin; empty drops placeholder records only). **Forbidden:** last live placeholder for an identity entering the bin; any skill-package isolate/rename/`rm`. Multi-filing = copy placeholders. Backend package **E2**: **E2.1–E2.2 done** (R2 + strip body-delete + Open legacy); remaining **E2.3** (rehome ItemIDs + document snapshot). **UI out of E2** (future redesign).
- **Code today:** Workbench recycle is R2 only; `internal/infra/quarantine` removed. Open strips legacy `RecycleBin` body rows and keeps `kind=recycle` placeholders as icon-bin. Do **not** reintroduce body-delete.
- **Done previously:** workbench/HTTP/thin UI skeleton; layout `cmd` → `internal/app` → `workbench` + `server`/`ui` + `infra/{scanner,index}` + `config/`; E2.1 R2 recycle; E2.2 Open legacy normalize.
- **Do next:** E2.3 rehome + document snapshot (#11). Frontend: design discussion only until a UI ticket; do not re-implement domain in the thin UI.
- **Do not implement:** `prototypes/tag-pick-flow/` (rejected). Treat `prototypes/workbench-desktop/` as behavior oracle for desk/box **except** where it implies body-delete — product authority is `CONTEXT.md`. Keep **Workbench** as the sole primary product seam.
