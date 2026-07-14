# Safe recycle-bin / soft-delete / delayed permanent delete (local skill directories)

**Date:** 2026-07-14  
**Purpose:** Primary-source and well-known engineering practice for safely “recycling” **local directories** (especially developer tools that delete user packages/folders), so the skills-manager workbench can implement a 30-day recycle bin without silent data loss or accidental agent load of “deleted” skills.  
**Product authority:** root [`CONTEXT.md`](../../CONTEXT.md) — recycle bin confirms, 30-day retention, then permanent delete of the skill directory; deletion safety (scan-root allowlist, realpath identity, multi-placeholder).  
**User preference under discussion:** **Option B** — leave files at the original path, only mark pending-delete in the central index, then `rm` after 30 days. Question: is there a **safer design still close to B**?

**Method:** FreeDesktop Trash Specification and desktop OS trash behavior; CLI/API wrappers (`trash-cli`, `gio trash`, Electron `shell.trashItem`); package-manager multi-phase uninstall (apt/dpkg); database soft-delete / tombstone analogies; quarantine practice (AV). Web + official specs, 2026-07-14.

---

## Executive summary

| Finding | Detail |
|--------|--------|
| **Industry default is move-aside, not mark-in-place** | FreeDesktop Trash, macOS Trash, Windows Recycle Bin, `trash-cli`, `gio trash`, and Electron `shell.trashItem` all **move** (or copy-then-erase) content out of the original path. None treat “leave at original path + flag” as trashing. |
| **Pure Option B is unsafe for agent skills** | A skill that remains at its original realpath under a scan root stays **loadable, editable, and path-reusable** while the UI claims it is in the recycle bin. Index-only truth is easily bypassed by other tools, restarts, or agents that do not consult the workbench index. |
| **Safer B exists and is still “B-like UX”** | Keep a 30-day recycle-bin UI and restore path, but **immediately remove the skill from live scan roots** via **same-filesystem rename/move** into a sibling quarantine (app-private trash), with the index recording original path + purge-after. Same-FS rename is cheap (no tree copy). |
| **Recommended** | **Safer B = quarantine move-aside (same FS) + index metadata + 30-day purge job.** Prefer app-private trash over OS Trash for controllable restore/audit; optionally offer “send to OS Trash” as a permanent-delete shortcut. Do **not** ship pure in-place deferred `rm`. |

---

## 1. Why pure in-place deferred `rm` is dangerous

Option B (files stay at original path; only the central index has `pending_delete` / `purge_after`; a job later runs `rm -rf`) fails several safety properties that matter for **agent skill packages**.

### 1.1 Still loadable by agents

Skill discovery is almost always **path-based**: harnesses and scanners walk scan roots looking for `SKILL.md` (or equivalent). While the directory remains at the original realpath:

- Agents and CLIs that **do not read** the workbench index will still discover and invoke the skill.
- The workbench may hide the placeholder, but **disk truth ≠ product truth**.
- “Deleted” skills can still mutate model behavior — the worst failure mode for a recycle bin in this domain.

This is the skill-domain analogue of a database row with `deleted_at` that **every query forgets to filter**: soft-delete only works if **all readers** honor the tombstone. On a multi-tool machine, they will not.

### 1.2 Partial edits during the retention window

During 30 days at the original path:

- The user (or an agent) can **edit** files inside the “trashed” skill.
- Restore then resurrects a **mutated** tree, not the tree as of delete-confirm time (unless the tool snapshots content — which pure B does not).
- Concurrent tools may re-index or re-tag the path as a live skill.

Move-aside quarantine freezes *location* even if content is still readable under the trash path; better designs optionally **chmod** or write a sentinel so editors/scanners skip it.

### 1.3 Path reuse / name collision

While the original basename remains occupied:

- The user cannot reinstall or re-clone a skill of the same name into the same folder without fighting the pending corpse.
- Or worse: if a future “restore” expects that path, and the user has **already created a different skill** at that path after a bug or after manual delete of the tombstone-without-rm, restore can clobber live data.

