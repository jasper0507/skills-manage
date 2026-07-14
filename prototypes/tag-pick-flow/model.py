"""PROTOTYPE pure model — tag inventory + pick flow. No I/O."""

from __future__ import annotations

from dataclasses import dataclass, field, replace
from typing import Literal


Screen = Literal["home", "pick_tag", "pick_skill", "manage", "tag_skill", "done"]


@dataclass(frozen=True)
class Skill:
    """One inventory row. identity = realpath (simulated as path string)."""

    path: str
    name: str
    description: str
    tags: frozenset[str] = frozenset()

    def invocation(self) -> str:
        return f"/{self.name}"


@dataclass(frozen=True)
class State:
    skills: tuple[Skill, ...]
    screen: Screen = "home"
    # pick flow
    selected_tag: str | None = None
    selected_skill_path: str | None = None
    last_copied: str | None = None
    # manage flow
    manage_focus_path: str | None = None
    tag_draft: str = ""
    message: str = ""
    # derived helpers stored only for display (recomputed by actions when needed)
    status_line: str = "seeded in-memory skills (not a real scan)"


def seed() -> State:
    """Fake local inventory — stand-ins for a multi-tool skill pile."""
    rows = [
        Skill(
            "/home/you/.agents/skills/grill-with-docs",
            "grill-with-docs",
            "Grill a plan and write ADRs/glossary as you go",
            frozenset({"design", "docs"}),
        ),
        Skill(
            "/home/you/.agents/skills/grilling",
            "grilling",
            "Relentless interview on a plan",
            frozenset({"design"}),
        ),
        Skill(
            "/home/you/.agents/skills/tdd",
            "tdd",
            "Test-driven development loop",
            frozenset({"engineering", "testing"}),
        ),
        Skill(
            "/home/you/.agents/skills/triage",
            "triage",
            "Triage issues with label vocabulary",
            frozenset({"github", "workflow"}),
        ),
        Skill(
            "/home/you/.agents/skills/research",
            "research",
            "Primary-source research into a markdown note",
            frozenset({"docs", "workflow"}),
        ),
        Skill(
            "/home/you/.agents/skills/domain-modeling",
            "domain-modeling",
            "Glossary + ADRs",
            frozenset({"design", "docs"}),
        ),
        Skill(
            "/home/you/.agents/skills/to-tickets",
            "to-tickets",
            "Break work into tickets",
            frozenset({"workflow"}),
        ),
        Skill(
            "/home/you/.agents/skills/prototype",
            "prototype",
            "Throwaway prototype to answer a design question",
            frozenset({"design", "engineering"}),
        ),
        Skill(
            "/home/you/.codex/skills/only-in-codex-demo",
            "only-in-codex-demo",
            "Simulates a skill only visible under a codex path",
            frozenset({"engineering"}),
        ),
        Skill(
            "/home/you/.agents/skills/untagged-orphan",
            "untagged-orphan",
            "No tags yet — forces the empty-tag case",
            frozenset(),
        ),
    ]
    return State(skills=tuple(rows))


# --- queries (pure) ---


def all_tags(state: State) -> tuple[str, ...]:
    tags: set[str] = set()
    for s in state.skills:
        tags |= set(s.tags)
    return tuple(sorted(tags))


def skills_for_tag(state: State, tag: str) -> tuple[Skill, ...]:
    return tuple(s for s in state.skills if tag in s.tags)


def untagged(state: State) -> tuple[Skill, ...]:
    return tuple(s for s in state.skills if not s.tags)


def skill_by_path(state: State, path: str) -> Skill | None:
    for s in state.skills:
        if s.path == path:
            return s
    return None


def public_view(state: State) -> dict:
    """Full relevant state for the TUI 'surface the state' rule."""
    return {
        "screen": state.screen,
        "selected_tag": state.selected_tag,
        "selected_skill_path": state.selected_skill_path,
        "last_copied": state.last_copied,
        "manage_focus_path": state.manage_focus_path,
        "tag_draft": state.tag_draft,
        "message": state.message,
        "status_line": state.status_line,
        "tags": list(all_tags(state)),
        "untagged_count": len(untagged(state)),
        "skills": [
            {
                "name": s.name,
                "path": s.path,
                "tags": sorted(s.tags),
                "invocation": s.invocation(),
            }
            for s in state.skills
        ],
    }


# --- actions (pure: State in → State out) ---


def go_home(state: State) -> State:
    return replace(
        state,
        screen="home",
        selected_tag=None,
        selected_skill_path=None,
        manage_focus_path=None,
        tag_draft="",
        message="",
    )


def start_pick(state: State) -> State:
    tags = all_tags(state)
    if not tags and not untagged(state):
        return replace(state, message="no skills at all")
    return replace(
        state,
        screen="pick_tag",
        selected_tag=None,
        selected_skill_path=None,
        message="pick a tag (or untagged)",
    )


def choose_tag(state: State, tag: str | None) -> State:
    """tag=None means the synthetic 'untagged' bucket."""
    if tag is None:
        bucket = untagged(state)
        label = "(untagged)"
    else:
        if tag not in all_tags(state):
            return replace(state, message=f"unknown tag: {tag}")
        bucket = skills_for_tag(state, tag)
        label = tag
    if not bucket:
        return replace(state, message=f"no skills for {label}")
    return replace(
        state,
        screen="pick_skill",
        selected_tag=tag if tag is not None else "",
        selected_skill_path=None,
        message=f"tag={label} → pick a skill",
    )


def choose_skill(state: State, path: str) -> State:
    sk = skill_by_path(state, path)
    if sk is None:
        return replace(state, message="unknown skill path")
    inv = sk.invocation()
    return replace(
        state,
        screen="done",
        selected_skill_path=path,
        last_copied=inv,
        message=f"copied invocation to clipboard buffer: {inv}",
    )


def start_manage(state: State) -> State:
    return replace(
        state,
        screen="manage",
        manage_focus_path=None,
        tag_draft="",
        message="select a skill to edit tags",
    )


def focus_skill_for_tags(state: State, path: str) -> State:
    if skill_by_path(state, path) is None:
        return replace(state, message="unknown skill")
    return replace(
        state,
        screen="tag_skill",
        manage_focus_path=path,
        tag_draft="",
        message="add/remove tags on this skill",
    )


def add_tag(state: State, tag: str) -> State:
    tag = tag.strip().lower().replace(" ", "-")
    if not tag or state.manage_focus_path is None:
        return replace(state, message="need focus skill + non-empty tag")
    path = state.manage_focus_path
    new_skills = []
    for s in state.skills:
        if s.path == path:
            new_skills.append(replace(s, tags=frozenset(set(s.tags) | {tag})))
        else:
            new_skills.append(s)
    return replace(
        state,
        skills=tuple(new_skills),
        tag_draft="",
        message=f"added tag '{tag}'",
    )


def remove_tag(state: State, tag: str) -> State:
    tag = tag.strip().lower()
    if state.manage_focus_path is None:
        return replace(state, message="no skill focused")
    path = state.manage_focus_path
    new_skills = []
    for s in state.skills:
        if s.path == path:
            new_skills.append(replace(s, tags=frozenset(set(s.tags) - {tag})))
        else:
            new_skills.append(s)
    return replace(
        state,
        skills=tuple(new_skills),
        message=f"removed tag '{tag}'",
    )


def set_tag_draft(state: State, draft: str) -> State:
    return replace(state, tag_draft=draft)
