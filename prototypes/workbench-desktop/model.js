/**
 * PROTOTYPE pure model — Windows icon grid + Coodesker boxes.
 * Icons snap to cells; no icon-icon overlap (overlap → auto simple box).
 * Icon on box → enter box. Box must not overlap desktop icons.
 */

export function uid(prefix = "id") {
  return `${prefix}_${Math.random().toString(36).slice(2, 9)}`;
}

/** Windows-like icon cell size */
export const ICON_GRID = {
  originX: 16,
  originY: 16,
  w: 90,
  h: 96,
  iconW: 86,
  iconH: 90,
};

function snap(n, g = 16) {
  return Math.round(n / g) * g;
}

export function snapIconCell(x, y) {
  const col = Math.max(0, Math.round((x - ICON_GRID.originX) / ICON_GRID.w));
  const row = Math.max(0, Math.round((y - ICON_GRID.originY) / ICON_GRID.h));
  return {
    col,
    row,
    x: ICON_GRID.originX + col * ICON_GRID.w,
    y: ICON_GRID.originY + row * ICON_GRID.h,
    key: `${col},${row}`,
  };
}

function iconRect(x, y) {
  return { x, y, w: ICON_GRID.iconW, h: ICON_GRID.iconH };
}

function boxRect(b) {
  return { x: b.x, y: b.y, w: b.w, h: b.h };
}

function rectsOverlap(a, b, pad = 2) {
  return (
    a.x + pad < b.x + b.w - pad &&
    a.x + a.w - pad > b.x + pad &&
    a.y + pad < b.y + b.h - pad &&
    a.y + a.h - pad > b.y + pad
  );
}

export function desktopIcons(state) {
  return state.placeholders.filter((p) => p.location.type === "desktop");
}

export function iconAtCell(state, col, row, excludePhIds = []) {
  const key = `${col},${row}`;
  const ex = new Set(excludePhIds);
  for (const p of desktopIcons(state)) {
    if (ex.has(p.id)) continue;
    const c = snapIconCell(p.location.x, p.location.y);
    if (c.key === key) return p;
  }
  // recycle icon occupies a cell when on desktop
  if (state.recycleIcon.location.type === "desktop") {
    const c = snapIconCell(state.recycleIcon.location.x, state.recycleIcon.location.y);
    if (c.key === key) return { id: state.recycleIcon.id, system: "recycle" };
  }
  return null;
}

export function nextSimpleBoxName(state) {
  const n = state.boxNameSeq || 1;
  return { name: `普通盒子${n}`, nextSeq: n + 1 };
}

export function seedState() {
  const skills = [
    { id: "sk_grill", name: "grill-with-docs", path: "/home/you/.agents/skills/grill-with-docs" },
    { id: "sk_tdd", name: "tdd", path: "/home/you/.agents/skills/tdd" },
    { id: "sk_triage", name: "triage", path: "/home/you/.agents/skills/triage" },
    { id: "sk_research", name: "research", path: "/home/you/.agents/skills/research" },
    { id: "sk_proto", name: "prototype", path: "/home/you/.agents/skills/prototype" },
    { id: "sk_domain", name: "domain-modeling", path: "/home/you/.agents/skills/domain-modeling" },
    { id: "sk_tickets", name: "to-tickets", path: "/home/you/.agents/skills/to-tickets" },
    { id: "sk_go1", name: "go-test", path: "/home/you/.agents/skills/go-test" },
    { id: "sk_go2", name: "go-mod", path: "/home/you/.agents/skills/go-mod" },
    { id: "sk_orphan", name: "untagged-orphan", path: "/home/you/.agents/skills/untagged-orphan" },
  ];

  // recycle = row0 col0; skills start from row1 same column
  const recycleCell = snapIconCell(ICON_GRID.originX, ICON_GRID.originY);
  const placeholders = skills.map((s, i) => {
    const cell = snapIconCell(
      ICON_GRID.originX,
      ICON_GRID.originY + (i + 1) * ICON_GRID.h
    );
    return {
      id: uid("ph"),
      skillId: s.id,
      location: { type: "desktop", x: cell.x, y: cell.y },
    };
  });

  // boxes start on free area (right of icon column)
  return {
    skills,
    placeholders,
    boxes: [
      {
        id: "box_design",
        kind: "simple",
        tag: "design",
        title: null,
        x: 320,
        y: 32,
        w: 240,
        h: 220,
        itemIds: [],
        compartments: null,
        activeCompartmentId: null,
      },
      {
        id: "box_eng",
        kind: "simple",
        tag: "engineering",
        title: null,
        x: 600,
        y: 32,
        w: 240,
        h: 220,
        itemIds: [],
        compartments: null,
        activeCompartmentId: null,
      },
    ],
    recycleIcon: {
      id: "sys_recycle",
      location: {
        type: "desktop",
        x: recycleCell.x,
        y: recycleCell.y,
      },
    },
    recycleBin: [],
    clipboard: null,
    multiSelect: false,
    selectedIds: [],
    message: "网格桌面：图标不重叠；叠图标→自动普通盒；图标入盒；盒子不压图标",
    lastAction: null,
    grid: 16,
    boxNameSeq: 1,
  };
}

