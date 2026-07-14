#!/usr/bin/env python3
"""PROTOTYPE throwaway TUI shell over model.py — not production."""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

# allow `python prototypes/tag-pick-flow/app.py` from repo root or this dir
sys.path.insert(0, str(Path(__file__).resolve().parent))

from dataclasses import replace

from model import (  # noqa: E402
    add_tag,
    all_tags,
    choose_skill,
    choose_tag,
    focus_skill_for_tags,
    go_home,
    public_view,
    remove_tag,
    seed,
    skills_for_tag,
    skill_by_path,
    start_manage,
    start_pick,
    untagged,
)

BOLD = "\x1b[1m"
DIM = "\x1b[2m"
RESET = "\x1b[0m"
CLEAR = "\x1b[2J\x1b[H"


def try_clipboard(text: str) -> str:
    """Best-effort real clipboard; always keep in-state buffer via model."""
    for cmd in (
        ["wl-copy"],
        ["xclip", "-selection", "clipboard"],
        ["xsel", "--clipboard", "--input"],
        ["pbcopy"],
    ):
        if shutil.which(cmd[0]):
            try:
                subprocess.run(cmd, input=text.encode(), check=True)
                return f"os-clipboard via {cmd[0]}"
            except Exception:
                continue
    # OSC 52 (works in some terminals)
    try:
        import base64

        b64 = base64.b64encode(text.encode()).decode()
        sys.stdout.write(f"\x1b]52;c;{b64}\x07")
        sys.stdout.flush()
        return "osc52 (if terminal allows)"
    except Exception:
        return "in-memory only (no clipboard tool)"


def render(state: State) -> None:
    sys.stdout.write(CLEAR)
    print(f"{BOLD}PROTOTYPE{RESET} {DIM}tag-pick-flow — throwaway{RESET}")
    print(f"{DIM}Q: tag manage + pick tag→skill→clipboard /name — feel right?{RESET}")
    print()

    print(f"{BOLD}state{RESET}")
    view = public_view(state)
    # compact state surface
    print(f"  screen:           {view['screen']}")
    print(f"  selected_tag:     {view['selected_tag']!r}")
    print(f"  selected_skill:   {view['selected_skill_path']!r}")
    print(f"  last_copied:      {BOLD}{view['last_copied']!r}{RESET}")
    print(f"  manage_focus:     {view['manage_focus_path']!r}")
    print(f"  tags:             {view['tags']}")
    print(f"  untagged_count:   {view['untagged_count']}")
    if view["message"]:
        print(f"  message:          {BOLD}{view['message']}{RESET}")
    print(f"  {DIM}{view['status_line']}{RESET}")
    print()

    screen = state.screen
    if screen == "home":
        print(f"{BOLD}home{RESET}")
        print(f"  {len(state.skills)} skills in memory inventory")
        print()
        print(f"  {BOLD}[p]{RESET} {DIM}pick skill (tag → skill → copy){RESET}")
        print(f"  {BOLD}[m]{RESET} {DIM}manage tags{RESET}")
        print(f"  {BOLD}[j]{RESET} {DIM}dump full state JSON{RESET}")
        print(f"  {BOLD}[q]{RESET} {DIM}quit{RESET}")

    elif screen == "pick_tag":
        print(f"{BOLD}pick tag{RESET} {DIM}(then skills under it){RESET}")
        tags = list(all_tags(state))
        for i, t in enumerate(tags, 1):
            n = len(skills_for_tag(state, t))
            print(f"  {BOLD}[{i}]{RESET} {t}  {DIM}({n} skills){RESET}")
        u = untagged(state)
        print(f"  {BOLD}[u]{RESET} (untagged)  {DIM}({len(u)} skills){RESET}")
        print(f"  {BOLD}[b]{RESET} {DIM}back home{RESET}")

    elif screen == "pick_skill":
        tag = state.selected_tag
        if tag == "":
            bucket = untagged(state)
            label = "(untagged)"
        else:
            bucket = skills_for_tag(state, tag or "")
            label = tag
        print(f"{BOLD}pick skill{RESET} under {BOLD}{label}{RESET}")
        for i, s in enumerate(bucket, 1):
            print(f"  {BOLD}[{i}]{RESET} /{s.name}")
            print(f"      {DIM}{s.description}{RESET}")
            print(f"      {DIM}{s.path}{RESET}")
        print(f"  {BOLD}[b]{RESET} {DIM}back to tags{RESET}")

    elif screen == "done":
        sk = skill_by_path(state, state.selected_skill_path or "")
        print(f"{BOLD}done — paste into your coding CLI{RESET}")
        print(f"  invocation: {BOLD}{state.last_copied}{RESET}")
        if sk:
            print(f"  skill:      {sk.name}")
            print(f"  path:       {DIM}{sk.path}{RESET}")
            print(f"  tags:       {sorted(sk.tags)}")
        print()
        print(f"  {BOLD}[p]{RESET} {DIM}pick again{RESET}")
        print(f"  {BOLD}[h]{RESET} {DIM}home{RESET}")
        print(f"  {BOLD}[q]{RESET} {DIM}quit{RESET}")

    elif screen == "manage":
        print(f"{BOLD}manage tags{RESET} {DIM}select skill{RESET}")
        for i, s in enumerate(state.skills, 1):
            t = ",".join(sorted(s.tags)) or "(none)"
            print(f"  {BOLD}[{i}]{RESET} {s.name}  {DIM}tags=[{t}]{RESET}")
        print(f"  {BOLD}[b]{RESET} {DIM}back home{RESET}")

    elif screen == "tag_skill":
        sk = skill_by_path(state, state.manage_focus_path or "")
        print(f"{BOLD}edit tags{RESET}")
        if sk:
            print(f"  skill: {sk.name}")
            print(f"  path:  {DIM}{sk.path}{RESET}")
            print(f"  tags:  {sorted(sk.tags)}")
            for i, t in enumerate(sorted(sk.tags), 1):
                print(f"    {BOLD}[r{i}]{RESET} {DIM}remove {t}{RESET}  (type r{i})")
        print()
        print(f"  {BOLD}a <tag>{RESET} {DIM}add tag e.g.  a testing{RESET}")
        print(f"  {BOLD}[b]{RESET} {DIM}back to skill list{RESET}")

    print()


