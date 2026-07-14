/**
 * PROTOTYPE UI — free desktop (Windows shortcuts + Coodesker boxes + system recycle).
 */
import * as M from "./model.js";

let state = M.seedState();
let pendingTrashIds = null;
let dragPayload = null;
let ctxMenu = null;
let pointerDrag = null; // free move: { kind, id, startX, startY, origX, origY, moved }

const $ = (id) => document.getElementById(id);

function deskPoint(clientX, clientY) {
  const desk = $("desktop");
  const rect = desk.getBoundingClientRect();
  return {
    x: clientX - rect.left + desk.scrollLeft,
    y: clientY - rect.top + desk.scrollTop,
  };
}

function hideCtx() {
  ctxMenu?.remove();
  ctxMenu = null;
}

function showCtx(x, y, items) {
  hideCtx();
  const menu = document.createElement("div");
  menu.className = "ctx-menu";
  menu.style.left = `${x}px`;
  menu.style.top = `${y}px`;

  for (const it of items) {
    if (it.sep) {
      menu.appendChild(Object.assign(document.createElement("div"), { className: "ctx-sep" }));
      continue;
    }
    if (it.submenu) {
      const wrap = document.createElement("div");
      wrap.className = "ctx-subwrap";
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "ctx-parent";
      btn.textContent = it.label + " ›";
      const sub = document.createElement("div");
      sub.className = "ctx-submenu";
      for (const s of it.submenu) {
        const b = document.createElement("button");
        b.type = "button";
        b.textContent = s.label;
        b.onclick = (e) => {
          e.stopPropagation();
          hideCtx();
          s.action();
        };
        sub.appendChild(b);
      }
      wrap.append(btn, sub);
      menu.appendChild(wrap);
      continue;
    }
    const b = document.createElement("button");
    b.type = "button";
    b.textContent = it.label;
    b.disabled = !!it.disabled;
    b.onclick = (e) => {
      e.stopPropagation();
      hideCtx();
      it.action();
    };
    menu.appendChild(b);
  }
  document.body.appendChild(menu);
  ctxMenu = menu;
}

document.addEventListener("click", hideCtx);

function render() {
  $("msg").textContent = state.message || "";
  $("state-pre").textContent = JSON.stringify(M.publicView(state), null, 2);
  const flash = $("flash");
  if (state.lastAction?.skillNames?.length) {
    flash.hidden = false;
    flash.textContent = state.message;
  } else {
    flash.hidden = true;
  }
  renderBinSide();
  renderDesktop();
}

function renderBinSide() {
  const root = $("bin-list");
  root.innerHTML = "";
  if (!state.recycleBin.length) {
    root.innerHTML = `<div style="color:var(--dim)">空</div>`;
    return;
  }
  for (const e of state.recycleBin) {
    const div = document.createElement("div");
    div.className = "bin-item";
    div.innerHTML = `<strong>${e.name}</strong>`;
    const btn = document.createElement("button");
    btn.textContent = "还原到桌面";
    btn.onclick = () => {
      state = M.restoreFromRecycle(state, e.skillId, 80, 80);
      render();
    };
    div.appendChild(btn);
    root.appendChild(div);
  }
}

function startPointerDrag(kind, id, ev, origX, origY) {
  ev.preventDefault();
  pointerDrag = {
    kind,
    id,
    startX: ev.clientX,
    startY: ev.clientY,
    origX,
    origY,
    moved: false,
  };
}

function onPointerMove(ev) {
  if (!pointerDrag) return;
  const dx = ev.clientX - pointerDrag.startX;
  const dy = ev.clientY - pointerDrag.startY;
  if (Math.abs(dx) + Math.abs(dy) > 4) pointerDrag.moved = true;
  const el = document.querySelector(`[data-drag-id="${pointerDrag.kind}:${pointerDrag.id}"]`);
  if (el) {
    el.style.transform = `translate(${dx}px,${dy}px)`;
    el.style.zIndex = "50";
    el.classList.add("dragging");
  }
}