export function skillById(state, id) {
  return state.skills.find((s) => s.id === id);
}

export function phById(state, id) {
  return state.placeholders.find((p) => p.id === id);
}

export function boxById(state, id) {
  return state.boxes.find((b) => b.id === id);
}

export function countPlaceholdersForSkill(state, skillId) {
  return state.placeholders.filter(
    (p) => p.skillId === skillId && p.location.type !== "recycle"
  ).length;
}

export function publicView(state) {
  return {
    message: state.message,
    clipboard: state.clipboard,
    multiSelect: state.multiSelect,
    selectedIds: state.selectedIds,
    desktopIcons: state.placeholders
      .filter((p) => p.location.type === "desktop")
      .map((p) => ({
        name: skillById(state, p.skillId)?.name,
        x: p.location.x,
        y: p.location.y,
      })),
    recycleIcon: state.recycleIcon,
    boxes: state.boxes.map((b) => {
      if (b.kind === "simple") {
        return {
          kind: "simple",
          tag: b.tag,
          pos: [b.x, b.y],
          items: b.itemIds.map((id) => skillById(state, phById(state, id)?.skillId)?.name),
          hasRecycle: recycleInBox(state, b.id, null),
        };
      }
      return {
        kind: "composite",
        title: b.title,
        compartments: b.compartments.map((c) => ({
          tag: c.tag,
          items: c.itemIds.map((id) => skillById(state, phById(state, id)?.skillId)?.name),
          hasRecycle: recycleInBox(state, b.id, c.id),
        })),
      };
    }),
    recycleBin: state.recycleBin,
  };
}

function recycleInBox(state, boxId, compartmentId) {
  const loc = state.recycleIcon.location;
  if (loc.type !== "box") return false;
  if (loc.boxId !== boxId) return false;
  if (compartmentId == null) return loc.compartmentId == null;
  return loc.compartmentId === compartmentId;
}

function clone(state) {
  return structuredClone(state);
}

function setMsg(state, message, lastAction = null) {
  state.message = message;
  state.lastAction = lastAction;
  return state;
}

function removePhFromAllContainers(state, phId) {
  for (const b of state.boxes) {
    if (b.kind === "simple") {
      b.itemIds = b.itemIds.filter((id) => id !== phId);
    } else if (b.compartments) {
      for (const c of b.compartments) {
        c.itemIds = c.itemIds.filter((id) => id !== phId);
      }
    }
  }
}

function ensureUniqueTag(tags, tag) {
  let t = tag;
  let n = 2;
  const used = new Set(tags);
  while (used.has(t)) t = `${tag}-${n++}`;
  return t;
}

function namesFor(state, phIds) {
  return phIds
    .map((id) => skillById(state, phById(state, id)?.skillId)?.name)
    .filter(Boolean);
}

/** If composite has exactly 1 compartment → demote to simple box. */
export function demoteCompositeIfSingle(state, boxId) {
  const box = boxById(state, boxId);
  if (!box || box.kind !== "composite") return state;
  if (box.compartments.length !== 1) return state;
  const last = box.compartments[0];
  box.kind = "simple";
  box.tag = last.tag;
  box.title = null;
  box.itemIds = [...last.itemIds];
  box.compartments = null;
  box.activeCompartmentId = null;
  for (const phId of box.itemIds) {
    const ph = phById(state, phId);
    if (ph) ph.location = { type: "box", boxId: box.id, compartmentId: null };
  }
  // recycle icon in that compartment
  const r = state.recycleIcon.location;
  if (r.type === "box" && r.boxId === boxId) {
    state.recycleIcon.location = { type: "box", boxId: box.id, compartmentId: null };
  }
  state.message = `组合盒仅剩一标签 → 已退化为普通盒 #${box.tag}`;
  return state;
}

