# PROTOTYPE — tag → skill → clipboard pick flow

> **REJECTED for v1.** Do not implement this flow. See `prototypes/README.md` and root `CONTEXT.md` (分类工作台 is the v1 path).

**Throwaway.** Answers one question, then die or move to a throwaway branch.

## Question

Does this interaction model feel right for *local skill inventory + human select*?

1. Maintain tags on already-known local skills (CLI-style actions in the same toy app).
2. Interactive pick: **choose a tag → choose a skill under that tag → copy invocation string to clipboard** (e.g. `/grill`).
3. User pastes into Codex/Claude/Grok themselves — **no host TUI binding**.

If while driving it you think “I would never do it this way” or “I need X before pick”, that is the answer.

## Verdict (2026-07-14)

**Does not match user intent.**

1. **Classification** should be a **Web UI**: create tags, **drag-and-drop** skills onto tags — not CLI tag commands.
2. **Selection** should be via an **entry skill** invoked inside Codex / Claude Code (interactive pick under a tag), or another in-session convenience — not a standalone terminal toy as the primary path.

Kept as a negative primary source for the discarded CLI+clipboard-first shape.

## Run

```bash
python prototypes/tag-pick-flow/app.py
```

From repo root. Python 3 stdlib only.

## Layout

| File | Role |
|------|------|
| `model.py` | Pure state + actions (portable) |
| `app.py` | Throwaway terminal shell |
| `README.md` | This file |

## Not in scope

Real filesystem scan, persistence, collections CRUD, vendor adapters, polish.