function onPointerUp(ev) {
  if (!pointerDrag) return;
  const pd = pointerDrag;
  pointerDrag = null;
  const el = document.querySelector(`[data-drag-id="${pd.kind}:${pd.id}"]`);
  if (el) {
    el.style.transform = "";
    el.style.zIndex = "";
    el.classList.remove("dragging");
  }
  if (!pd.moved) {
    // short click: in multi-select toggle checkbox selection
    if (pd.kind === "ph" && state.multiSelect) {
      state = M.toggleSelected(state, pd.id);
      render();
    }
    return;
  }

  const pt = deskPoint(ev.clientX, ev.clientY);
  // hit-test under cursor without the dragged node
  if (el) el.style.pointerEvents = "none";
  const under = document.elementFromPoint(ev.clientX, ev.clientY);
  if (el) el.style.pointerEvents = "";
  const boxEl = under?.closest?.(".fence-box");
  const recycleEl = under?.closest?.(".recycle-icon");
  const iconEl = under?.closest?.(".app-icon.free:not(.system-icon)");

  if (pd.kind === "ph") {
    const ids =
      state.multiSelect && state.selectedIds.includes(pd.id)
        ? [...state.selectedIds]
        : [pd.id];
    if (recycleEl) {
      pendingTrashIds = ids;
      showTrashModal();
      return;
    }
    // icon overlaps a box → enter current tab/box
    if (boxEl) {
      const boxId = boxEl.dataset.boxId;
      const box = M.boxById(state, boxId);
      const cmp = box?.kind === "composite" ? box.activeCompartmentId : null;
      state = M.movePlaceholdersToBox(state, ids, boxId, cmp);
      render();
      return;
    }
    // drop on another desktop icon → auto box (model also checks cell)
    // free place on grid (snap + no overlap / auto-box)
    state = M.dropIconsOnDesktop(state, ids, pt.x - 30, pt.y - 30);
    render();
    return;
  }

  if (pd.kind === "box") {
    // drop onto another box = compose
    if (boxEl && boxEl.dataset.boxId !== pd.id) {
      const tgtId = boxEl.dataset.boxId;
      const src = M.boxById(state, pd.id);
      const tgt = M.boxById(state, tgtId);
      if (src?.kind === "simple" && tgt?.kind === "simple") {
        state = M.composeSimpleIntoSimple(state, pd.id, tgtId);
      } else if (src?.kind === "simple" && tgt?.kind === "composite") {
        state = M.addSimpleToComposite(state, pd.id, tgtId);
      } else {
        state = structuredClone(state);
        state.message = "禁止该组盒操作";
      }
      render();
      return;
    }
    // boxes must not overlap desktop icons
    if (iconEl && !iconEl.classList.contains("system-icon")) {
      state = structuredClone(state);
      state.message = "盒子不能与桌面图标重叠";
      // still try auto-nudge placement
    }
    state = M.moveBox(state, pd.id, pt.x - 40, pt.y - 12);
    render();
    return;
  }

  if (pd.kind === "recycle") {
    if (boxEl) {
      const boxId = boxEl.dataset.boxId;
      const box = M.boxById(state, boxId);
      const cmp = box?.kind === "composite" ? box.activeCompartmentId : null;
      state = M.moveRecycleToBox(state, boxId, cmp);
      render();
      return;
    }
    state = M.moveRecycleToDesktop(state, pt.x - 30, pt.y - 30);
    render();
    return;
  }

  if (pd.kind === "compartment") {
    const [boxId, cmpId] = pd.id.split("::");
    const pos = M.findBoxPosWithoutIconOverlap(state, pt.x, pt.y, 240, 220, null);
    state = M.ejectCompartment(state, boxId, cmpId, pos.x, pos.y);
    render();
  }
}

window.addEventListener("pointermove", onPointerMove);
window.addEventListener("pointerup", onPointerUp);

