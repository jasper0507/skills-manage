# PROTOTYPE — workbench desktop (Coodesker-style)

**Throwaway.** Validates whether the **classification workbench** interaction matches intent.

## Question

Does this feel right? (rev 2 after user feedback)

1. Box must **visibly list** skills (icon + list) after drop + flash what was just added  
2. Simple→simple compose; simple→composite add tab; **eject tab → simple box** (split allowed)  
3. Delete box → all placeholders **return to staging** (never vanish)  
4. Copy in-place (staging or inside box); trash last icon = body quarantine, else icon-only  
5. Visual polish deferred — function only

In-memory only. No real skill scan/delete.

## Rev 6

- Default layout: **recycle bin at row0/col0**; unfiled skills stack **below** in the same column.
- Domain decisions locked into root `CONTEXT.md` (prototype accepted as behavior baseline).

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