/** Find nearest free icon cell near (x,y), excluding some ph ids. */
export function findFreeIconCell(state, x, y, excludePhIds = []) {
  const start = snapIconCell(x, y);
  if (!iconAtCell(state, start.col, start.row, excludePhIds)) return start;
  for (let r = 0; r < 40; r++) {
    for (let dc = -r; dc <= r; dc++) {
      for (let dr = -r; dr <= r; dr++) {
        if (Math.max(Math.abs(dc), Math.abs(dr)) !== r) continue;
        const col = Math.max(0, start.col + dc);
        const row = Math.max(0, start.row + dr);
        if (!iconAtCell(state, col, row, excludePhIds)) {
          return {
            col,
            row,
            x: ICON_GRID.originX + col * ICON_GRID.w,
            y: ICON_GRID.originY + row * ICON_GRID.h,
            key: `${col},${row}`,
          };
        }
      }
    }
  }
  return start;
}

/**
 * Drop skill icon(s) on desktop at point.
 * - If cell has another skill icon → auto simple box with both (name 普通盒子N)
 * - If cell has recycle → treat as free nearby (don't merge with recycle)
 * - Else snap to free cell
 */
export function dropIconsOnDesktop(state, phIds, x, y) {
  state = clone(state);
  const ids = phIds.filter((id) => {
    const ph = phById(state, id);
    return ph && ph.location.type !== "recycle";
  });
  if (!ids.length) return setMsg(state, "nothing to drop");

  const cell = snapIconCell(x, y);
  const occupant = iconAtCell(state, cell.col, cell.row, ids);

  // overlap another skill icon → auto create simple box
  if (occupant && !occupant.system) {
    const allIds = [...new Set([occupant.id, ...ids])];
    return mergeIconsIntoAutoBox(state, allIds, cell.x, cell.y);
  }

  // park movers so they don't block free-cell search
  for (const phId of ids) {
    const ph = phById(state, phId);
    removePhFromAllContainers(state, phId);
    ph.location = { type: "desktop", x: -10000, y: -10000 };
  }
  let cx = x;
  let cy = y;
  for (const phId of ids) {
    const ph = phById(state, phId);
    const free = findFreeIconCell(state, cx, cy, []);
    ph.location = { type: "desktop", x: free.x, y: free.y };
    cx = free.x + ICON_GRID.w;
    cy = free.y;
  }
  state.selectedIds = [];
  return setMsg(state, `已放到网格: ${namesFor(state, ids).join(", ")}`);
}

/** Create simple box 普通盒子N containing phIds at position. */
export function mergeIconsIntoAutoBox(state, phIds, x, y) {
  // state may already be clone
  if (!state.boxNameSeq) state.boxNameSeq = 1;
  const { name, nextSeq } = nextSimpleBoxName(state);
  state.boxNameSeq = nextSeq;

  const ids = [...new Set(phIds)];
  for (const phId of ids) {
    removePhFromAllContainers(state, phId);
  }

  // box should not cover other desktop icons
  const w = 240;
  const h = 220;
  const pos = findBoxPosWithoutIconOverlap(state, x, y, w, h, null);

  const box = {
    id: uid("box"),
    kind: "simple",
    tag: name,
    title: null,
    x: pos.x,
    y: pos.y,
    w,
    h,
    itemIds: [],
    compartments: null,
    activeCompartmentId: null,
  };
  state.boxes.push(box);

  for (const phId of ids) {
    const ph = phById(state, phId);
    if (!ph) continue;
    ph.location = { type: "box", boxId: box.id, compartmentId: null };
    box.itemIds.push(phId);
  }
  state.selectedIds = [];
  return setMsg(
    state,
    `图标重叠 → 自动创建「${name}」: ${namesFor(state, ids).join(", ")}（双击标签可改名）`,
    { type: "auto-box", skillNames: namesFor(state, ids), where: name }
  );
}

export function movePlaceholdersToDesktop(state, phIds, x, y) {
  return dropIconsOnDesktop(state, phIds, x, y);
}