function appIcon(ph, { inBox = false } = {}) {
  const sk = M.skillById(state, ph.skillId);
  const count = M.countPlaceholdersForSkill(state, ph.skillId);
  const el = document.createElement("div");
  el.className =
    "app-icon" +
    (inBox ? " in-box" : " free") +
    (state.selectedIds.includes(ph.id) ? " selected" : "");
  el.dataset.dragId = `ph:${ph.id}`;
  el.dataset.phId = ph.id;

  if (!inBox && ph.location.type === "desktop") {
    el.style.left = `${ph.location.x}px`;
    el.style.top = `${ph.location.y}px`;
  }

  if (state.multiSelect) {
    const cb = document.createElement("input");
    cb.type = "checkbox";
    cb.className = "check";
    cb.checked = state.selectedIds.includes(ph.id);
    cb.addEventListener("click", (ev) => {
      ev.stopPropagation();
      state = M.toggleSelected(state, ph.id);
      render();
    });
    el.appendChild(cb);
  }

  const glyph = document.createElement("div");
  glyph.className = "glyph";
  glyph.textContent = (sk?.name || "?").slice(0, 2).toUpperCase();
  if (count > 1) {
    const badge = document.createElement("span");
    badge.className = "badge";
    badge.textContent = String(count);
    glyph.appendChild(badge);
  }
  const name = document.createElement("div");
  name.className = "name";
  name.textContent = sk?.name || ph.id;
  el.append(glyph, name);

  el.addEventListener("pointerdown", (ev) => {
    if (ev.button !== 0) return;
    if (ev.target.closest("input")) return;
    const ox = ph.location.type === "desktop" ? ph.location.x : 0;
    const oy = ph.location.type === "desktop" ? ph.location.y : 0;
    startPointerDrag("ph", ph.id, ev, ox, oy);
  });

  el.addEventListener("contextmenu", (ev) => {
    ev.preventDefault();
    ev.stopPropagation();
    const ids =
      state.multiSelect && state.selectedIds.includes(ph.id)
        ? [...state.selectedIds]
        : [ph.id];
    const items = [
      {
        label: "复制",
        action: () => {
          state = M.setClipboard(state, "copy", ids);
          render();
        },
      },
      {
        label: "剪切",
        action: () => {
          state = M.setClipboard(state, "cut", ids);
          render();
        },
      },
      {
        label: state.multiSelect ? "取消多选" : "多选",
        action: () => {
          if (state.multiSelect) {
            state = M.disableMultiSelect(state);
          } else {
            state = M.enableMultiSelectWith(state, ph.id);
          }
          render();
        },
      },
      { sep: true },
      {
        label: "删除",
        action: () => {
          pendingTrashIds = ids;
          showTrashModal();
        },
      },
    ];
    showCtx(ev.clientX, ev.clientY, items);
  });

  return el;
}

function recycleIconEl() {
  const loc = state.recycleIcon.location;
  const el = document.createElement("div");
  el.className = "app-icon system-icon recycle-icon" + (loc.type === "desktop" ? " free" : " in-box");
  el.dataset.dragId = `recycle:${state.recycleIcon.id}`;
  el.innerHTML = `<div class="glyph bin-glyph">🗑</div><div class="name">回收站</div>`;
  el.title = "系统图标：可拖动/入盒；不可复制剪切删除；右键可清空";

  if (loc.type === "desktop") {
    el.style.left = `${loc.x}px`;
    el.style.top = `${loc.y}px`;
  }

  el.addEventListener("pointerdown", (ev) => {
    if (ev.button !== 0) return;
    const ox = loc.type === "desktop" ? loc.x : 0;
    const oy = loc.type === "desktop" ? loc.y : 0;
    startPointerDrag("recycle", state.recycleIcon.id, ev, ox, oy);
  });

  el.addEventListener("contextmenu", (ev) => {
    ev.preventDefault();
    ev.stopPropagation();
    showCtx(ev.clientX, ev.clientY, [
      {
        label: "清空当前回收站",
        disabled: !state.recycleBin.length,
        action: () => {
          if (!state.recycleBin.length) return;
          const paths = state.recycleBin.map((e) => e.path).join("\n");
          if (confirm(`清空回收站并真删（模拟）？\n\n${paths}`)) {
            state = M.emptyRecycleBin(state);
            render();
          }
        },
      },
    ]);
  });

  return el;
}

