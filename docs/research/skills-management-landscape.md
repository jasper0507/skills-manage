# Agent Skills Management Landscape

**Date:** 2026-07-14  
**Purpose:** Primary-source research for a greenfield unified skills management tool spanning Codex CLI, Grok Build / Grok CLI, Claude Code, and related agent CLIs.  
**Method:** Official docs, GitHub READMEs/specs, and local Grok CLI documentation where public web docs are thin.

---

## Executive summary

An **agent skill** is a directory containing a required `SKILL.md` (YAML frontmatter + Markdown instructions), optionally with `scripts/`, `references/`, and `assets/`. The format is an open standard at [agentskills.io](https://agentskills.io), originally developed by Anthropic and adopted by a large set of coding agents (Claude Code, OpenAI Codex, Cursor, Gemini CLI, VS Code / GitHub Copilot, OpenCode, Goose, Grok CLI, and many others).

**Progressive disclosure** is the shared runtime model: agents load only `name` + `description` at session start; full `SKILL.md` body loads on activation; bundled files load on demand.

**Storage is fragmented.** There is no single global path. Common patterns:

| Pattern | Examples |
|--------|----------|
| Vendor-specific | `~/.claude/skills`, `~/.grok/skills`, `~/.gemini/skills`, `~/.cursor/skills` |
| Interop / “universal” | `~/.agents/skills`, `.agents/skills` (Codex primary; Cursor/Gemini aliases) |
| Secondary compat | Many agents also scan `.claude/skills` or each other’s dirs |

**Installation tooling is dominated by Vercel’s `npx skills`** ([vercel-labs/skills](https://github.com/vercel-labs/skills), MIT, ~26k★, active as of 2026-07-14), paired with the [skills.sh](https://skills.sh) discovery directory. It supports 70+ agents via symlink-or-copy into each agent’s project/global skills tree, tracks installs in a lockfile (e.g. `~/.agents/.skill-lock.json`), and ships a `find-skills` meta-skill. **Grok is not listed among its supported agents**, though Grok natively reads `.agents/skills/`, `.claude/skills/`, and `.cursor/skills/`.

**Portability of `SKILL.md` content is high** for the core frontmatter (`name`, `description`) and Markdown body. **Portability of behavior is medium**: Claude Code (and partially Cursor/VS Code) add proprietary frontmatter (`context: fork`, `hooks`, dynamic `!` shell injection, `allowed-tools` semantics). **Portability of install paths is low** without a multi-target installer or symlink strategy.

**Security model is mostly “agent sandbox + user consent,” not skill signing.** Skills can ship executable scripts; tools rely on workspace trust, permission prompts, optional skill-load consent (Gemini), allow/deny skill permissions (OpenCode), and OS sandboxes—not package signatures or a universal allowlist.

**Gaps for a unified manager (Codex / Grok / Claude Code specifically):** no first-class Grok target in the leading CLI; Codex path ambiguity (`~/.agents/skills` vs `~/.codex/skills`); vendor-specific frontmatter and invocation UX (`/` vs `$` vs tools); weak versioning (git SHAs/hashes in lockfiles, not semver registries); name collisions across scopes; no shared trust/signing story; enterprise managed paths differ (`/etc/codex/skills`, Claude managed settings).

---

## 1. What is an “agent skill”?

### 1.1 Open standard (agentskills.io)

**Sources:** [agentskills.io/home](https://agentskills.io/home), [agentskills.io/specification](https://agentskills.io/specification), [github.com/agentskills/agentskills](https://github.com/agentskills/agentskills) (Apache-2.0 code; ~23k★).

A skill is a directory:

```text
skill-name/
├── SKILL.md          # Required: metadata + instructions
├── scripts/          # Optional: executable code
├── references/       # Optional: documentation
├── assets/           # Optional: templates, resources
└── ...
```

**`SKILL.md`:** YAML frontmatter + Markdown body.

| Field | Required | Constraints (spec) |
|-------|----------|-------------------|
| `name` | Yes | Max 64 chars; lowercase alphanumerics + hyphens; no leading/trailing hyphen; no consecutive hyphens; **must match parent directory name** |
| `description` | Yes | Max 1024 chars; what it does **and when to use it** |
| `license` | No | License name or file reference |
| `compatibility` | No | Max 500 chars; env / product requirements |
| `metadata` | No | String key→value map (e.g. author, version) |
| `allowed-tools` | No | Space-separated pre-approved tools (**experimental**) |

**Progressive disclosure (spec):**

1. **Metadata** (~100 tokens) — always loaded  
2. **Instructions** (full body; recommend &lt;5000 tokens / keep under ~500 lines) — on activation  
3. **Resources** — on demand  

**Validation:** [skills-ref](https://github.com/agentskills/agentskills/tree/main/skills-ref) (`skills-ref validate ./my-skill`).

### 1.2 Vendor extensions (same file, extra frontmatter)

| Vendor | Notable extensions | Source |
|--------|-------------------|--------|
| **Claude Code** | `disable-model-invocation`, `user-invocable`, `allowed-tools` / `disallowed-tools`, `context: fork`, `agent`, `hooks`, `paths`, `model`, `effort`, `when_to_use`, `arguments`, dynamic `` !`cmd` `` injection, `${CLAUDE_SKILL_DIR}` / `${CLAUDE_PROJECT_DIR}` | [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills) |
| **Cursor** | `paths` (globs), `disable-model-invocation`, nested monorepo skill dirs | [cursor.com/docs/skills](https://cursor.com/docs/skills) |
| **Codex** | Optional `agents/openai.yaml` (UI metadata, `allow_implicit_invocation`, MCP tool deps); `$skill` / `/skills` invocation | [learn.chatgpt.com docs build-skills](https://learn.chatgpt.com/docs/build-skills) |
| **VS Code / Copilot** | `argument-hint`, `user-invocable`, `disable-model-invocation`, experimental `context: fork` | [code.visualstudio.com … agent-skills](https://code.visualstudio.com/docs/copilot/customization/agent-skills) |
| **OpenCode** | Spec fields only in skill file; **permissions** for skill loading in `opencode.json` | [opencode.ai/docs/skills](https://opencode.ai/docs/skills/) |
| **Grok CLI** | `when-to-use`, `allowed-tools`, `argument-hint`, `user-invocable`, `disable-model-invocation`, `model`, `effort`, plus Claude-like slash invocation | Local: `~/.grok/docs/user-guide/08-skills.md` |

Agents that follow the open standard **ignore unknown frontmatter keys** (explicitly stated for OpenCode; implied by agentskills.io design). That keeps basic skills portable even when extended fields exist.

### 1.3 Related-but-not-skills

| Artifact | Role vs skills |
|----------|----------------|
| `AGENTS.md` / `CLAUDE.md` | Always-on project instructions; not progressive skill packages |
| Claude `.claude/commands/*.md` | Legacy custom slash commands; Claude docs say **merged into skills** (still work) |
| Cursor rules (`.mdc`, alwaysApply/globs) | Static or path-scoped policy; Cursor prefers skills for dynamic workflows |
| MCP servers | Tools/data connections; skills teach *procedures* that may use MCP |
| Plugins (Claude / Codex / Grok) | Packaging unit that can *contain* skills + hooks + MCP |

---

## 2. Where CLIs store skills

Paths below are from official docs unless marked **observed** (local install / lockfile evidence).

### 2.1 Comparison table (primary targets + peers)

| Agent | Project paths | User / global paths | Env / config notes |
|-------|---------------|---------------------|--------------------|
| **Claude Code** | `.claude/skills/<name>/SKILL.md`; nested package `.claude/skills/`; plugins `…/skills/` | `~/.claude/skills/`; enterprise managed settings | `CLAUDE_CONFIG_DIR` relocates all `~/.claude` paths. Precedence: enterprise &gt; personal &gt; project; personal overrides project for same name. Symlinks supported for skill dirs. |
| **OpenAI Codex** | `.agents/skills` from **CWD up to repo root** (multiple REPO tiers) | **`$HOME/.agents/skills`** (official USER); admin `/etc/codex/skills`; SYSTEM bundled | Docs also describe plugins and `$skill-installer`. Community/older notes and Vercel CLI also use `~/.codex/skills/`. Symlinks supported. Disable via `[[skills.config]]` in `~/.codex/config.toml`. |
| **Grok CLI / Grok Build** | `./.grok/skills/`, `<repo>/.grok/skills/`, also `.agents/skills/`, `.claude/skills/`, `.cursor/skills/` | `~/.grok/skills/`; also `~/.claude/skills/`, `~/.cursor/skills/` (compat, configurable) | **Public web docs are thin**; authoritative text ships with CLI (`~/.grok/docs/user-guide/08-skills.md`, README). Config: `[skills] paths / ignore / disabled` in `~/.grok/config.toml`. Compat toggles: `GROK_CLAUDE_SKILLS_ENABLED`, `GROK_CURSOR_SKILLS_ENABLED`. Bundled skills under `~/.grok/skills` and `~/.grok/bundled/skills`. |
| **Cursor** | `.agents/skills/`, `.cursor/skills/`; nested monorepo dirs | `~/.agents/skills/`, `~/.cursor/skills/` | **Also loads** `.claude/skills/`, `.codex/skills/`, `~/.claude/skills/`, `~/.codex/skills/` for compatibility. Recursive walk under skills roots. |
| **Gemini CLI** | `.gemini/skills/` or **`.agents/skills/`** (alias; **`.agents` wins** same tier) | `~/.gemini/skills/` or `~/.agents/skills/` | Built-in + extension skills. Install: `gemini skills install <git-url>`. Activation requires **user consent**. Note: unpaid tier migration to Antigravity CLI announced. |
| **VS Code / GitHub Copilot** | `.github/skills/`, `.claude/skills/`, `.agents/skills/` | `~/.copilot/skills/`, `~/.claude/skills/`, `~/.agents/skills/` | Extra roots via `chat.agentSkillsLocations`. `gh skill` CLI for install. |
| **OpenCode** | `.opencode/skills/`, `.claude/skills/`, `.agents/skills/` | `~/.config/opencode/skills/`, `~/.claude/skills/`, `~/.agents/skills/` | Walks up to git worktree for project paths. |
| **Windsurf** (via `npx skills` map) | `.windsurf/skills/` | `~/.codeium/windsurf/skills/` | Confirm against Windsurf docs before treating as sole path. |
| **Continue** (via `npx skills` map) | `.continue/skills/` | `~/.continue/skills/` | — |
| **Goose** | Marketplace + skill docs under goose-docs / AAIF | Block/AAIF goose has skills marketplace UI | Using-skills guide URL moved with site migration; marketplace at [goose-docs.ai/skills](https://goose-docs.ai/skills). |
| **Aider** | Not a primary SKILL.md consumer in agentskills client list | — | Vercel maps **AiderDesk** (not classic Aider) to `.aider-desk/skills/`. |

### 2.2 Claude Code (detail)

**Source:** [Extend Claude with skills](https://code.claude.com/docs/en/skills), [Explore the .claude directory](https://code.claude.com/docs/en/claude-directory).

| Scope | Path |
|-------|------|
| Personal | `~/.claude/skills/<skill-name>/SKILL.md` |
| Project | `.claude/skills/<skill-name>/SKILL.md` |
| Plugin | `<plugin>/skills/<skill-name>/SKILL.md` → `/plugin-name:skill-name` |
| Enterprise | Managed settings |

- Command name for disk skills comes from **directory name**, not frontmatter `name` (except plugin-root `SKILL.md`).
- Nested monorepo skills get qualified names (`/apps/web:deploy`).
- Live reload of `SKILL.md` without restart for watched skill dirs.
- `--add-dir` also loads `.claude/skills/` from added directories (exception to “file access only”).

### 2.3 Codex CLI (detail)

**Source:** [Build skills](https://learn.chatgpt.com/docs/build-skills) (OpenAI Codex docs; `developers.openai.com/codex/skills` redirects here).

| Scope | Location |
|-------|----------|
| REPO | `$CWD/.agents/skills`, parent folders’ `.agents/skills`, `$REPO_ROOT/.agents/skills` |
| USER | `$HOME/.agents/skills` |
| ADMIN | `/etc/codex/skills` |
| SYSTEM | Bundled with Codex (e.g. skill-creator; catalog also at [github.com/openai/skills](https://github.com/openai/skills)) |

- Explicit: `$skill-name` or `/skills`; implicit via description match.
- Initial skill list budget: ≤2% of context window (or 8k chars if unknown)—descriptions may be shortened/omitted when many skills installed.
- Install curated skills: `$skill-installer`; create: `$skill-creator` / Record & Replay.
- Optional `agents/openai.yaml` for ChatGPT desktop UI + invocation policy.

**Path caveat for managers:** Vercel `skills` CLI installs Codex **global** skills to `~/.codex/skills/` while OpenAI’s published table emphasizes `$HOME/.agents/skills`. Both appear in the wild; Cursor explicitly loads both `.codex/skills` and `.agents/skills`. A unified manager should write **both** or document a single canonical + symlink.

### 2.4 Grok CLI (detail)

**Sources:** shipped docs `~/.grok/docs/user-guide/08-skills.md`; `~/.grok/README.md` (Skills section). **Not** prominently documented on public marketing sites as of this research date.

**Discovery priority (highest → lowest, simplified):**

1. `./.grok/skills/` (and `./.grok/commands/`)
2. Repo-root `.grok/skills/`
3. Project `.claude/skills/`, `.cursor/skills/`
4. `.agents/skills/` at each tier (scanned alongside `.grok/`)
5. User: `~/.grok/skills/`, `~/.claude/skills/`, `~/.cursor/skills/`

- Dedup by skill name: higher priority wins.
- Slash invocation: `/skill-name`; list via `/skills`, inspect via `grok inspect`.
- Collision handling: qualified names `local:`, `repo:`, `user:`, or plugin name.
- Extra roots: `[skills].paths` in config; disable names with `[skills].disabled`.
- **Claude Code compatibility** loads Claude skills/commands/plugins components.
- Observed on a live install: `~/.grok/skills/` (bundled helpers like `create-skill`, `code-review`), `~/.grok/bundled/skills/`, and shared installs under `~/.agents/skills/` with `.skill-lock.json` from Vercel skills CLI.

**Implication:** Installing only into `~/.agents/skills` or `~/.claude/skills` already reaches Grok without a dedicated `~/.grok/skills` copy—but project-scoped Grok-first workflows prefer `.grok/skills/`.

### 2.5 Cursor (detail)

**Source:** [cursor.com/docs/skills](https://cursor.com/docs/skills).

- Project: `.agents/skills/`, `.cursor/skills/`
- User: `~/.agents/skills/`, `~/.cursor/skills/`
- Compat: Claude + Codex paths listed above
- Nested `.cursor/skills` / `.agents/skills` under packages auto-scoped to that directory
- GitHub import via Customize UI “Remote Rule (Github)”
- Built-in `/create-skill`, `/migrate-to-skills`

### 2.6 Gemini CLI (detail)

**Source:** [geminicli.com/docs/cli/skills](https://geminicli.com/docs/cli/skills/).

- User: `~/.gemini/skills/` **or** `~/.agents/skills/`
- Workspace: `.gemini/skills/` **or** `.agents/skills/` (`.agents` precedence within tier)
- `gemini skills install|uninstall|list`; `/skills link|enable|disable|reload`
- Activation tool + **consent** before injecting skill body and granting skill-dir file access

---

## 3. Existing open-source managers, installers, and registries

### 3.1 `npx skills` / `add-skill` — **vercel-labs/skills** (dominant)

| | |
|--|--|
| **URL** | https://github.com/vercel-labs/skills |
| **npm** | `skills` (also bin `add-skill`), v1.5.17 as of 2026-07-13 |
| **License** | MIT (`npm view skills license`) |
| **Activity** | Very high: ~26k★, push 2026-07-14 |
| **Registry UI** | https://skills.sh |
| **Problem solved** | Install/discover/update/remove skills across many agents from git sources |

**Commands (selected):**

```bash
npx skills add vercel-labs/agent-skills
npx skills add owner/repo --skill frontend-design -g -a claude-code -y
npx skills find typescript
npx skills list | update | remove | init
npx skills use owner/repo@skill-name --agent claude-code
```

**Install mechanism:** Clone/fetch git sources (GitHub shorthand, full URL, GitLab, any git, local path); discover `SKILL.md` under known container dirs (`skills/`, `.agents/skills/`, `.claude/skills/`, etc.); install via **symlink (default)** or **copy** into each agent’s project or global skills directory.

**Lockfile (observed):** `~/.agents/.skill-lock.json` schema includes `version`, `skills` map with `source`, `sourceType`, `sourceUrl`, `skillPath`, `skillFolderHash`, `pluginName`, `installedAt`, `updatedAt`.

**Agents:** OpenCode, Claude Code, Codex, Cursor, Gemini CLI, Windsurf, Continue, Goose, Copilot, and 60+ more—**Grok not listed**.

**Gaps:** No Grok agent id; no cryptographic signing; telemetry-based skills.sh discovery rather than formal publish API; “versioning” is content hash / re-fetch, not semver packages; agent path table can drift from vendor docs (Codex global path example).

**Related meta-skill:** `find-skills` in the same repo / installed into agent skill dirs.

### 3.2 Official skill catalogs (content, not full managers)

| Project | URL | Notes |
|---------|-----|-------|
| **anthropics/skills** | https://github.com/anthropics/skills | Reference + example skills; many Apache-2.0; docx/pdf/pptx/xlsx source-available. ~161k★. |
| **openai/skills** | https://github.com/openai/skills | Codex catalog: `.curated`, `.system`, experimental layouts. ~24k★. |
| **vercel-labs/agent-skills** | https://github.com/vercel-labs/agent-skills | Vercel’s skill pack (React/Next, web design, etc.). MIT stated in README. ~29k★. Install via `npx skills add vercel-labs/agent-skills`. |
| **agentskills/agentskills** | https://github.com/agentskills/agentskills | Spec + `skills-ref` validator. Apache-2.0. |
| **google/skills** | Community reports of official Gemini skill pack | Install example: `gemini skills install https://github.com/google/skills.git` |
| **composiohq/awesome-codex-skills** | https://github.com/composiohq/awesome-codex-skills | Curated list + install hints (often via Codex skill-installer scripts). |
| **github/awesome-copilot** | https://github.com/github/awesome-copilot | Copilot community skills/agents/prompts. |
| **mattpocock/skills** | https://github.com/mattpocock/skills | Popular engineering skill pack; appears in observed lockfiles. |

### 3.3 Other managers / shims (smaller)

| Project | URL | Role | Maturity (stars, rough) |
|---------|-----|------|-------------------------|
| **AgentSkillsManager** | https://github.com/lasoons/AgentSkillsManager | Multi-IDE extension; cloud catalog ~58k skills claim | ~98★ |
| **skiller** | https://github.com/beautyfree/skiller | Desktop app: Claude/Cursor/Codex skill sync | ~45★ |
| **umutbozdag/agent-skills-manager** | https://github.com/umutbozdag/agent-skills-manager | Dashboard for Cursor/Claude/Agents | ~24★ |
| **openskill** | https://github.com/vudknguyen/openskill | “Universal” manager | ~9★ |
| **skillz.sh / asma / skill-manager / aiasm** | various | Niche CLIs | ≤5★ each |
| **Skilldex** | arXiv:2604.16911 | Research package manager + registry (academic) | Paper |
| **GitHub `gh skill`** | Copilot docs | Discover/install skills for Copilot | Official but Copilot-scoped |
| **Codex `$skill-installer`** | Built into Codex | Local curated install | Vendor-specific |
| **Gemini `gemini skills`** | Built into Gemini CLI | install/link/enable | Vendor-specific |
| **Claude plugins marketplace** | Claude Code docs | Plugin packaging for skills + more | Vendor-specific |

### 3.4 Package-manager analogies

| Model | Reality today |
|-------|----------------|
| **npm-like** | Closest: `npx skills` + skills.sh install telemetry; **not** a real registry with versioned tarballs |
| **Git-as-source** | Dominant distribution unit |
| **Lockfiles** | Content hashes (`skillFolderHash`), not semver ranges |
| **Internal flags** | `metadata.internal: true` + `INSTALL_INTERNAL_SKILLS=1` in Vercel CLI |

---

## 4. Skill distribution models

| Model | How it works | Pros | Cons |
|-------|--------------|------|------|
| **Git repo / monorepo skill pack** | Repo with `skills/<name>/SKILL.md` or nested categories | Simple; PR reviewable; default for Vercel/Anthropic/OpenAI catalogs | No fine-grained version pin without commit SHA |
| **GitHub tree URL** | `…/tree/main/skills/foo` | Single-skill install | Still git-coupled |
| **`npx skills add`** | CLI materializes into agent dirs | Multi-agent; symlink updates | Ecosystem dependency on one CLI |
| **Vendor plugin marketplaces** | Claude plugins, Codex plugins | Bundles MCP + skills + hooks | Vendor lock-in |
| **Commit project skills** | `.claude/skills` or `.agents/skills` in repo | Team sync via git | Multi-agent repos may need multiple trees or symlinks |
| **Symlink farms** | One canonical store → many agent paths | DRY | Windows/symlink policy; broken links |
| **Copy per agent** | Independent trees | Robust | Drift on update |
| **Git submodules** | Submodule skill packs | Explicit pin | Submodule UX cost |
| **Raw URL / curl** | Fetch single SKILL.md | Quick | No package integrity; rare officially |
| **Hosted API skills** | OpenAI API Skills upload for containers | Versioned remote skills for hosted agents | Different lifecycle from local CLI dirs |
| **npm package of skills** | Theoretical | Familiar | Not standard; skills are folders not JS modules |

**skills.sh “publish” model** (Vercel KB): put skills in a git repo and share it; installs via `npx skills add` surface on the directory via telemetry—**no formal submit API**.

---

## 5. Interop and portability

### 5.1 What ports cleanly

- Directory + `SKILL.md` with `name` + `description` + Markdown body  
- Relative links to `scripts/`, `references/`, `assets/`  
- Progressive disclosure semantics across major agents  
- Using **`.agents/skills`** as a shared project root for Codex, Cursor, Gemini, OpenCode, Copilot (and Grok discovery)

### 5.2 What does not port cleanly

| Issue | Detail |
|-------|--------|
| **Root directory names** | `.claude` vs `.agents` vs `.grok` vs `.cursor` vs `.github` |
| **Name collisions** | Same `name` in personal vs project: precedence differs (Claude: enterprise &gt; personal &gt; project; Gemini: workspace &gt; user &gt; extension; Grok: path priority table) |
| **Command surface** | Claude/Grok `/name`; Codex `$name` / `/skills`; OpenCode `skill` tool; Gemini `activate_skill` tool |
| **Proprietary frontmatter** | `context: fork`, hooks, `` !`cmd` ``, Claude-only substitutions |
| **`allowed-tools` meaning** | Spec experimental; Claude grants without prompt when skill active; not universal |
| **Global Codex path** | Docs vs installers disagree (`~/.agents/skills` vs `~/.codex/skills`) |
| **Grok not in Vercel agent table** | Installs may still work if targeting `.agents` or `.claude` because Grok scans those |

### 5.3 Symlink strategies (common practice)

1. **Canonical store** → symlink into each agent path (`npx skills` default).  
2. **Claude-primary store** → Grok/Cursor/OpenCode also read `.claude/skills` (partial multi-read).  
3. **`.agents/skills` as team standard** in repo; agents that only read vendor dirs need a manager to fan-out.  
4. Claude docs: skill directory entries may be **symlinks**; duplicate targets load once.

### 5.4 Adapters

- Runtime adapters are rare; most “adapters” are **install-time path mappers** (Vercel skills agent table).  
- Claude → Gemini conversion tools exist in community (e.g. skill migration repos)—quality varies.  
- Cursor `/migrate-to-skills` converts Cursor rules/commands → skills **within Cursor**, not cross-CLI.

---

## 6. Security and trust model

There is **no ecosystem-wide skill signing or notarization** comparable to signed npm packages with provenance. Trust is local and agent-mediated.

| Mechanism | Who | Notes |
|-----------|-----|-------|
| **Workspace trust** | Claude Code | Project skills’ `allowed-tools` apply after trust dialog; review project skills before trusting |
| **Disable skill shell execution** | Claude Code | `disableSkillShellExecution` replaces `` !`cmd` `` with policy message for non-bundled skills |
| **Permission rules on Skill tool** | Claude Code | Deny `Skill` or allow `Skill(name)` patterns |
| **skillOverrides** | Claude Code | Hide/disable skills from settings without editing SKILL.md |
| **Activation consent** | Gemini CLI | User confirms skill name, purpose, path before load; skill dir added to allowed paths |
| **Skill load permissions** | OpenCode | `permission.skill` allow/deny/ask patterns in config |
| **Install `--consent`** | Gemini CLI | Skip interactive security prompt on install (acknowledges risk) |
| **Scripts as ordinary agent tool use** | Most agents | Skill scripts run under normal shell/tool permission + sandbox profiles |
| **OS sandbox** | Codex, Grok, etc. | Separate from skill packaging; limits blast radius of untrusted instructions/scripts |
| **VS Code terminal approvals** | Copilot | Allow-lists for terminal; docs warn to review shared skills |
| **No signing** | Ecosystem | Git host + human review + hash in lockfile are the practical integrity tools |

**Threat model notes for a unified manager:**

- Installing a skill is equivalent to adding **prompt injection surface + optional executable code** into agent context.  
- Symlink installs mean updating the canonical tree updates all agents immediately—good for patching, bad if supply chain is compromised.  
- Enterprise needs: allowlists of sources, read-only system skill dirs (`/etc/codex/skills`), managed Claude settings, disable model invocation for dangerous workflows (`deploy`).

---

## 7. Gaps for a unified manager (Codex CLI · Grok · Claude Code)

### 7.1 What already exists

- Shared **format** (agentskills.io)  
- Mature multi-agent installer (**`npx skills`**) covering Claude + Codex (and Cursor etc.)  
- Overlapping discovery: Grok reads Claude + agents + cursor paths  
- Lockfile + content hash pattern to copy  

### 7.2 Gaps a new tool should close

| Gap | Why it matters |
|-----|----------------|
| **First-class Grok targets** | Vercel CLI has no `grok` agent; project installs to `.grok/skills` and global `~/.grok/skills` are not automated by the ecosystem leader |
| **Codex path normalization** | Dual reality of `~/.agents/skills` vs `~/.codex/skills` (and admin `/etc/codex/skills`) |
| **Single source of truth without N copies** | Symlinks work but need Windows story, CI story, and conflict policy |
| **True versioning** | Pin by git tag/commit + migrate; optional semver overlays; reproducible team installs |
| **Cross-CLI inventory** | One `list` showing where each skill is visible (Claude only / Grok only / all) |
| **Name collision policy** | Explicit rules when `deploy` exists in `.claude`, `.grok`, and `.agents` |
| **Frontmatter lint / portability report** | Flag Claude-only features when targeting Codex/Grok |
| **Trust & provenance** | Source allowlist, optional checksum verification, “review diff before update” |
| **Enterprise/managed layers** | Claude managed skills, Codex ADMIN path, Grok “server” skill store (mentioned in `grok inspect` sources)—not unified |
| **Invocation UX docs** | Same skill invoked as `/x`, `$x`, or auto—manager can’t fix UX but can document per target |
| **Update fan-out** | Update once → all linked agents; detect broken symlinks / copy drift |
| **Offline / air-gap** | Vendoring skill packs without skills.sh telemetry |
| **Testing** | Claude skill-creator evals exist; no cross-harness skill test runner |

### 7.3 Suggested positioning for a greenfield manager

Relative to `npx skills`, a differentiated tool might:

1. **Default agent set:** `claude-code`, `codex`, `grok` with correct dual paths for Codex and multi-root for Grok.  
2. **Canonical store:** e.g. `~/.agents/skills` + repo `.agents/skills`, with **optional** fan-out into `.claude` / `.grok` when those agents need exclusive roots.  
3. **Manifest** (`skills.lock` / `skills.json`) with source URL, resolved commit, content hash, targets[].  
4. **Portability check:** validate against agentskills.io + report vendor-specific keys.  
5. **Security defaults:** never auto-approve skill scripts; show tree summary on install; support source allowlists.  
6. **Interop with Vercel CLI:** import/export lockfiles rather than fighting the ecosystem.

---

## 8. Implications for a unified skills manager

1. **Treat agentskills.io as the interchange format**, not any single vendor’s frontmatter.  
2. **Path matrix is the product**—format is mostly solved; discovery roots are not.  
3. **Prefer `.agents/skills` for shared project skills** (Codex official, Cursor/Gemini/OpenCode/Copilot-friendly, Grok-scanned); add `.claude/skills` when Claude-only features or Claude-first teams require it; add `.grok/skills` for Grok-primary project skills and highest Grok priority.  
4. **Symlink-first, copy-fallback** is proven by Vercel; reimplement carefully with lockfiles.  
5. **Do not assume skills.sh is a real package registry**—design for git sources first.  
6. **Grok documentation is local-first**—encode Grok paths from CLI docs (`08-skills.md`) in the tool; don’t wait for a public skills page.  
7. **Security is layered outside the package format**—integrate with each agent’s permission model rather than inventing signing day one (unless enterprise requires it).  
8. **Validate with real installs** on Claude Code, Codex, and Grok: same skill, three roots, collision cases, update flows.

---

## Appendix A — Quick path cheat sheet

```text
# Claude Code
~/.claude/skills/<name>/SKILL.md
.claude/skills/<name>/SKILL.md

# Codex (official docs)
$HOME/.agents/skills/<name>/SKILL.md
.agents/skills/<name>/SKILL.md          # CWD → repo root
/etc/codex/skills/...                   # admin

# Codex (also seen / Vercel global)
~/.codex/skills/<name>/SKILL.md

# Grok
~/.grok/skills/<name>/SKILL.md
.grok/skills/<name>/SKILL.md
# also discovers:
.agents/skills/, ~/.agents/skills/
.claude/skills/, ~/.claude/skills/
.cursor/skills/, ~/.cursor/skills/

# Cursor
.agents/skills/, .cursor/skills/
~/.agents/skills/, ~/.cursor/skills/
# + Claude/Codex paths

# Gemini
.gemini/skills/ or .agents/skills/
~/.gemini/skills/ or ~/.agents/skills/

# VS Code / Copilot
.github/skills/, .claude/skills/, .agents/skills/
~/.copilot/skills/, ~/.claude/skills/, ~/.agents/skills/
```

---

## Appendix B — Source list

### Specs and standards
- https://agentskills.io/home  
- https://agentskills.io/specification  
- https://github.com/agentskills/agentskills  

### Claude Code / Anthropic
- https://code.claude.com/docs/en/skills  
- https://code.claude.com/docs/en/claude-directory  
- https://code.claude.com/docs/en/agent-sdk/skills  
- https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview  
- https://github.com/anthropics/skills  

### OpenAI Codex
- https://learn.chatgpt.com/docs/build-skills (Build skills; primary as of redirect from developers.openai.com/codex/skills)  
- https://github.com/openai/codex (docs pointer to skills docs)  
- https://github.com/openai/skills  
- https://developers.openai.com/api/docs/guides/tools-skills (API-hosted skills)  

### Cursor
- https://cursor.com/docs/skills  

### Gemini CLI
- https://geminicli.com/docs/cli/skills/  
- https://geminicli.com/docs/cli/tutorials/skills-getting-started/  

### VS Code / GitHub Copilot
- https://code.visualstudio.com/docs/copilot/customization/agent-skills  
- https://docs.github.com/en/copilot/concepts/agents/about-agent-skills  

### OpenCode / Goose / others
- https://opencode.ai/docs/skills/  
- https://goose-docs.ai/skills  
- https://agentskills.io/home (client showcase listing many products)  

### Installers and directories
- https://github.com/vercel-labs/skills  
- https://github.com/vercel-labs/agent-skills  
- https://skills.sh  
- https://vercel.com/changelog/introducing-skills-the-open-agent-skills-ecosystem  
- https://vercel.com/docs/agent-resources/skills  
- https://vercel.com/kb/guide/agent-skills-creating-installing-and-sharing-reusable-agent-context  

### Grok CLI (local primary sources; weak public web coverage)
- `~/.grok/docs/user-guide/08-skills.md` (shipped with CLI)  
- `~/.grok/README.md` (Skills, Plugins, Claude compatibility, file locations)  
- Installer reference in README: `curl -fsSL https://x.ai/cli/install.sh | bash`  

### Community / secondary (use cautiously)
- https://blog.fsck.com/2025/12/19/codex-skills/ (early Codex path notes)  
- https://github.com/composiohq/awesome-codex-skills  
- Various small “agent skills manager” GitHub projects (see §3.3)  

---

*End of research note. Paths and product behavior change quickly; re-verify against vendor docs before shipping path maps in code.*