OS trash and FreeDesktop address this by **vacating the original path immediately** and recording original path only in sidecar metadata.

### 1.4 Crash / power loss mid-`rm`

Deferred permanent delete eventually runs a recursive unlink. If the process dies mid-tree:

- Partial trees remain on disk (orphan, inconsistent package).
- Index may already say `purged` while files remain (or vice versa).

Mitigations exist (rename-to-unique-temp then delete; two-phase state machine: `purging` → `purged`; journal), but **crash mid-rm is worse when the path is still a “normal” skill path** that scanners treat as live. Quarantine paths are easier to treat as “owned by the manager; incomplete delete → retry purge only.”

### 1.5 Symlinks, realpath, and multi-link

From product model ([`CONTEXT.md`](../../CONTEXT.md)): skill identity is **realpath**; multiple placeholders may share one realpath.

Pure B hazards:

| Hazard | Why it bites |
|--------|----------------|
| **Symlink into skill** | Deleting “the skill” via a symlink path without resolving can delete the wrong thing or leave the real tree. |
| **Symlink *out* of scan root** | Following a link and `rm -rf` can destroy trees **outside** configured scan roots. |
| **Hard links** | Unlink decrements nlink; other hard links keep content visible under other names; “deleted” content still exists. |
| **Multiple placeholders, one realpath** | Index must tombstone **all** placeholders and run **one** filesystem lifecycle for the realpath. Pure B that keys only on placeholder id double-deletes or leaves orphans. |
| **Dangling index after manual `rm`** | User deletes the folder in a file manager; index still has `pending_delete` and the purge job errors or re-targets a path that was reused. |

FreeDesktop explicitly checks that `$topdir/.Trash` is **not a symbolic link** before use — a recognition that trash locations themselves must not be symlink attacks.

### 1.6 Summary: pure B’s core flaw

> **Soft-delete that only updates an index assumes exclusive control of the namespace.**  
> For local skill dirs under shared scan roots, the filesystem namespace is shared with agents, CLIs, editors, and the user. Leaving content at the live path is not a recycle bin; it is a **lie with a delayed `rm`.**

---

## 2. Safer patterns used in the wild

### 2.1 OS Trash (move vs mark)

#### FreeDesktop Trash Specification (primary source)