export function movePlaceholderToDesktopPos(state, phId, x, y) {
  return dropIconsOnDesktop(state, [phId], x, y);
}

/** Box placement: snap soft grid, must not overlap desktop skill icons (or recycle). */
export function findBoxPosWithoutIconOverlap(state, x, y, w, h, excludeBoxId) {
  const g = state.grid || 16;
  let px = snap(x, g);
  let py = snap(y, g);
  const tryPos = (tx, ty) => {
    const rect = { x: tx, y: ty, w, h };
    for (const p of desktopIcons(state)) {
      if (rectsOverlap(rect, iconRect(p.location.x, p.location.y))) return false;
    }
    if (state.recycleIcon.location.type === "desktop") {
      const r = state.recycleIcon.location;
      if (rectsOverlap(rect, iconRect(r.x, r.y))) return false;
    }
    for (const b of state.boxes) {
      if (excludeBoxId && b.id === excludeBoxId) continue;
      // boxes may stack partially for prototype — only forbid icon overlap per user
    }
    return true;
  };
  if (tryPos(px, py)) return { x: px, y: py };
  for (let step = g; step < 800; step += g) {
    for (const [dx, dy] of [
      [step, 0],
      [-step, 0],
      [0, step],
      [0, -step],
      [step, step],
      [-step, step],
      [step, -step],
      [-step, -step],
    ]) {
      const tx = Math.max(0, px + dx);
      const ty = Math.max(0, py + dy);
      if (tryPos(tx, ty)) return { x: tx, y: ty };
    }
  }
  return { x: px, y: py };
}

export function movePlaceholdersToBox(state, phIds, boxId, compartmentId) {
  state = clone(state);
  const box = boxById(state, boxId);
  if (!box) return setMsg(state, "unknown box");

  let loc;
  let where;
  let compartmentIdResolved = null;
  if (box.kind === "simple") {
    loc = { type: "box", boxId, compartmentId: null };
    where = `#${box.tag}`;
  } else {
    const cid = compartmentId || box.activeCompartmentId;
    const c = box.compartments.find((x) => x.id === cid);
    if (!c) return setMsg(state, "no compartment");
    compartmentIdResolved = cid;
    loc = { type: "box", boxId, compartmentId: cid };
    where = `「${box.title}」/${c.tag}`;
  }

  const moved = [];
  for (const phId of phIds) {
    const ph = phById(state, phId);
    if (!ph || ph.location.type === "recycle") continue;
    removePhFromAllContainers(state, phId);
    const b2 = boxById(state, boxId);
    const list =
      b2.kind === "simple"
        ? b2.itemIds
        : b2.compartments.find((x) => x.id === compartmentIdResolved).itemIds;
    ph.location = { ...loc };
    if (!list.includes(phId)) list.push(phId);
    moved.push(phId);
  }
  state.selectedIds = [];
  const names = namesFor(state, moved);
  return setMsg(state, `已放入 ${where}: ${names.join(", ")}`, {
    type: "drop-into-box",
    where,
    skillNames: names,
  });
}

export function moveRecycleIcon(state, location) {
  state = clone(state);
  state.recycleIcon.location = location;
  return setMsg(state, "已移动回收站图标");
}

export function moveRecycleToDesktop(state, x, y) {
  state = clone(state);
  const free = findFreeIconCell(state, x, y, []);
  // free cell might still be wrong if we consider recycle not excluding self — exclude recycle
  const cell = snapIconCell(x, y);
  const occ = iconAtCell(state, cell.col, cell.row, []);
  let pos = free;
  if (occ && occ.id === state.recycleIcon.id) {
    pos = cell;
  } else if (occ && !occ.system) {
    // don't merge recycle with skill — find free
    pos = findFreeIconCell(state, x + ICON_GRID.w, y, []);
  }
  state.recycleIcon.location = { type: "desktop", x: pos.x, y: pos.y };
  return setMsg(state, "回收站已放到网格");
}

export function moveRecycleToBox(state, boxId, compartmentId) {
  state = clone(state);
  const box = boxById(state, boxId);
  if (!box) return setMsg(state, "unknown box");
  if (box.kind === "simple") {
    state.recycleIcon.location = { type: "box", boxId, compartmentId: null };
  } else {
    const cid = compartmentId || box.activeCompartmentId;
    state.recycleIcon.location = { type: "box", boxId, compartmentId: cid };
  }
  return setMsg(state, "回收站已放入盒子（系统图标）");
}

