# Similar Local Skill Inventory Projects

**Date:** 2026-07-14  
**Purpose:** Primary-source research for a greenfield **local inventory manager** of already-installed agent skills (`SKILL.md` packages)—list / tag / collection / search / select—so the author can borrow design ideas or fork/extend rather than start from zero.  
**Product authority:** root [`CONTEXT.md`](../../CONTEXT.md).  
**Out of scope for re-research:** marketplace/registry landscape, path maps, and install-tool dominance already covered in [`skills-management-landscape.md`](./skills-management-landscape.md).  

**Method:** Web search + primary GitHub READMEs/docs (and arXiv for Skilldex). Star counts via GitHub API on 2026-07-14.

---

## Executive summary

### Can we stand on an existing OSS base?

**Short answer: mostly no for a clean fork; yes for product patterns.**

There is **no mature open-source project whose v1 is “scan multi-root local skills → user-level central index of tags + named scenario collections → human select with invocation hints,” without install/marketplace/sync as the main product.**

What *does* exist:

| Cluster | What it is | Match to CONTEXT |
|--------|------------|------------------|
| **Desktop “control centers”** | Unify skills across agents, often with marketplace + symlink fan-out | Strong multi-tool + sometimes tags/presets; weak “index-only / no re-home” |
| **Install CLIs** | `npx skills`, `osk`, `gh skill`, Gemini `skills` | List/remove/enable; **not** user taxonomy |
| **Inventory-first experiments** | ASAM, chemny `agent-skill-manager`, VersoXBT audit plugin | Closest *intent* (scan/list/search); immature or single-tool |
| **Delivery/ops toolkits** | SkillPort (MCP/CLI serve), Skilldex (registry + skillsets) | Tags/skillsets interesting; wrong primary job |
| **Pattern donors (adjacent)** | `pet` (snippet tags + select), `buku` (bookmark tags), chezmoi/stow (symlink farms) | UX/model transfer only |