- **Spec:** [Trash Specification v1.0](https://specifications.freedesktop.org/trash/latest/) (2014-01-02; still current FreeDesktop trash standard as of this research date).
- **Definition:** *Trashing* = transfer into the Trash can; *Erasing* = unlink from the filesystem (possibly after trash).
- **Mandatory storage model:** each trash directory has:
  - `$trash/files` — **moved** payload (directories moved with entire contents).
  - `$trash/info` — sidecar `*.trashinfo` with original `Path=` and `DeletionDate=`.
- **Normative move:** *“When a file or directory is trashed, it MUST be moved into this directory [`$trash/files`].”*
- **Atomic metadata first:** create the `.trashinfo` with exclusive create (`O_EXCL` pattern) **before** relying on the move; unique names so repeated trash of the same original path does not overwrite previous copies.
- **Locations:** home trash at `$XDG_DATA_HOME/Trash` (typically `~/.local/share/Trash`); optional per-volume `$topdir/.Trash/$uid` or `$topdir/.Trash-$uid` so cross-device trash can avoid expensive **copy** when possible.
- **Copy fallback:** when home trash is on another filesystem, implementations may copy then erase original; permissions may change; user must be **warned** if trash fails — **MUST NOT** silently erase when the user expected trash.
- **Sticky bit / not-a-symlink checks** on `$topdir/.Trash` for multi-user safety.

**Takeaway:** FreeDesktop is explicitly **move (+ optional copy) + metadata**, never “mark in place.”

#### macOS Trash

- API: [`FileManager.trashItem(at:resultingItemURL:)`](https://developer.apple.com/documentation/foundation/filemanager/trashitem(at:resultingitemurl:)) — *“Moves an item to the trash.”*
- Volume-local trash (e.g. `.Trashes/<uid>` on external volumes) so move stays on-volume when possible — same motivation as FreeDesktop topdir trash (avoid full-tree copy).

#### Windows Recycle Bin

- Desktop “Delete” moves items into per-volume `$Recycle.Bin` structures with metadata for restore; Shift+Delete bypasses to permanent delete.
- Programmatic safe path is shell/`IFileOperation` recycle semantics, not raw `DeleteFile` on the live path with a later timer.

**Cross-OS summary:** all major desktop trash systems **vacate the original path immediately** and store restore metadata beside the payload.

### 2.2 Quarantine / move-aside directories

Antivirus and mail clients use **quarantine**: move (often encrypt/rename) the object out of its active path so it cannot be executed or casually opened, while retaining recoverability for false positives.

- Microsoft Defender and peers: quarantine is isolation + restore UI, then optional permanent delete.
- Engineering parallel for skills: **pending-delete skills must not remain on the agent load path**, same as malware must not remain on the execute path.

### 2.3 Tombstone + hide from scanners (files stay)

Database soft-delete patterns (analogies only):

| Pattern | Idea | File-system analogue |
|---------|------|----------------------|
| `deleted_at` on row | Readers filter `WHERE deleted_at IS NULL` | Index flag; **every** scanner/agent must filter — unrealistic for multi-harness skills |
| Tombstone table | Row removed from live table; id + payload in `deleted_*` | Move dir out of live tree; record original path in index |
| Separate archive table | Brandur-style `deleted_record` dump | App trash dir + JSON/SQLite metadata |
| Event sourcing critique | Soft-delete is “half of event sourcing” | Prefer explicit lifecycle states: `active → quarantined → purged` |

**Lesson from DB practice:** soft-delete works only with **enforced read scopes** (ORM default scope, DB views, RLS). On a raw filesystem with third-party readers, the only reliable “scope” is **not being under the scan root with a discoverable name**.

**Hide-in-place variants (weaker than move):**

- Rename to `.pending-delete-<id>-<basename>` so naive scanners that skip dotfiles miss it (many skill scanners **do not** skip all dot patterns, and some list all dirs — **unreliable**).
- Write `DISABLED` / remove or rename `SKILL.md` → `SKILL.md.trashed` so package is invalid as a skill while tree stays (closer to B; still path-occupying; partial restore complexity).
- Add path to every harness’s ignore list (Gemini enable/disable, etc.) — **incomplete** coverage across 15+ tools; not a substitute for path vacation for true delete.

### 2.4 Two-phase delete in package managers

Not soft-delete of arbitrary trees, but useful lifecycle thinking:

| System | Phases | Relevance |
|--------|--------|-----------|
| **dpkg/apt** | `remove` (files gone, config may remain, status `rc`) vs `purge` (config too) | Two-step intentional uninstall; residual state is **explicit** in the package DB, not “files still look installed.” |
| **npm** | `npm uninstall` removes from `node_modules` + updates manifests; cache clean is separate | Live install path is cleared; cache is a **separate** tree, not the live module path. |
| **cargo** | `cargo clean` / cache GC targets build artifacts and caches, not “mark crate deleted but leave in `src`” | Again: remove from the active location. |

None of these leave a package **fully present and loadable** for 30 days while claiming removal. Residual config (`rc` packages) is intentional **non-code** leftover, not a full deferred delete of the payload.

### 2.5 `trash-cli`, `gio trash`, Electron `shell.trashItem`

| Tool | Behavior | Source |
|------|----------|--------|
| **trash-cli** | FreeDesktop-compliant CLI: `trash-put`, `trash-list`, `trash-restore`, `trash-empty`; records original path, deletion date; same trash as KDE/GNOME/XFCE | [github.com/andreafrancia/trash-cli](https://github.com/andreafrancia/trash-cli) |
| **gio trash** | GLib/GIO: move to trashcan per FreeDesktop; `gio trash PATH`, list/empty options; volume-aware | [Gio.File.trash](https://docs.gtk.org/gio/vfunc.File.trash.html); [ArchWiki Trash management](https://wiki.archlinux.org/title/Trash_management) |
| **Electron `shell.trashItem(path)`** | Moves to OS-specific trash (macOS Trash, Windows Recycle Bin, DE trash on Linux); promise resolves/rejects | [Electron shell API](https://www.electronjs.org/docs/latest/api/shell#shelltrashitempath) |

**Engineering note:** These are excellent for **user-facing permanent dispose** (“I want OS recoverability”), but for a **product-owned 30-day recycle bin with custom restore UI and audit log**, an **app-private quarantine** is usually preferable:

- Predictable layout and metadata (skill id, realpath, purge_after, multi-placeholder).
- Restore without depending on FreeDesktop/OS trash UI or third-party empty.
- Avoid mixing skills trash with unrelated desktop trash items.
- Still use same-FS rename for performance (mirror FreeDesktop’s topdir trash motivation).

### 2.6 Immutable backup / snapshot before delete

Stronger recoverability (orthogonal to recycle UX):

- **FS snapshots:** ZFS/btrfs snapshot, APFS snapshot, LVM — cheap COW restore of whole trees.
- **User backup:** Time Machine, restic, etc.
- **App-level:** tarball or content-addressed copy into backup store **before** purge (expensive for large trees; usually overkill for skill dirs, which are small).

For skills manager v1, snapshots are optional hardening for **purge**, not a substitute for vacating the live path on trash-confirm.

---

## 3. Hybrid designs that keep “B-like UX”

**B-like UX (product):**

1. User drags skill placeholder(s) to recycle bin → **confirm** (show path + name).
2. Placeholder appears **in recycle bin** for up to **30 days**; can **restore**.
3. After 30 days or Empty → **permanent delete** of the skill directory.
4. Index remains source of truth for workbench layout; realpath identity; all placeholders for that realpath enter the same lifecycle.

Below: designs that preserve that UX while improving safety over pure in-place mark.

### 3.1 Mark + move to sibling app trash (recommended “safer B”)

**On confirm:**

1. Resolve **realpath**; verify under scan root allowlist; refuse symlinks-to-outside, non-skill dirs, system/bundled.
2. Atomically (same FS): `rename(original, trash_root / entry_id)` where e.g.  
   `trash_root = <scan-root>/.skills-manage-trash/` **or**  
   `~/.local/share/skills-manage/trash/files/` (if same device; else copy+verify+remove with progress/warning).
3. Write index record: `{ realpath_original, trash_path, deleted_at, purge_after, skill_id, placeholders[] }`.
4. Remove/hide all placeholders from staging/boxes; show one recycle-bin card.
5. Ensure scanners **never** treat `trash_root` as a skill root (hardcoded ignore + config).

**Restore:** rename back to `realpath_original` if free; if occupied, fail with conflict UI or restore-as-new-name.

**Purge:** `rm -rf trash_path` only (never original path string without re-check); index → `purged` + audit.

**Why this is still “B-like”:** User never thinks about FreeDesktop; 30-day bin is product-owned; no full-tree copy on same FS (rename is O(1) metadata). Closest safe cousin of “don’t re-home skills for classification” — quarantine is only for delete lifecycle.

### 3.2 Mark + rename in place with distinctive prefix

**On confirm:** `rename(skillDir, sibling ".pending-delete-<uuid>-" + basename)` under same parent.

- Vacates the **original basename** (good for reinstall).
- Often still under the scan root — **must** exclude `.pending-delete-*` (and preferably all dot-dirs) in every scan path.
- Slightly weaker isolation than a dedicated trash directory (easier to stumble upon in file managers; uglier parent dirs).

Still far safer than pure B because naive loaders looking for the old name miss it; scanners with ignore rules miss it.

### 3.3 Mark + disable via tool config without moving

- Flip “disabled” in Gemini/other managers; add to ignore lists.
- **Pros:** No filesystem churn.  
- **Cons:** Incomplete multi-tool coverage; path still occupied; agents that scan raw FS still load; **not sufficient** for “true delete after 30 days” product promise.

Use only as **extra** belt-and-suspenders after move-aside, not as the primary mechanism.

### 3.4 Mark + hide via invalidating the package entrypoint

- Rename `SKILL.md` → `SKILL.md.trashed` or add frontmatter `disabled: true` if all tools honor it (they do not universally).
- Tree stays; restore is easy; **path reuse and accidental open of remaining files** remain.
- Acceptable only for soft-disable feature, **not** for recycle-bin-with-eventual-rm.

### 3.5 OS Trash immediately on confirm

- Call `trash-put` / `gio trash` / `shell.trashItem` on confirm; product recycle bin becomes a **mirror of metadata** only, or skips custom 30-day and relies on OS retention.
- **Pros:** User-familiar; free cross-app restore.  
- **Cons:** Harder multi-placeholder/audit model; OS may empty trash on its own schedule; restore path may not re-link workbench placeholders cleanly; Linux without DE trash support varies.

**Hybrid:** custom quarantine for 30-day product bin; optional “Empty now → OS Trash or permanent” at purge time.

### 3.6 Copy-on-quarantine (usually wrong for large trees)

FreeDesktop notes copy is costly across devices. For skill dirs (typically small text trees), copy is often fine, but:

| Approach | Cost | Risk |
|----------|------|------|
| Same-FS **rename** | ~O(1) | Best default |
| Cross-FS **copy + verify + remove original** | O(size) | Need fsync/verify; don’t remove original until copy OK; disk double-use during retention if copy kept |
| **Copy and leave original** | Double disk | **Wrong** — defeats delete; agents still load original |

Never “copy to trash and leave original as the soft-delete.” That is backup, not trash.

### 3.7 Snapshot-then-mark (advanced)

Take FS snapshot or tarball, then either move-aside or (if pure B) mark. Snapshot helps recovery after purge mistakes; still **does not** fix agent load if original path remains live.

---

## 4. Safety checklist (any scheme)

Apply regardless of move vs mark. Aligns with [`CONTEXT.md`](../../CONTEXT.md) deletion safety.

### 4.1 UX / policy

- [ ] **Explicit confirm** showing skill name(s), **absolute realpath(s)**, retention (“30 days”), and that permanent delete removes the directory from disk.
- [ ] Distinct confirm for **Empty recycle bin** / **Delete now** vs drag-to-bin.
- [ ] No silent permanent delete; no delete of non-confirmed paths.
- [ ] Multi-select batch: list all paths; one confirm for the set.

### 4.2 Path allowlist / identity

- [ ] Only delete skills **discovered by scan** and whose **realpath is under a configured scan root**.
- [ ] Refuse **bundled/system/read-only** roots.
- [ ] Refuse paths outside allowlist after `realpath` / `canonicalize`.
- [ ] Refuse **symlink-to-outside** (if path is symlink, resolve; if any component escapes policy, abort).
- [ ] Refuse non-skill directories (no valid `SKILL.md` / package marker at delete time — or require prior inventory record).
- [ ] Skill identity = **realpath**; all placeholders sharing that realpath enter the same trash lifecycle **once**.

### 4.3 Atomic ops & state machine

- [ ] Lifecycle states: e.g. `active` → `quarantined` (or `pending_delete`) → `purging` → `purged` | `restored`.
- [ ] Index update and filesystem op ordered so crash recovery can reconcile (prefer: reserve trash entry id → move → commit index; or write intent log).
- [ ] FreeDesktop-style: unique trash entry names; never overwrite prior trash of same basename.
- [ ] Purge job: only `rm` the **recorded trash_path**, after re-validating it still lives under trash root; set `purging` first; retry incomplete trees.
- [ ] **Dry-run** mode for purge and for delete-confirm preview.

### 4.4 Symlinks, mounts, multi-link

- [ ] Do not follow unexpected symlinks when deleting trees (`rm`/`fs.rm` with careful options; prefer delete of rename target that was itself validated).
- [ ] Same-filesystem rename preferred; detect EXDEV and choose copy-or-fail policy explicitly.
- [ ] Document hard-link residual risk (rare for skill trees).

### 4.5 Restore

- [ ] Restore only if original path free **or** user chooses alternate path.
- [ ] Re-create placeholders in staging (product rule); do not invent box membership unless stored in trash metadata.
- [ ] Refuse restore of corrupted trash entries (missing payload).

### 4.6 Clock / retention jobs

- [ ] Store `deleted_at` and `purge_after` as UTC timestamps.
- [ ] Do not trust client-only clocks for security boundaries; for local single-user tool, local clock is OK but handle **clock skew** (purge_after in the past on startup → purge; far-future → clamp).
- [ ] Run retention on workbench start + optional periodic timer; no requirement for always-on daemon in v1 if startup sweep is reliable.
- [ ] User-visible: days remaining; manual “Delete now.”

### 4.7 Audit log

- [ ] Append-only or durable log: who/when (local user), action (`trash`, `restore`, `purge`, `purge_failed`), realpath, trash_path, entry id.
- [ ] Failures must be auditable (partial purge, path missing, allowlist reject).

### 4.8 Scanner / agent isolation

- [ ] Trash / quarantine directories **excluded** from all scan roots by default.
- [ ] Dot-directory and/or fixed name (`.skills-manage-trash`) documented.
- [ ] Never register trash as a scan root in default config.

### 4.9 Concurrency

- [ ] Lock or transactional update so two UI sessions cannot double-trash or restore vs purge race.
- [ ] Multi-placeholder: single flight per realpath.

---

## 5. Concrete recommendation for this project

### 5.1 Ranked options (safest → closest-to-pure-B)

| Rank | Option | Safety | UX fit | Cost | Notes |
|:----:|--------|:------:|:------:|:----:|-------|
| **1** | **App-private quarantine (same-FS rename) + index + 30-day purge** | ★★★★★ | ★★★★★ B-like | Low | **Recommended “safer B”** |
| **2** | **OS Trash on confirm** (`trash-cli` / `gio` / `shell.trashItem`) | ★★★★ | ★★★ | Low | Less control over 30-day policy & restore into workbench |
| **3** | **In-parent rename** `.pending-delete-<id>-*` + scan ignore + index | ★★★★ | ★★★★ | Lowest | Messier parents; ignore-list discipline required |
| **4** | **Invalidate entrypoint** (`SKILL.md` rename) + index + later `rm` of original tree | ★★★ | ★★★ | Low | Path stays; weaker isolation |
| **5** | **Index mark only + hide in UI + later `rm` (pure B)** | ★ | ★★★★★ | Lowest | **Not recommended** — agents still load |
| **6** | **Immediate `rm` after confirm (no retention)** | ★★ (with allowlist) | ★★ | Low | Fails 30-day product requirement |
| **7** | **Full tree copy to trash leaving original** | ★ | ★ | High | Not trash; doubles disk; still loadable |

### 5.2 Recommended “safer B”

**Name:** *Quarantine move-aside with product-owned recycle bin*  
**Still “B” in spirit:** classification remains index-first; skills are not re-homed for boxing/tags; only the **delete lifecycle** moves bytes, and only via **cheap rename** when possible.

**Algorithm (normative sketch):**

```
on_user_confirm_trash(placeholder_ids):
  realpaths = unique realpaths of placeholders
  for each realpath:
    assert is_inventory_skill(realpath)
    assert realpath under scan_roots (canonical)
    assert not symlink_escape(realpath)
    entry_id = new_unique_id()
    trash_path = trash_root / entry_id
    # prefer rename; on EXDEV: copytree → verify → remove original (or refuse with message)
    rename(realpath, trash_path)
    index.insert_trash_entry({
      entry_id, original_path: realpath, trash_path,
      deleted_at: now_utc(), purge_after: now_utc()+30d,
      placeholder_ids, skill_meta_snapshot...
    })
    mark all placeholders for realpath as in recycle bin

on_restore(entry_id):
  e = index.get(entry_id)
  assert e.trash_path under trash_root
  if exists(e.original_path): conflict UI
  else rename(e.trash_path, e.original_path); restore placeholders to staging

on_purge(entry_id) or retention_job:
  e = index.get(entry_id)
  assert now >= e.purge_after or user_forced
  e.state = purging
  assert e.trash_path under trash_root  # never delete original_path string blindly
  rm_rf(e.trash_path)
  e.state = purged; audit log
```

**Trash root placement:**

1. **Preferred for simplicity:** one user-level store  
   `~/.local/share/skills-manage/trash/{files,info}`  
   FreeDesktop-inspired layout; if skill was on another device, either copy with warning or per-scan-root trash:
2. **Preferred for zero-copy:** per scan root  
   `<scan-root>/.skills-manage-trash/`  
   always same FS as the skill; must be excluded from scans.

**Do not implement pure Option B** for permanent-delete candidates. If a future “soft disable without delete” feature is needed, that is a **different** product action (disable), not recycle bin.

### 5.3 Optional enhancements (later)

- At purge time, offer **OS Trash** instead of hard `rm` for a second recovery layer.
- Optional tarball snapshot in `trash/info` for tiny skills if rename target might be scrubbed by the user.
- Startup reconciliation: index says quarantined but path missing → mark purged/missing; trash dir entry without index → quarantine recovery UI.
- `directorysizes`-style cache only if trash UI needs aggregate disk usage.

### 5.4 Mapping back to CONTEXT.md

| CONTEXT requirement | How safer B satisfies it |
|---------------------|---------------------------|
| Confirm + show path | Unchanged; show original realpath + “will move to quarantine” |
| 30-day restore | Restore = rename back + placeholders to staging |
| True delete of skill dir | Purge deletes **trash_path** only after retention |
| Scan-root allowlist | Enforced before rename; trash roots never scanned |
| realpath identity / multi-placeholder | One trash entry per realpath; all placeholders bound to it |
| Isolation choice left open | **Decision: quarantine move-aside**, not pure deferred in-place `rm` |

---

## 6. Sources (URLs)

Primary / official:

- FreeDesktop.org Trash Specification v1.0 — https://specifications.freedesktop.org/trash/latest/
- Apple `FileManager.trashItem` — https://developer.apple.com/documentation/foundation/filemanager/trashitem(at:resultingitemurl:)
- Electron `shell.trashItem` — https://www.electronjs.org/docs/latest/api/shell#shelltrashitempath
- GIO `GFile.trash` — https://docs.gtk.org/gio/vfunc.File.trash.html

Implementations & practice:

- trash-cli — https://github.com/andreafrancia/trash-cli
- ArchWiki: Trash management — https://wiki.archlinux.org/title/Trash_management
- npm uninstall docs — https://docs.npmjs.com/uninstalling-packages-and-dependencies/
- apt/dpkg residual config (`rc`) / purge discussion — e.g. https://kitson-consulting.co.uk/blog/apt-dpkg-purge-rc-packages
- Soft-delete / tombstone discussion (DB analogy) — https://brandur.org/fragments/deleted-record-insert ; https://dba.stackexchange.com/questions/14402/tombstone-table-vs-deleted-flag-in-database-syncronization-soft-delete-scenari
- Microsoft Defender restore quarantined files — https://learn.microsoft.com/en-us/defender-endpoint/restore-quarantined-files-microsoft-defender-antivirus

Product context (this repo):

- [`CONTEXT.md`](../../CONTEXT.md) — Recycle bin, deletion safety, realpath identity, scan roots

---

## 7. One-line answer to the user question

**Is there a safer design still close to B?**  
**Yes:** keep the 30-day recycle-bin UX and index-driven restore, but on confirm **immediately rename/move the skill out of live scan paths into an app quarantine** (same filesystem), then `rm` only that quarantine path after 30 days — not pure in-place deferred `rm`.