function renderBox(box) {
  const root = document.createElement("div");
  root.className = "fence-box";
  root.dataset.boxId = box.id;
  root.dataset.dragId = `box:${box.id}`;
  root.style.left = `${box.x}px`;
  root.style.top = `${box.y}px`;
  root.style.width = `${box.w}px`;
  root.style.minHeight = `${box.h}px`;

  const titlebar = document.createElement("div");
  titlebar.className = "fence-title";
  const kind = document.createElement("span");
  kind.className = "kind";
  kind.textContent = box.kind === "simple" ? "普通盒" : "组合盒";
  const title = document.createElement("span");
  title.className = "title-text";
  title.textContent = box.kind === "simple" ? box.tag : box.title || "组合盒";
  title.title =
    box.kind === "simple"
      ? "双击编辑标签"
      : "双击编辑组合盒标题";
  // double-click: simple = tag; composite title = title
  title.addEventListener("dblclick", (e) => {
    e.stopPropagation();
    e.preventDefault();
    if (box.kind === "simple") {
      const t = prompt("编辑标签", box.tag || "");
      if (t != null && t.trim()) {
        state = M.renameBoxTag(state, box.id, t.trim());
        render();
      }
    } else {
      const t = prompt("组合盒标题", box.title || "");
      if (t != null && t.trim()) {
        state = M.renameCompositeTitle(state, box.id, t.trim());
        render();
      }
    }
  });
  titlebar.append(kind, title);
  titlebar.addEventListener("pointerdown", (ev) => {
    if (ev.button !== 0) return;
    // don't start drag from title if double-click intent — still ok for single
    startPointerDrag("box", box.id, ev, box.x, box.y);
  });
  root.appendChild(titlebar);

  let itemIds = [];
  let activeCmp = null;
  if (box.kind === "composite") {
    const tabs = document.createElement("div");
    tabs.className = "fence-tabs";
    for (const c of box.compartments) {
      const tab = document.createElement("div");
      tab.className = "tab" + (c.id === box.activeCompartmentId ? " active" : "");
      tab.textContent = `${c.tag} (${c.itemIds.length})`;
      tab.title = "悬停切换 · 双击编辑标签 · 拖出=普通盒";
      tab.addEventListener("mouseenter", () => {
        if (box.activeCompartmentId !== c.id) {
          state = M.setActiveCompartment(state, box.id, c.id);
          render();
        }
      });
      // double-click to rename tag — suppress drag
      let tabDragTimer = null;
      tab.addEventListener("pointerdown", (ev) => {
        if (ev.button !== 0) return;
        ev.stopPropagation();
        tabDragTimer = setTimeout(() => {
          startPointerDrag("compartment", `${box.id}::${c.id}`, ev, box.x, box.y);
        }, 200);
      });
      tab.addEventListener("pointerup", () => {
        if (tabDragTimer) clearTimeout(tabDragTimer);
      });
      tab.addEventListener("pointerleave", () => {
        if (tabDragTimer) clearTimeout(tabDragTimer);
      });
      tab.addEventListener("dblclick", (ev) => {
        ev.stopPropagation();
        ev.preventDefault();
        if (tabDragTimer) clearTimeout(tabDragTimer);
        const t = prompt("编辑隔间标签", c.tag || "");
        if (t != null && t.trim()) {
          state = M.renameBoxTag(state, box.id, t.trim(), c.id);
          render();
        }
      });
      tabs.appendChild(tab);
    }
    root.appendChild(tabs);
    const c = box.compartments.find((x) => x.id === box.activeCompartmentId);
    itemIds = c ? [...c.itemIds] : [];
    activeCmp = box.activeCompartmentId;
  } else {
    itemIds = [...box.itemIds];
  }

  const body = document.createElement("div");
  body.className = "fence-body";
  if (!itemIds.length && !recycleInThis(box, activeCmp)) {
    body.innerHTML = `<div class="fence-empty">拖入快捷方式图标</div>`;
  } else {
    for (const id of itemIds) {
      const ph = M.phById(state, id);
      if (ph) body.appendChild(appIcon(ph, { inBox: true }));
    }
    if (recycleInThis(box, activeCmp)) {
      body.appendChild(recycleIconEl());
    }
  }
  root.appendChild(body);

  const foot = document.createElement("div");
  foot.className = "fence-foot";
  foot.innerHTML = `<span>${itemIds.length} 个项目</span>`;
  root.appendChild(foot);

  // right-click box (not on skill icon) → paste + delete box
  root.addEventListener("contextmenu", (ev) => {
    if (ev.target.closest(".app-icon")) return;
    ev.preventDefault();
    ev.stopPropagation();
    showCtx(ev.clientX, ev.clientY, [
      {
        label: "粘贴",
        disabled: !state.clipboard?.phIds?.length,
        action: () => {
          state = M.pasteClipboard(state, {
            type: "box",
            boxId: box.id,
            compartmentId: activeCmp,
          });
          render();
        },
      },
      { sep: true },
      {
        label: "删除盒子",
        action: () => {
          if (confirm(`删除盒子「${box.kind === "simple" ? box.tag : box.title}」？盒内图标将回到桌面。`)) {
            state = M.deleteBox(state, box.id);
            render();
          }
        },
      },
    ]);
  });

  return root;
}

