# PROTOTYPE — workbench desktop (Coodesker-style)

**Throwaway.** Validates whether the **classification workbench** interaction matches intent.

> **Authority:** root `CONTEXT.md` wins if this prototype disagrees.  
> **v1 recycle:** icon-only soft trash (no body quarantine). Line 4 below is **historical prototype behavior**, not v1 product.

## Question

Does this feel right? (rev 2 after user feedback)

1. Box must **visibly list** skills (icon + list) after drop + flash what was just added  
2. Simple→simple compose; simple→composite add tab; **eject tab → simple box** (split allowed)  
3. Delete box → all placeholders **return to staging** (never vanish)  
4. ~~Copy + trash last icon = body quarantine~~ → **v1:** copy for multi-file; trash = icon-level only; last live placeholder cannot enter bin  
5. Visual polish deferred — function only

In-memory only. No real skill scan/delete.

## Rev 6

- Default layout: **recycle bin at row0/col0**; unfiled skills stack **below** in the same column (later product: row-major viewport fill — see `CONTEXT.md`).
- Desk/box interactions accepted as baseline; **delete semantics updated in CONTEXT / ADR-0001**.

## Run

```bash
# from repo root
python -m http.server 8765 --directory prototypes/workbench-desktop
```

Open http://127.0.0.1:8765/

## Files

| File | Role |
|------|------|
| `model.js` | Pure state / actions |
| `app.js` | DOM shell |
| `index.html` / `styles.css` | Throwaway UI |