export function copyPlaceholder(state, phId) {
  state = clone(state);
  const ph = phById(state, phId);
  if (!ph || ph.location.type === "recycle") return setMsg(state, "cannot copy");

  const copy = {
    id: uid("ph"),
    skillId: ph.skillId,
    location: structuredClone(ph.location),
  };

  if (ph.location.type === "desktop") {
    copy.location.x = snap(ph.location.x + 20, state.grid);
    copy.location.y = snap(ph.location.y + 20, state.grid);
    state.placeholders.push(copy);
  } else if (ph.location.type === "box") {
    state.placeholders.push(copy);
    const box = boxById(state, ph.location.boxId);
    if (box.kind === "simple") box.itemIds.push(copy.id);
    else {
      const c = box.compartments.find((x) => x.id === ph.location.compartmentId);
      c.itemIds.push(copy.id);
    }
  }
  return setMsg(state, `已复制: ${skillById(state, ph.skillId)?.name}`);
}

export function setClipboard(state, mode, phIds) {
  state = clone(state);
  // filter invalid
  const ids = phIds.filter((id) => {
    const ph = phById(state, id);
    return ph && ph.location.type !== "recycle";
  });
  if (!ids.length) return setMsg(state, "剪贴板为空");
  state.clipboard = { mode, phIds: ids };
  return setMsg(state, mode === "cut" ? `已剪切 ${ids.length} 项` : `已复制 ${ids.length} 项到剪贴板`);
}

export function pasteClipboard(state, target) {
  // target: { type:'desktop', x, y } | { type:'box', boxId, compartmentId }
  state = clone(state);
  const cb = state.clipboard;
  if (!cb?.phIds?.length) return setMsg(state, "剪贴板为空");

  if (cb.mode === "copy") {
    const newIds = [];
    for (const phId of cb.phIds) {
      const ph = phById(state, phId);
      if (!ph) continue;
      const copy = { id: uid("ph"), skillId: ph.skillId, location: null };
      if (target.type === "desktop") {
        copy.location = {
          type: "desktop",
          x: snap(target.x + newIds.length * 16, state.grid),
          y: snap(target.y + newIds.length * 16, state.grid),
        };
        state.placeholders.push(copy);
      } else {
        copy.location = {
          type: "box",
          boxId: target.boxId,
          compartmentId: target.compartmentId ?? null,
        };
        state.placeholders.push(copy);
        const box = boxById(state, target.boxId);
        if (box.kind === "simple") box.itemIds.push(copy.id);
        else {
          const c = box.compartments.find((x) => x.id === target.compartmentId);
          if (c) c.itemIds.push(copy.id);
        }
      }
      newIds.push(copy.id);
    }
    return setMsg(state, `已粘贴(复制) ${newIds.length} 项`);
  }

  // cut = move
  if (target.type === "desktop") {
    state = movePlaceholdersToDesktop(state, cb.phIds, target.x, target.y);
  } else {
    state = movePlaceholdersToBox(state, cb.phIds, target.boxId, target.compartmentId);
  }
  state.clipboard = null;
  return setMsg(state, "已粘贴(剪切)");
}

export function createSimpleBox(state, tag, x, y) {
  state = clone(state);
  const w = 240;
  const h = 220;
  const pos = findBoxPosWithoutIconOverlap(state, x ?? 200, y ?? 200, w, h, null);
  state.boxes.push({
    id: uid("box"),
    kind: "simple",
    tag: tag || "新建",
    title: null,
    x: pos.x,
    y: pos.y,
    w,
    h,
    itemIds: [],
    compartments: null,
    activeCompartmentId: null,
  });
  return setMsg(state, `新建普通盒 #${tag}`);
}