def read_line(prompt: str = "> ") -> str:
    try:
        return input(prompt).strip()
    except EOFError:
        return "q"


def main() -> None:
    state = seed()
    dump_once = False

    while True:
        render(state)
        if dump_once:
            print(f"{DIM}{json.dumps(public_view(state), indent=2)}{RESET}")
            dump_once = False

        raw = read_line()
        if not raw:
            continue
        low = raw.lower()
        screen = state.screen

        if low == "q":
            print("bye (prototype — nothing persisted)")
            return

        if low == "j":
            dump_once = True
            continue

        if screen == "home":
            if low == "p":
                state = start_pick(state)
            elif low == "m":
                state = start_manage(state)
            else:
                state = replace(state, message=f"unknown key: {raw}")

        elif screen == "pick_tag":
            if low == "b":
                state = go_home(state)
            elif low == "u":
                state = choose_tag(state, None)
            elif raw.isdigit():
                tags = list(all_tags(state))
                idx = int(raw) - 1
                if 0 <= idx < len(tags):
                    state = choose_tag(state, tags[idx])
                else:
                    state = replace(state, message="index out of range")
            else:
                state = replace(state, message=f"unknown: {raw}")

        elif screen == "pick_skill":
            if low == "b":
                state = start_pick(state)
            elif raw.isdigit():
                tag = state.selected_tag
                if tag == "":
                    bucket = untagged(state)
                else:
                    bucket = skills_for_tag(state, tag or "")
                idx = int(raw) - 1
                if 0 <= idx < len(bucket):
                    state = choose_skill(state, bucket[idx].path)
                    if state.last_copied:
                        how = try_clipboard(state.last_copied)
                        state = replace(
                            state,
                            status_line=f"clipboard: {how}",
                            message=f"copied {state.last_copied!r} — paste into Codex/Claude/Grok",
                        )
                else:
                    state = replace(state, message="index out of range")
            else:
                state = replace(state, message=f"unknown: {raw}")

        elif screen == "done":
            if low == "p":
                state = start_pick(state)
            elif low == "h":
                state = go_home(state)
            else:
                state = replace(state, message=f"unknown: {raw}")

        elif screen == "manage":
            if low == "b":
                state = go_home(state)
            elif raw.isdigit():
                idx = int(raw) - 1
                if 0 <= idx < len(state.skills):
                    state = focus_skill_for_tags(state, state.skills[idx].path)
                else:
                    state = replace(state, message="index out of range")
            else:
                state = replace(state, message=f"unknown: {raw}")

        elif screen == "tag_skill":
            if low == "b":
                state = start_manage(state)
            elif low.startswith("a "):
                tag = raw.split(None, 1)[1]
                state = add_tag(state, tag)
            elif low.startswith("r") and low[1:].isdigit():
                sk = skill_by_path(state, state.manage_focus_path or "")
                if sk:
                    tags = sorted(sk.tags)
                    idx = int(low[1:]) - 1
                    if 0 <= idx < len(tags):
                        state = remove_tag(state, tags[idx])
                    else:
                        state = replace(state, message="bad remove index")
            else:
                state = replace(
                    state,
                    message="use: a <tag>  |  r1 / r2 …  |  b",
                )


if __name__ == "__main__":
    # nicer on broken pipes
    try:
        main()
    except KeyboardInterrupt:
        print("\nbye")
        sys.exit(0)