**Closest product analogue overall:** [xingkongliang/skills-manager](https://github.com/xingkongliang/skills-manager) (~3k★, MIT, Tauri/Rust + TS CLI)—**tags**, **presets ≈ collections**, multi-agent (incl. **Grok**), central library, **adopt** existing agent dirs, enable/disable via presets. Gaps vs CONTEXT: install/marketplace-first, skills are **copied into** `~/.skills-manager` rather than index-only over realpath identity; desktop-heavy.

**Closest “inventory language” analogue:** [mode-io/skill-manager](https://github.com/mode-io/skill-manager) (~107★, MIT)—explicitly **“adopt local Skills into one shared inventory, enable/disable per harness.”** Still re-homes packages to a shared store and links out; also MCP + slash commands + marketplace.

**Closest pure scan/index CLIs (immature):** [cikorsky/agent-skill-asset-manager](https://github.com/cikorsky/agent-skill-asset-manager) (0★, ASAM) and [chemny/agent-skill-manager](https://github.com/chemny/agent-skill-manager) (1★)—local registry/SQLite or JSON index, search, multi-root scan; little/no user tags+collections product maturity.

**Recommendation:** Treat v1 as **greenfield**, but **steal hard** from skills-manager (presets/tags/adopt UX), mode-io (adopt + enable matrix), ASAM/chemny (scan→index→search), and `pet` (human select + tags). Forking a full desktop control center to strip it down to inventory-only is usually **not** saner than building a small CLI with a user-level index.

---

## Comparison table

Columns map to CONTEXT decisions. ★ as of 2026-07-14 (GitHub API).

| Project | ★ | License | Lang | Inventory of installed | Tags | Collections / presets | Multi-tool scan | Central index (not in skill) | Multi-root / realpath | Primary job |
|---------|---|:-------:|------|:----------------------:|:----:|:---------------------:|:---------------:|:----------------------------:|:---------------------:|-------------|
| **xingkongliang/skills-manager** | ~3.0k | MIT | Rust+TS | ✅ (library + agent views) | ✅ | ✅ **Presets** | ✅ 15+ incl. Grok | ✅ SQLite (+ skill files in library) | Partial (linked workspaces; library is SoT) | Desktop library + sync + marketplace |
| **mode-io/skill-manager** | ~107 | MIT | TS+Py | ✅ **Adopt → shared inventory** | ❌ (not primary) | ❌ | ✅ Codex/Claude/Cursor/OpenCode/… | ✅ app store + links | Shared store + links, not pure realpath | Local control center (skills/MCP/commands) |
| **chemny/agent-skill-manager** | 1 | MIT | Python | ✅ scan + dashboard | ❌ | ❌ | ✅ multi platform roots | ✅ `~/.agent-skill-manager` SQLite | Multi-root scan; group by **name** | Local inventory / cleanup |
| **cikorsky/ASAM** | 0 | MIT | Python | ✅ scan→router JSON | tags in frontmatter (search weight) | ❌ | Config multi-dir | ✅ file-based router | Config scan_dirs; dedupe by **name** | Searchable asset index |
| **VersoXBT/skill-manager** | 6 | MIT | JS | ✅ Claude inventory | ❌ | ❌ (report categories) | Claude only | config for ignore | Claude paths | Claude plugin audit |
| **vercel-labs/skills** | ~26k | MIT | TS | ✅ `skills list` (lockfile) | ❌ | ❌ | ✅ 70+ agents (no Grok id) | lockfile (install provenance) | Install path map | Install from git / skills.sh |
| **gotalab/skillport** | ~403 | MIT | Python | ✅ list under own dir | ✅ in **frontmatter** | category filter | Serve via MCP/CLI | own skills dir | Single managed tree | Validate + serve skills |
| **umutbozdag/agent-skills-manager** | 24 | MIT | TS | ✅ multi-source dashboard | auto-categories | ❌ | ✅ skills+rules mix | browser app local FS | Multi path | Web dashboard inventory |
| **beautyfree/skiller** | 45 | MIT | TS | ✅ dashboard | ❌ | ❌ | ✅ 44 agents | app data | Install/sync focus | Electron install/sync |
| **vudknguyen/openskill** | 9 | MIT | TS | ✅ `osk ls` | ❌ | ❌ | ✅ multi | `~/.openskill` config | Installer | Installer CLI |
| **lasoons/AgentSkillsManager** | 98 | MIT | TS | partial (active IDE dir) | ❌ | preset **repos** (not user collections) | multi-IDE extension | — | project install | VS Code install from cloud |
| **wanghuan9/SkillDock** | ~275 | **closed** | — | ✅ source-grouped list | ❌ | source grouping | ✅ many tools | managed library | sync via links | Desktop (not forkable) |
| **Pandemonium Skilldex** | 2 | MIT | TS | list installed scopes | registry tags | ✅ **skillsets** | own scope tree | skilldex manifests | hierarchical scopes | PM + registry + skillsets |
| **Gemini CLI skills** | — | vendor | — | ✅ list/enable/disable | ❌ | ❌ | Gemini roots only | disable state | user/workspace | Vendor manager |
| **`gh skill`** | — | vendor | — | install/list (Copilot-oriented) | ❌ | ❌ | GitHub skills | — | install | Discover/install |
| **pet** (snippet mgr) | ~5.3k | MIT | Go | N/A (snippets) | ✅ | ❌ | N/A | TOML store | multi snippet dirs | **Pattern donor** |
| **buku** | ~7.2k | GPL-3 | Python | N/A (bookmarks) | ✅ | ❌ | N/A | SQLite | — | **Pattern donor** |
| **chezmoi** | ~20k | MIT | Go | N/A (dotfiles) | N/A | N/A | multi machine | source dir | template + apply | **Pattern donor** (paths) |

---

## Per-project sections (top candidates first)

### 1. xingkongliang/skills-manager — strongest product analogue

| | |
|--|--|
| **URL** | https://github.com/xingkongliang/skills-manager |
| **License** | MIT |
| **Language** | Rust (Tauri backend) + TypeScript/React UI; shared CLI |
| **Activity** | ~3.0k★, ~259 forks, releases through v1.28.x (Jul 2026), active |
| **Problem** | One app to install, organize, and sync AI agent skills across 15+ coding tools |

**Overlap with CONTEXT**

- **Inventory:** Central **Library** plus Global/Project/Linked workspaces that list skills agents actually see (including ones installed outside the app).
- **Tags:** First-class tag add/list/filter (incl. **Untagged** filter). CLI: `skills tag add|list`.
- **Collections:** **Presets** = named groups of skills; activate/deactivate in a workspace (one-shot apply, not live sync). Closest OSS match to CONTEXT **集合**.
- **Multi-tool:** Cursor, Claude Code, Codex, **Grok**, OpenCode, Gemini CLI, Copilot, Windsurf, etc.; custom tools with custom paths.
- **Central index:** SQLite metadata + git-backed skill library under `~/.skills-manager` (configurable). Tags/presets backed up; secrets stay local.
- **Adopt:** `skills adopt <agent-skills-dir>` for skills already on disk.
- **Enable/disable:** Via preset membership + per-agent sync badges; batch enable/disable.
- **Search:** Library search + marketplace search.

**Gaps vs CONTEXT**

- **Primary job is install + multi-tool sync + marketplace**, not “remember how to invoke already-installed skills.”
- Skills live in a **managed central library** (copy/import), not pure **realpath identity of existing trees** with index-only metadata.
- Presets are **activation-for-agents** (sync out), not primarily **human scenario bundles for selection UX**.
- Heavy desktop stack; CLI exists but product gravity is GUI.
- Identity/dedup is library-oriented, not CONTEXT’s “symlinks merge via realpath.”

**Fork/extend viability:** **Ideas-first (high value); full fork only if you want a Tauri product.** Extracting presets/tags/adopt/CLI surface as a library would be a large surgical strip of marketplace/sync/backup. Prefer **design borrow** of Preset model + tag UX + adopt flow.

---

### 2. mode-io/skill-manager — “shared inventory” + adopt/enable matrix

| | |
|--|--|
| **URL** | https://github.com/mode-io/skill-manager |
| **License** | MIT |
| **Language** | TypeScript frontend + Python backend |
| **Activity** | ~107★, npm `@mode-io/skill-manager`, Homebrew, releases ~v0.3.1 (May 2026) |
| **Problem** | Local-first control center for Skills, MCP servers, slash commands across harnesses |

**Overlap with CONTEXT**

- Explicit product language: **“Adopt local Skills into one shared inventory, then enable or disable them per harness.”**
- Unified view of **in use / needs review / discover**.
- After adopt: **one canonical package** in app storage, exposed via **local links**; disable removes harness binding without deleting package.
- Multi-harness: Codex (default skill root `~/.agents/skills`), Claude (`~/.claude/skills`), Cursor, OpenCode, Hermes, OpenClaw (skills partial).
- App-owned store (macOS Application Support / XDG)—classification not written into skill packages for managed items.
- Slash-command library as parallel “shared prompt library” pattern (sync state + content hashes).

**Gaps vs CONTEXT**

- Still **re-homes** skills into shared store (not scan-and-index-in-place).
- Scope includes MCP + marketplace + LLM security **scan**—broader than v1 inventory.
- No documented user **tags** or **named scenario collections** (presets).
- Grok not in supported harness table.
- Desktop/local server UX, not a lean “human select” CLI.

**Fork/extend viability:** **Strong ideas (adopt semantics, enable matrix, ownership of bindings).** Forking the full app for inventory-only is heavy (Python+TS monorepo). Steal the **state model**: inventory record + per-harness bindings + “needs review.”

---

### 3. chemny/agent-skill-manager — inventory/cleanup CLI + local dashboard

| | |
|--|--|
| **URL** | https://github.com/chemny/agent-skill-manager |
| **License** | MIT |
| **Language** | Python (stdlib-heavy), ships as installable skill + `bin/asm` |
| **Activity** | 1★, v1.1.0 (Jun 2026)—early but **intent-aligned** |
| **Problem** | “Which skills are useful, duplicated, built-in, unused?” Local admin for growing skill folders |

**Overlap with CONTEXT**

- **Scan multi-root** defaults: `~/.agents/skills`, Codex, Claude (commands/agents), OpenClaw, Hermes; custom sources.
- **Local registry** in `~/.agent-skill-manager/` (SQLite)—not rewriting skill packages.
- List / search / show / usage (session logs when available) / health / report / web dashboard.
- Soft **enable/disable** as registry status (conservative; not full multi-agent rewrite).
- Duplicate detection; group copies by name; update/unify candidates.

**Gaps vs CONTEXT**

- **No tags or collections.**
- Identity by **name grouping**, not realpath merge of symlink farms.
- Claude paths lean **commands/agents** more than pure `skills/` multi-tool matrix (Codex/Grok/Claude skills roots incomplete vs CONTEXT scan roots).
- Install still present; maturity low.

**Fork/extend viability:** **Possible thin base for scan+SQLite index**, but expect rewrite of path matrix, identity, and taxonomy. Better as **proof that inventory-only tools are emerging** than production fork.

---

### 4. cikorsky/agent-skill-asset-manager (ASAM) — scan → index → weighted search

| | |
|--|--|
| **URL** | https://github.com/cikorsky/agent-skill-asset-manager |
| **License** | MIT |
| **Language** | Python CLI (`asam-scan|sync|search|update`) |
| **Activity** | 0★, early (Jun 2026) |
| **Problem** | Find which skill does X across large local skill libraries |

**Overlap with CONTEXT**

- **Inventory-first:** scan configured dirs for `SKILL.md`, build file-based **router JSON** / optional Excel—**no DB server**.
- Multi `scan_dirs` with types (skill-cc, agent-sub, …).
- Weighted search (name, aliases, triggers, **tags**, description) + synonym expansion.
- Optional GitHub update check.
- User-level workflow: configure roots → scan → search—aligns with “remember what I have.”

**Gaps vs CONTEXT**

- Tags are **frontmatter/search fields**, not user-maintained many-to-many overlay.
- No collections; no multi-agent enable matrix; no invocation-hint UX for humans.
- Dedupe by **name**, not realpath.
- Very early / low adoption.

**Fork/extend viability:** **Ideas and small modules** (frontmatter parse, scoring search). Not a full product base. Worth reading `discover.py` / router schema if implementing search.

---

### 5. vercel-labs/skills — dominant installer; list/lockfile only for inventory

| | |
|--|--|
| **URL** | https://github.com/vercel-labs/skills |
| **License** | MIT (README) |
| **Language** | TypeScript |
| **Activity** | ~26k★, very active, npm `skills` |
| **Problem** | Install/discover/update/remove skills across many agents from git sources |

**Inventory-relevant surface**

- `npx skills list` / `ls` — installed skills (project + global), filter by agent.
- Lockfile pattern (e.g. `~/.agents/.skill-lock.json`): source URL, skill path, content hash, timestamps—**install provenance**, not user taxonomy.
- Symlink-or-copy fan-out; path table for 70+ agents (**Grok not listed**).

**What is *not* inventory taxonomy**

- No user tags, favorites, or scenario collections.
- List is “what did this CLI install,” not “everything on disk across roots including hand-copied skills.”
- `find` searches the **marketplace/ecosystem**, not local classification.

**Fork/extend viability:** **Do not fork for tags/collections.** Possible **interop**: import lockfile as one scan source; optional later “sync” layer can reuse agent path table ideas (already documented in landscape note). Extending lockfile for tags would fight product identity (install CLI + skills.sh).

---

### 6. gotalab/skillport — tags in metadata + serve/search, not local multi-root inventory

| | |
|--|--|
| **URL** | https://github.com/gotalab/skillport |
| **License** | MIT |
| **Language** | Python |
| **Activity** | ~403★, PyPI `skillport` / `skillport-mcp` |
| **Problem** | SkillOps: validate, manage lifecycle, deliver skills via CLI or MCP (search-first load) |

**Overlap**

- `list` / `add` / `remove` / `update` / `validate`.
- **Organization:** `metadata.skillport.category` + `tags` in **SKILL.md frontmatter** (writes into package—opposite of CONTEXT central index rule).
- Per-client filtering via env (`SKILLPORT_ENABLED_CATEGORIES`, etc.).
- Search-first progressive load pattern for agents with many skills.

**Gaps**

- Skills live under SkillPort’s skills dir (`~/.skillport/skills` default), not multi-vendor root scan.
- Tags mutate skill packages; no user overlay index.
- Primary value is **serve to agents**, not human inventory UX.

**Fork/extend viability:** **Ideas only** (search-first loading, category filters). Do not adopt frontmatter-write tagging.

---

### 7. umutbozdag/agent-skills-manager — multi-source dashboard

| | |
|--|--|
| **URL** | https://github.com/umutbozdag/agent-skills-manager |
| **License** | MIT |
| **Language** | Next.js / TypeScript |
| **Activity** | ~24★ |
| **Problem** | Single dashboard for skills (and rules) scattered across agent dirs |

**Overlap**

- Scans global + project skill dirs for many tools; search; filter by **auto category** and **source**.
- Enable/disable via `SKILL.md` ↔ `SKILL.md.disabled` rename.
- Bulk copy/move/delete; git install; project discovery scan.
- Ships a **manage-skills** agent skill for terminal workflows.

**Gaps**

- Mixes **rules** and single-file configs (AGENTS.md, copilot-instructions)—broader than Skill definition.
- Categories are automatic, not user tags; no collections.
- Web app + node-pty complexity; mutates skill tree for disable.

**Fork/extend viability:** **Ideas** (dashboard filters, bulk ops). Disable-via-rename conflicts with CONTEXT “don’t write into packages / central index.”

---

### 8. beautyfree/skiller — Electron multi-agent install/sync

| | |
|--|--|
| **URL** | https://github.com/beautyfree/skiller |
| **License** | MIT |
| **Language** | TypeScript / Electron |
| **Activity** | ~45★, releases v0.2.x |
| **Problem** | One desktop control center: install, sync, edit skills across 44 agents |

**Overlap:** Dashboard of installed skills; per-agent status; project-scoped skills; marketplace.  
**Gaps:** Install/sync primary; no tags/collections product story.  
**Fork:** Ideas only; Electron surface area high for inventory-only CLI.

---

### 9. vudknguyen/openskill (`osk`) — small multi-agent installer

| | |
|--|--|
| **URL** | https://github.com/vudknguyen/openskill |
| **License** | MIT |
| **Language** | TypeScript |
| **Activity** | ~9★ |
| **Problem** | Lightweight agent-agnostic install/list/update/search repos |

**Overlap:** `list`/`ls`, multi-agent targets, repo management.  
**Gaps:** Installer/marketplace login features; no taxonomy.  
**Fork:** Only if needing a tiny installer core—not inventory+tags.

---

### 10. lasoons/AgentSkillsManager — IDE extension installer

| | |
|--|--|
| **URL** | https://github.com/lasoons/AgentSkillsManager |
| **License** | MIT |
| **Language** | TypeScript (VS Code extension) |
| **Activity** | ~98★ |
| **Problem** | Browse git skill repos + cloud catalog; install into active IDE skills dir |

**Overlap:** “Skill Collections” = **preset upstream repositories**, not user scenario bundles. Local skills group shows active directory.  
**Gaps:** Marketplace/install; not multi-root inventory taxonomy.  
**Fork:** No.

---

### 11. wanghuan9/skill-manager (SkillDock) — strong UX, not open source

| | |
|--|--|
| **URL** | https://github.com/wanghuan9/skill-manager |
| **License** | **Closed-source preview** (README: may not copy/modify until OSS) |
| **Stars** | ~275 |
| **Problem** | Desktop skills + MCP + plugins; Git-aware updates; multi-tool sync |

**Overlap (ideas only):** List/group by **source**, multi-tool enable, managed library + links, status filters.  
**Fork:** **Not viable** until licensed OSS.

---

### 12. Pandemonium-Research/Skilldex — skillsets as “collections” pattern

| | |
|--|--|
| **URL** | https://github.com/Pandemonium-Research/Skilldex · paper arXiv:2604.16911 |
| **License** | MIT (repo) |
| **Language** | TypeScript CLI (`skillpm` / `spm`) + registry |
| **Activity** | ~2★ |
| **Problem** | Package manager with conformance scoring + **skillset** bundles |

**Overlap**

- **Skillset** = named bundle of related skills **plus shared assets** (vocabulary, templates)—coherence across skills.
- Hierarchical scopes (global/shared/project) with manifests.
- List/install/validate; MCP tools.

**Gaps vs CONTEXT**

- Registry/package-manager primary; skillsets are **installable packages**, not user tags over existing inventory.
- Stores under Skilldex trees, not multi-vendor scan roots.
- Academic/early tooling.

**Fork/extend viability:** **Steal skillset mental model** carefully: CONTEXT collections are **user scenario bundles of references** (no requirement for shared assets). Skilldex skillsets are closer to “coherent skill packs.” Useful for v2 packaging, not v1 local index.

---

### 13. Vendor list UX (not full products)

#### Gemini CLI skills

- **Docs:** https://geminicli.com/docs/cli/skills/
- `gemini skills list|install|uninstall`; in-session `/skills list|link|enable|disable|reload`.
- Enable/disable scoped user/workspace; link local path.
- **Overlap:** list + enable/disable + link (adopt-ish).  
- **Gaps:** Single vendor; no tags/collections; install still present.

#### Claude / Codex / Grok

- In-session **list** (`/skills`, `$…`, `grok inspect`) is discovery for the **current agent**, not cross-tool inventory.
- Config-level disable lists exist per vendor (see landscape note)—not a shared taxonomy.

#### `gh skill`

- **Changelog:** https://github.blog/changelog/2026-04-16-manage-agent-skills-with-github-cli/
- Discover/install/manage/publish skills from GitHub—**install path**, Copilot-oriented.

---

### 14. VersoXBT/skill-manager — Claude-only inventory plugin

| | |
|--|--|
| **URL** | https://github.com/VersoXBT/skill-manager |
| **License** | MIT · ~6★ · JS |
| **Problem** | `/skill-check` inventory, structure lint, update checks for Claude |

**Overlap:** Read-only inventory table, token estimates, structure issues.  
**Gaps:** Claude-only; no tags/collections; fix mutates frontmatter on user skills.  
**Fork:** Ideas for audit report fields only.

---

### 15. Pattern donors (adjacent domains)

#### knqyf263/pet — CLI snippet manager (tags + select)

| | |
|--|--|
| **URL** | https://github.com/knqyf263/pet |
| **License** | MIT · ~5.3k★ · Go |

- Save / **tag** / search / **select** (fzf) / exec snippets; TOML store; multi snippet dirs.
- **Transferable:** Human “I have too many short procedures—find and invoke” UX maps to skill **选用**; tags as many-to-many filters; selectorcmd pattern; tags never require editing the snippet’s *payload* semantics separately if you keep metadata in the store (pet co-locates tags in TOML—CONTEXT prefers **sidecar index**).

#### jarun/buku — bookmark CLI with tags

| | |
|--|--|
| **URL** | https://github.com/jarun/buku |
| **License** | GPL-3.0 · ~7.2k★ |

- Local SQLite index of URLs with **tags**, search, CLI-first.
- **Transferable:** Tag query language, DB schema for many-to-many labels over **immutable resource IDs** (bookmarks ↔ realpath skill ids).

#### twpayne/chezmoi / GNU stow — multi-target path management

| | |
|--|--|
| **URL** | https://github.com/twpayne/chezmoi (MIT, ~20k★); stow (GPL) |

- Declarative mapping of source → many destination paths; symlink strategies.
- **Transferable for secondary CONTEXT goal (cross-CLI sync)** only—not v1 inventory. Avoid treating skill packages as “dotfiles to own” if v1 is index-only.

---

## Best bases to fork vs ideas-only

### Do not expect a drop-in fork for CONTEXT v1

CONTEXT’s combination is still **underserved as a focused OSS product**:

1. Inventory of **already installed** skills (not install-from-registry).  
2. User **tags** + named **collections** as scenario bundles.  
3. **User-level central index** that does **not** write into skill packages.  
4. **realpath** identity with multi-root scan and symlink merge.  
5. Human **select** with info + **invocation hints** (not agent launch).  
6. Multi-tool path matrix **including Grok**, with sync as **secondary**.

### Ranking for *this* product

| Rank | Source | Role | Rationale |
|------|--------|------|-----------|
| **1** | **xingkongliang/skills-manager** | **Best ideas base** (not full fork) | Only mature OSS with **tags + presets≈collections + adopt + multi-tool incl. Grok + CLI**. Invert product: keep taxonomy, drop marketplace gravity, replace library-SoT with **index over scan roots**. |
| **2** | **mode-io/skill-manager** | **Best “inventory binding” ideas** | Adopt → shared inventory → enable/disable matrix; “needs review” state; link bindings. Map to CONTEXT as **index + optional later sync**, without mandatory re-home if v1 forbids it. |
| **3** | **ASAM + chemny** (pair) | **Best scan/index skeleton ideas** | Pure inventory scan→search; file or SQLite index. Steal search/router patterns; rebuild path identity + taxonomy. |
| **Honorable** | **vercel-labs/skills** | **Interop only** | List/lockfile/agent path table; do not extend for tags. Import lockfile as a discovery signal. |
| **Honorable** | **pet** (+ optional **buku**) | **UX pattern donors** | Tag + interactive select + “remember how to run this” is the human job. |
| **Honorable** | **Skilldex skillsets** | **Collections design foil** | Named bundles; contrast with CONTEXT (references only vs shared assets package). |
| **Avoid as base** | SkillDock (closed), Skiller/AgentSkillsManager (install GUI), SkillPort frontmatter tags | Wrong license or wrong mutation model | |

### Practical recommendation

1. **Greenfield CLI** with user-level index (SQLite or JSON) keyed by **realpath**.  
2. **Copy design**, not code, from:
   - skills-manager **Presets** → Collections (scenario bundles; apply = filter/select, not necessarily agent sync).  
   - skills-manager **Tags** + Untagged filter.  
   - mode-io **Adopt** language for “skills already on disk.”  
   - ASAM **weighted search** over name/description.  
   - pet **select** UX for human 选用.  
3. **Later** optional sync: symlink fan-out patterns from vercel-labs / skills-manager / mode-io—secondary per CONTEXT.  
4. **Interop:** read `skills list` / lockfile and vendor disable configs as inputs, never as the taxonomy store.

### If almost nothing matches inventory+tags…

**That is the honest assessment for pure open-source maturity.** Mature OSS clusters around **install/sync GUIs** and **install CLIs**. User-level **tags + scenario collections over multi-root local skills** appear mainly in **one mature desktop app** (skills-manager presets/tags) and **early inventory experiments** (chemny, ASAM). The white space for a **lean, index-only, human-select CLI** remains real—and is the differentiated greenfield niche relative to skills.sh / `npx skills`.

---

## Sources

### Primary project pages

- https://github.com/xingkongliang/skills-manager  
- https://github.com/mode-io/skill-manager  
- https://github.com/chemny/agent-skill-manager  
- https://github.com/cikorsky/agent-skill-asset-manager  
- https://github.com/vercel-labs/skills  
- https://github.com/gotalab/skillport  
- https://github.com/umutbozdag/agent-skills-manager  
- https://github.com/beautyfree/skiller  
- https://github.com/vudknguyen/openskill  
- https://github.com/lasoons/AgentSkillsManager  
- https://github.com/wanghuan9/skill-manager (SkillDock; closed source stated in README)  
- https://github.com/Pandemonium-Research/Skilldex  
- https://github.com/VersoXBT/skill-manager  
- https://github.com/knqyf263/pet  
- https://github.com/jarun/buku  
- https://github.com/twpayne/chezmoi  

### Vendor / ecosystem docs

- https://geminicli.com/docs/cli/skills/  
- https://github.blog/changelog/2026-04-16-manage-agent-skills-with-github-cli/  
- https://skills.sh  
- https://agentskills.io  

### Research

- Skilldex paper: https://arxiv.org/html/2604.16911v1  

### Internal context

- [`CONTEXT.md`](../../CONTEXT.md)  
- [`skills-management-landscape.md`](./skills-management-landscape.md) (install paths, registries—not re-covered here)

### Metadata note

Star counts and push dates sampled via GitHub API on **2026-07-14**; re-check before citing in product materials.

---

*End of research note. The competitive set moves quickly; re-verify READMEs before basing architecture on any single project’s storage model.*