export function createCompositeBox(state, title, tags, x, y) {
  state = clone(state);
  const compartments = [];
  for (const t of tags?.length ? tags : ["默认"]) {
    const tag = ensureUniqueTag(
      compartments.map((c) => c.tag),
      t
    );
    compartments.push({ id: uid("cmp"), tag, itemIds: [] });
  }
  const w = 280;
  const h = 260;
  const pos = findBoxPosWithoutIconOverlap(state, x ?? 200, y ?? 200, w, h, null);
  const box = {
    id: uid("box"),
    kind: "composite",
    tag: null,
    title: title || "组合盒",
    x: pos.x,
    y: pos.y,
    w,
    h,
    itemIds: [],
    compartments,
    activeCompartmentId: compartments[0].id,
  };
  // if only one compartment created, demote to simple immediately
  if (compartments.length === 1) {
    box.kind = "simple";
    box.tag = compartments[0].tag;
    box.title = null;
    box.itemIds = [];
    box.compartments = null;
    box.activeCompartmentId = null;
  }
  state.boxes.push(box);
  return setMsg(
    state,
    box.kind === "simple"
      ? `仅一标签 → 已建普通盒 #${box.tag}`
      : `新建组合盒 「${box.title}」`
  );
}

export function composeSimpleIntoSimple(state, sourceBoxId, targetBoxId) {
  state = clone(state);
  if (sourceBoxId === targetBoxId) return setMsg(state, "same box");
  const src = boxById(state, sourceBoxId);
  const tgt = boxById(state, targetBoxId);
  if (!src || !tgt || src.kind !== "simple" || tgt.kind !== "simple") {
    return setMsg(state, "compose needs simple → simple");
  }

  const c1 = { id: uid("cmp"), tag: tgt.tag, itemIds: [...tgt.itemIds] };
  const c2 = {
    id: uid("cmp"),
    tag: ensureUniqueTag([c1.tag], src.tag),
    itemIds: [...src.itemIds],
  };

  for (const phId of c1.itemIds) {
    const ph = phById(state, phId);
    if (ph) ph.location = { type: "box", boxId: tgt.id, compartmentId: c1.id };
  }
  for (const phId of c2.itemIds) {
    const ph = phById(state, phId);
    if (ph) ph.location = { type: "box", boxId: tgt.id, compartmentId: c2.id };
  }

  // recycle in src or tgt
  const r = state.recycleIcon.location;
  if (r.type === "box" && (r.boxId === src.id || r.boxId === tgt.id)) {
    // keep in first compartment of new composite if was in either
    state.recycleIcon.location = {
      type: "box",
      boxId: tgt.id,
      compartmentId: c1.id,
    };
  }

  tgt.kind = "composite";
  tgt.title = tgt.tag;
  tgt.tag = null;
  tgt.itemIds = [];
  tgt.compartments = [c1, c2];
  tgt.activeCompartmentId = c1.id;
  tgt.w = 280;
  tgt.h = 260;

  state.boxes = state.boxes.filter((b) => b.id !== src.id);
  return setMsg(state, `组盒 「${tgt.title}」`);
}

export function addSimpleToComposite(state, sourceBoxId, compositeBoxId) {
  state = clone(state);
  const src = boxById(state, sourceBoxId);
  const tgt = boxById(state, compositeBoxId);
  if (!src || !tgt || src.kind !== "simple" || tgt.kind !== "composite") {
    return setMsg(state, "need simple → composite");
  }
  const tag = ensureUniqueTag(
    tgt.compartments.map((c) => c.tag),
    src.tag
  );
  const c = { id: uid("cmp"), tag, itemIds: [...src.itemIds] };
  for (const phId of c.itemIds) {
    const ph = phById(state, phId);
    if (ph) ph.location = { type: "box", boxId: tgt.id, compartmentId: c.id };
  }
  const r = state.recycleIcon.location;
  if (r.type === "box" && r.boxId === src.id) {
    state.recycleIcon.location = {
      type: "box",
      boxId: tgt.id,
      compartmentId: c.id,
    };
  }
  tgt.compartments.push(c);
  tgt.activeCompartmentId = c.id;
  state.boxes = state.boxes.filter((b) => b.id !== src.id);
  return setMsg(state, `追加隔间 「${tag}」`);
}