function recycleInThis(box, compartmentId) {
  const loc = state.recycleIcon.location;
  if (loc.type !== "box" || loc.boxId !== box.id) return false;
  if (box.kind === "simple") return true;
  return loc.compartmentId === compartmentId;
}

function renderDesktop() {
  const desk = $("desktop");
  desk.innerHTML = "";

  // free skill icons on desktop
  for (const ph of state.placeholders) {
    if (ph.location.type === "desktop") {
      desk.appendChild(appIcon(ph));
    }
  }

  // boxes
  for (const box of state.boxes) {
    desk.appendChild(renderBox(box));
  }

  // recycle on desktop
  if (state.recycleIcon.location.type === "desktop") {
    desk.appendChild(recycleIconEl());
  }

  // empty desktop context menu (only when not on icon/box)
  desk.oncontextmenu = (ev) => {
    if (ev.target.closest(".app-icon, .fence-box, .ctx-menu")) return;
    ev.preventDefault();
    const pt = deskPoint(ev.clientX, ev.clientY);
    showCtx(ev.clientX, ev.clientY, [
      {
        label: "粘贴",
        disabled: !state.clipboard?.phIds?.length,
        action: () => {
          state = M.pasteClipboard(state, { type: "desktop", x: pt.x, y: pt.y });
          render();
        },
      },
      { sep: true },
      {
        label: "创建新盒子",
        submenu: [
          {
            label: "创建普通盒子",
            action: () => {
              const tag = prompt("普通盒标签名", "新建");
              if (!tag?.trim()) return;
              state = M.createSimpleBox(state, tag.trim(), pt.x, pt.y);
              render();
            },
          },
          {
            label: "创建组合盒子",
            action: () => {
              const title = prompt("组合盒标题", "Go 开发");
              if (!title?.trim()) return;
              const tagsStr = prompt("隔间标签（逗号分隔，至少2个）", "testing,modules");
              if (!tagsStr?.trim()) return;
              const tags = tagsStr.split(/[,，]/).map((s) => s.trim()).filter(Boolean);
              state = M.createCompositeBox(state, title.trim(), tags, pt.x, pt.y);
              render();
            },
          },
        ],
      },
    ]);
  };
}

function showTrashModal() {
  const ids = pendingTrashIds || [];
  const rows = ids.map((id) => {
    const ph = M.phById(state, id);
    const sk = M.skillById(state, ph?.skillId);
    const n = M.countPlaceholdersForSkill(state, ph?.skillId);
    return { name: sk?.name, last: n <= 1 };
  });
  const root = $("modal-root");
  root.innerHTML = `
    <div class="modal-back">
      <div class="modal">
        <h3>删除</h3>
        <ul>
          ${rows
            .map(
              (r) =>
                `<li><strong>${r.name}</strong> — ${
                  r.last ? "最后一枚 → 本体进回收站" : "仅移除图标"
                }</li>`
            )
            .join("")}
        </ul>
        <div class="actions">
          <button type="button" id="cancel-trash">取消</button>
          <button type="button" class="primary" id="ok-trash">确定</button>
        </div>
      </div>
    </div>`;
  root.querySelector("#cancel-trash").onclick = () => {
    pendingTrashIds = null;
    root.innerHTML = "";
  };
  root.querySelector("#ok-trash").onclick = () => {
    state = M.confirmTrash(state, ids);
    pendingTrashIds = null;
    root.innerHTML = "";
    render();
  };
}

function bindToolbar() {
  // 测试用重置；成品不提供此按钮
  $("btn-reset").onclick = () => {
    state = M.seedState();
    render();
  };
}

bindToolbar();
render();