export function ejectCompartment(state, compositeBoxId, compartmentId, x, y) {
  state = clone(state);
  const box = boxById(state, compositeBoxId);
  if (!box || box.kind !== "composite") return setMsg(state, "not composite");
  const idx = box.compartments.findIndex((c) => c.id === compartmentId);
  if (idx < 0) return setMsg(state, "no compartment");

  const [comp] = box.compartments.splice(idx, 1);
  const newBox = {
    id: uid("box"),
    kind: "simple",
    tag: comp.tag,
    title: null,
    x: snap(x ?? box.x + 32, state.grid),
    y: snap(y ?? box.y + 32, state.grid),
    w: 240,
    h: 220,
    itemIds: [...comp.itemIds],
    compartments: null,
    activeCompartmentId: null,
  };
  for (const phId of newBox.itemIds) {
    const ph = phById(state, phId);
    if (ph) ph.location = { type: "box", boxId: newBox.id, compartmentId: null };
  }
  const r = state.recycleIcon.location;
  if (r.type === "box" && r.boxId === compositeBoxId && r.compartmentId === compartmentId) {
    state.recycleIcon.location = {
      type: "box",
      boxId: newBox.id,
      compartmentId: null,
    };
  }
  state.boxes.push(newBox);

  if (box.compartments.length === 1) {
    demoteCompositeIfSingle(state, box.id);
    return setMsg(state, `拖出 #${comp.tag}；原盒已退化为普通盒`);
  }
  if (box.compartments.length === 0) {
    state.boxes = state.boxes.filter((b) => b.id !== box.id);
    return setMsg(state, `拖出 #${comp.tag}`);
  }
  if (box.activeCompartmentId === compartmentId) {
    box.activeCompartmentId = box.compartments[0].id;
  }
  return setMsg(state, `隔间 #${comp.tag} 已拖出为普通盒`);
}

export function setActiveCompartment(state, boxId, compartmentId) {
  state = clone(state);
  const box = boxById(state, boxId);
  if (!box || box.kind !== "composite") return state;
  box.activeCompartmentId = compartmentId;
  return state;
}

export function moveBox(state, boxId, x, y) {
  state = clone(state);
  const box = boxById(state, boxId);
  if (!box) return state;
  const pos = findBoxPosWithoutIconOverlap(state, x, y, box.w, box.h, boxId);
  if (pos.x !== snap(x, state.grid || 16) || pos.y !== snap(y, state.grid || 16)) {
    box.x = pos.x;
    box.y = pos.y;
    return setMsg(state, "盒子已避开桌面图标（不可与图标重叠）");
  }
  box.x = pos.x;
  box.y = pos.y;
  return state;
}

export function deleteBox(state, boxId) {
  state = clone(state);
  const box = boxById(state, boxId);
  if (!box) return state;

  const ids = [];
  if (box.kind === "simple") ids.push(...box.itemIds);
  else for (const c of box.compartments) ids.push(...c.itemIds);

  if (box.kind === "simple") box.itemIds = [];
  else for (const c of box.compartments) c.itemIds = [];

  let x = box.x;
  let y = box.y + 40;
  const names = [];
  for (const phId of ids) {
    const ph = phById(state, phId);
    if (!ph) continue;
    ph.location = { type: "desktop", x: snap(x, state.grid), y: snap(y, state.grid) };
    names.push(skillById(state, ph.skillId)?.name);
    x += 20;
    y += 20;
  }

  // recycle icon out of box → desktop near box
  const r = state.recycleIcon.location;
  if (r.type === "box" && r.boxId === boxId) {
    state.recycleIcon.location = {
      type: "desktop",
      x: snap(box.x + 40, state.grid),
      y: snap(box.y + 40, state.grid),
    };
  }

  state.boxes = state.boxes.filter((b) => b.id !== boxId);
  return setMsg(state, `已删盒；图标回桌面: ${names.filter(Boolean).join(", ") || "(空)"}`, {
    type: "delete-box",
    skillNames: names.filter(Boolean),
  });
}

export function confirmTrash(state, phIds) {
  state = clone(state);
  const now = Date.now();
  const purgeAfter = now + 30 * 24 * 3600 * 1000;
  const iconOnly = [];
  const bodyTrash = [];

  for (const phId of phIds) {
    const ph = phById(state, phId);
    if (!ph || ph.location.type === "recycle") continue;
    const sk = skillById(state, ph.skillId);
    const liveCount = countPlaceholdersForSkill(state, ph.skillId);
    removePhFromAllContainers(state, phId);

    if (liveCount > 1) {
      state.placeholders = state.placeholders.filter((p) => p.id !== phId);
      iconOnly.push(sk?.name);
    } else {
      ph.location = { type: "recycle" };
      state.recycleBin.push({
        placeholderId: phId,
        skillId: ph.skillId,
        name: sk?.name,
        path: sk?.path,
        originalPath: sk?.path,
        quarantinedAs: `${(sk?.path || "").replace(/\/[^/]+$/, "")}/.skills-manage-trash/${ph.skillId}`,
        purgeAfter,
        note: "last placeholder → body quarantine",
      });
      bodyTrash.push(sk?.name);
    }
  }
  // clear cut items if trashed
  if (state.clipboard) {
    state.clipboard.phIds = state.clipboard.phIds.filter((id) => phById(state, id));
    if (!state.clipboard.phIds.length) state.clipboard = null;
  }
  state.selectedIds = [];
  const parts = [];
  if (iconOnly.length) parts.push(`仅删图标: ${iconOnly.join(", ")}`);
  if (bodyTrash.length) parts.push(`本体进回收站: ${bodyTrash.join(", ")}`);
  return setMsg(state, parts.join(" · ") || "nothing", { type: "trash", iconOnly, bodyTrash });
}

export function restoreFromRecycle(state, skillId, x = 40, y = 40) {
  state = clone(state);
  const entry = state.recycleBin.find((e) => e.skillId === skillId);
  if (!entry) return setMsg(state, "not in bin");
  state.recycleBin = state.recycleBin.filter((e) => e.skillId !== skillId);
  let ph = phById(state, entry.placeholderId);
  if (!ph) {
    ph = {
      id: entry.placeholderId || uid("ph"),
      skillId: entry.skillId,
      location: { type: "desktop", x: snap(x, state.grid), y: snap(y, state.grid) },
    };
    state.placeholders.push(ph);
  } else {
    ph.location = { type: "desktop", x: snap(x, state.grid), y: snap(y, state.grid) };
  }
  return setMsg(state, `已还原 ${entry.name} → 桌面`);
}

export function emptyRecycleBin(state) {
  state = clone(state);
  const names = state.recycleBin.map((e) => e.name);
  for (const e of state.recycleBin) {
    state.placeholders = state.placeholders.filter((p) => p.id !== e.placeholderId);
    state.skills = state.skills.filter((s) => s.id !== e.skillId);
  }
  state.recycleBin = [];
  return setMsg(state, `已清空回收站(真删模拟): ${names.join(", ") || "(empty)"}`);
}

export function toggleMultiSelect(state) {
  state = clone(state);
  state.multiSelect = !state.multiSelect;
  if (!state.multiSelect) state.selectedIds = [];
  return setMsg(state, state.multiSelect ? "多选 ON" : "多选 OFF");
}

export function toggleSelected(state, phId) {
  state = clone(state);
  if (!state.multiSelect) return state;
  if (state.selectedIds.includes(phId)) {
    state.selectedIds = state.selectedIds.filter((id) => id !== phId);
  } else state.selectedIds = [...state.selectedIds, phId];
  return state;
}

export function renameCompositeTitle(state, boxId, title) {
  state = clone(state);
  const box = boxById(state, boxId);
  if (!box || box.kind !== "composite") return state;
  box.title = title;
  return setMsg(state, `标题 → ${title}`);
}

/** Rename simple box tag or composite compartment tag. */
export function renameBoxTag(state, boxId, newTag, compartmentId = null) {
  state = clone(state);
  const box = boxById(state, boxId);
  if (!box) return setMsg(state, "unknown box");
  const tag = (newTag || "").trim();
  if (!tag) return setMsg(state, "标签不能为空");

  if (box.kind === "simple") {
    box.tag = tag;
    return setMsg(state, `普通盒标签 → #${tag}`);
  }

  if (!compartmentId) return setMsg(state, "need compartment");
  const c = box.compartments.find((x) => x.id === compartmentId);
  if (!c) return setMsg(state, "no compartment");
  const others = box.compartments.filter((x) => x.id !== compartmentId).map((x) => x.tag);
  c.tag = ensureUniqueTag(others, tag);
  return setMsg(state, `隔间标签 → #${c.tag}`);
}

/** Enter multi-select with current ph pre-selected. */
export function enableMultiSelectWith(state, phId) {
  state = clone(state);
  state.multiSelect = true;
  state.selectedIds = phId ? [phId] : [];
  return setMsg(state, "多选 ON（已选当前）");
}

export function disableMultiSelect(state) {
  state = clone(state);
  state.multiSelect = false;
  state.selectedIds = [];
  return setMsg(state, "多选 OFF");
}
