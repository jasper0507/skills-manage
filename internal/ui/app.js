/**
 * 分类工作台 UI — thin client over /api/* → Workbench.
 * Grid/pixel helpers are presentation only; domain rules live on the server.
 */

const ICON_GRID = {
  originX: 16,
  originY: 16,
  w: 90,
  h: 96,
  iconW: 86,
  iconH: 90,
};

/** @type {{ desk: any, recycleBin: any[] }} */
let state = { desk: emptyDesk(), recycleBin: [] };
let pendingTrashIds = null;
let pointerDrag = null;
let ctxMenu = null;
let msg = "";

const $ = (id) => document.getElementById(id);

function emptyDesk() {
  return {
    placeholders: [],
    recycleIcon: { location: { kind: "desktop", row: 1, col: 1 } },
    boxes: [],
    clipboard: null,
    multiSelect: false,
    selectedIds: [],
  };
}

function cellToPx(row, col) {
  // Workbench uses 1-based row/col.
  return {
    x: ICON_GRID.originX + (col - 1) * ICON_GRID.w,
    y: ICON_GRID.originY + (row - 1) * ICON_GRID.h,
  };
}

function pxToCell(x, y) {
  const col = Math.max(1, Math.round((x - ICON_GRID.originX) / ICON_GRID.w) + 1);
  const row = Math.max(1, Math.round((y - ICON_GRID.originY) / ICON_GRID.h) + 1);
  return { row, col };
}

function deskPoint(clientX, clientY) {
  const desk = $("desktop");
  const rect = desk.getBoundingClientRect();
  return {
    x: clientX - rect.left + desk.scrollLeft,
    y: clientY - rect.top + desk.scrollTop,
  };
}

async function api(method, path, body) {
  const opts = { method, headers: {} };
  if (body !== undefined) {
    opts.headers["Content-Type"] = "application/json";
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(path, opts);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || res.statusText || String(res.status));
  }
  return data;
}

async function loadState() {
  const data = await api("GET", "/api/state");
  applyState(data);
}

function applyState(data) {
  if (data.desk) state.desk = data.desk;
  if (Array.isArray(data.recycleBin)) state.recycleBin = data.recycleBin;
  // Normalize slices that may be null from JSON.
  state.desk.placeholders = state.desk.placeholders || [];
  state.desk.boxes = state.desk.boxes || [];
  state.desk.selectedIds = state.desk.selectedIds || [];
  render();
}

/** Serialize mutations so out-of-order responses cannot clobber newer desk state. */
let mutChain = Promise.resolve();

function mut(path, body) {
  const run = async () => {
    try {
      const data = await api("POST", path, body || {});
      msg = "";
      applyState(data);
      return data;
    } catch (e) {
      msg = String(e.message || e);
      render();
      throw e;
    }
  };
  const next = mutChain.then(run, run);
  // Keep the chain alive even when a mutation fails.
  mutChain = next.then(
    () => {},
    () => {}
  );
  return next;
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

function phById(id) {
  return state.desk.placeholders.find((p) => p.id === id);
}

function boxById(id) {
  return state.desk.boxes.find((b) => b.id === id);
}

function countPlaceholdersForIdentity(identity) {
  return state.desk.placeholders.filter((p) => p.identity === identity).length;
}

function render() {
  $("msg").textContent = msg || "";
  const flash = $("flash");
  flash.hidden = true;
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
    div.innerHTML = `<strong>${escapeHtml(e.name || e.identity)}</strong>
      <div style="color:var(--dim);word-break:break-all">${escapeHtml(e.originalPath || "")}</div>`;
    const btn = document.createElement("button");
    btn.textContent = "还原";
    btn.onclick = () => mut("/api/recycle/restore", { entryId: e.id }).catch(() => {});
    div.appendChild(btn);
    root.appendChild(div);
  }
}

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
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

async function onPointerUp(ev) {
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
    if (pd.kind === "ph" && state.desk.multiSelect) {
      await mut("/api/multiselect/toggle", { placeholderId: pd.id }).catch(() => {});
    }
    return;
  }

  const pt = deskPoint(ev.clientX, ev.clientY);
  if (el) el.style.pointerEvents = "none";
  const under = document.elementFromPoint(ev.clientX, ev.clientY);
  if (el) el.style.pointerEvents = "";
  const boxEl = under?.closest?.(".fence-box");
  const recycleEl = under?.closest?.(".recycle-icon");

  try {
    if (pd.kind === "ph") {
      const ids =
        state.desk.multiSelect && state.desk.selectedIds.includes(pd.id)
          ? [...state.desk.selectedIds]
          : [pd.id];
      if (recycleEl) {
        pendingTrashIds = ids;
        await showTrashModal();
        return;
      }
      if (boxEl) {
        const boxId = boxEl.dataset.boxId;
        const box = boxById(boxId);
        const cmp = box?.kind === "composite" ? box.activeCompartmentId : "";
        if (ids.length === 1) {
          await mut("/api/placeholders/move-box", {
            placeholderId: ids[0],
            boxId,
            compartmentId: cmp || "",
          });
        } else {
          await mut("/api/placeholders/move-many-box", {
            placeholderIds: ids,
            boxId,
            compartmentId: cmp || "",
          });
        }
        return;
      }
      // Desktop drop: bulk endpoint parks movers and free-cells (or auto-box on occupant).
      const cell = pxToCell(pt.x - 30, pt.y - 30);
      if (ids.length === 1) {
        await mut("/api/placeholders/move-desktop", {
          placeholderId: ids[0],
          row: cell.row,
          col: cell.col,
        });
      } else {
        await mut("/api/placeholders/move-many-desktop", {
          placeholderIds: ids,
          row: cell.row,
          col: cell.col,
        });
      }
      return;
    }

    if (pd.kind === "box") {
      if (boxEl && boxEl.dataset.boxId !== pd.id) {
        await mut("/api/boxes/compose", {
          sourceBoxId: pd.id,
          targetBoxId: boxEl.dataset.boxId,
        });
        return;
      }
      await mut("/api/boxes/move", { boxId: pd.id, x: pt.x - 40, y: pt.y - 12 });
      return;
    }

    if (pd.kind === "recycle") {
      if (boxEl) {
        const boxId = boxEl.dataset.boxId;
        const box = boxById(boxId);
        const cmp = box?.kind === "composite" ? box.activeCompartmentId : "";
        await mut("/api/recycle/move-box", {
          boxId,
          compartmentId: cmp || "",
        });
        return;
      }
      const cell = pxToCell(pt.x - 30, pt.y - 30);
      await mut("/api/recycle/move-desktop", { row: cell.row, col: cell.col });
      return;
    }

    if (pd.kind === "compartment") {
      const [boxId, cmpId] = pd.id.split("::");
      await mut("/api/boxes/eject", {
        boxId,
        compartmentId: cmpId,
        x: pt.x,
        y: pt.y,
      });
    }
  } catch {
    /* msg already set */
  }
}

window.addEventListener("pointermove", onPointerMove);
window.addEventListener("pointerup", onPointerUp);

function appIcon(ph, { inBox = false } = {}) {
  const count = countPlaceholdersForIdentity(ph.identity);
  const el = document.createElement("div");
  el.className =
    "app-icon" +
    (inBox ? " in-box" : " free") +
    (state.desk.selectedIds.includes(ph.id) ? " selected" : "");
  el.dataset.dragId = `ph:${ph.id}`;
  el.dataset.phId = ph.id;

  if (!inBox && ph.location?.kind === "desktop") {
    const px = cellToPx(ph.location.row, ph.location.col);
    el.style.left = `${px.x}px`;
    el.style.top = `${px.y}px`;
  }

  if (state.desk.multiSelect) {
    const cb = document.createElement("input");
    cb.type = "checkbox";
    cb.className = "check";
    cb.checked = state.desk.selectedIds.includes(ph.id);
    cb.addEventListener("click", (ev) => {
      ev.stopPropagation();
      mut("/api/multiselect/toggle", { placeholderId: ph.id }).catch(() => {});
    });
    el.appendChild(cb);
  }

  const glyph = document.createElement("div");
  glyph.className = "glyph";
  glyph.textContent = (ph.name || "?").slice(0, 2).toUpperCase();
  if (count > 1) {
    const badge = document.createElement("span");
    badge.className = "badge";
    badge.textContent = String(count);
    glyph.appendChild(badge);
  }
  const name = document.createElement("div");
  name.className = "name";
  name.textContent = ph.name || ph.id;
  el.append(glyph, name);

  el.addEventListener("pointerdown", (ev) => {
    if (ev.button !== 0) return;
    if (ev.target.closest("input")) return;
    const px =
      ph.location?.kind === "desktop"
        ? cellToPx(ph.location.row, ph.location.col)
        : { x: 0, y: 0 };
    startPointerDrag("ph", ph.id, ev, px.x, px.y);
  });

  el.addEventListener("contextmenu", (ev) => {
    ev.preventDefault();
    ev.stopPropagation();
    const ids =
      state.desk.multiSelect && state.desk.selectedIds.includes(ph.id)
        ? [...state.desk.selectedIds]
        : [ph.id];
    showCtx(ev.clientX, ev.clientY, [
      {
        label: "复制",
        action: () =>
          mut("/api/clipboard/set", { mode: "copy", placeholderIds: ids }).catch(() => {}),
      },
      {
        label: "剪切",
        action: () =>
          mut("/api/clipboard/set", { mode: "cut", placeholderIds: ids }).catch(() => {}),
      },
      {
        label: state.desk.multiSelect ? "取消多选" : "多选",
        action: () => {
          if (state.desk.multiSelect) {
            mut("/api/multiselect/disable", {}).catch(() => {});
          } else {
            mut("/api/multiselect/enable", { placeholderId: ph.id }).catch(() => {});
          }
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
    ]);
  });

  return el;
}

function recycleIconEl() {
  const loc = state.desk.recycleIcon.location;
  const el = document.createElement("div");
  el.className =
    "app-icon system-icon recycle-icon" + (loc.kind === "desktop" ? " free" : " in-box");
  el.dataset.dragId = `recycle:bin`;
  el.innerHTML = `<div class="glyph bin-glyph">🗑</div><div class="name">回收站</div>`;
  el.title = "系统图标：可拖动/入盒；右键可清空；不可复制/剪切/删除";

  if (loc.kind === "desktop") {
    const px = cellToPx(loc.row || 1, loc.col || 1);
    el.style.left = `${px.x}px`;
    el.style.top = `${px.y}px`;
  }

  el.addEventListener("pointerdown", (ev) => {
    if (ev.button !== 0) return;
    const px =
      loc.kind === "desktop"
        ? cellToPx(loc.row || 1, loc.col || 1)
        : { x: 0, y: 0 };
    startPointerDrag("recycle", "bin", ev, px.x, px.y);
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
          const paths = state.recycleBin.map((e) => e.originalPath || e.quarantinePath).join("\n");
          if (confirm(`清空回收站并真删？\n\n${paths}`)) {
            mut("/api/recycle/empty", {}).catch(() => {});
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
  title.title = box.kind === "simple" ? "双击编辑标签" : "双击编辑组合盒标题";
  title.addEventListener("dblclick", (e) => {
    e.stopPropagation();
    e.preventDefault();
    if (box.kind === "simple") {
      const t = prompt("编辑标签", box.tag || "");
      if (t != null && t.trim()) {
        mut("/api/boxes/rename-tag", { boxId: box.id, tag: t.trim(), compartmentId: "" }).catch(
          () => {}
        );
      }
    } else {
      const t = prompt("组合盒标题", box.title || "");
      if (t != null && t.trim()) {
        mut("/api/boxes/rename-title", { boxId: box.id, title: t.trim() }).catch(() => {});
      }
    }
  });
  titlebar.append(kind, title);
  titlebar.addEventListener("pointerdown", (ev) => {
    if (ev.button !== 0) return;
    startPointerDrag("box", box.id, ev, box.x, box.y);
  });
  root.appendChild(titlebar);

  let items = [];
  let activeCmp = "";
  if (box.kind === "composite") {
    const tabs = document.createElement("div");
    tabs.className = "fence-tabs";
    for (const c of box.compartments || []) {
      const tab = document.createElement("div");
      tab.className = "tab" + (c.id === box.activeCompartmentId ? " active" : "");
      const n = (c.items || []).length;
      tab.textContent = `${c.tag} (${n})`;
      tab.title = "悬停切换 · 双击编辑标签 · 拖出=普通盒";
      tab.addEventListener("mouseenter", () => {
        if (box.activeCompartmentId !== c.id) {
          mut("/api/boxes/set-active", { boxId: box.id, compartmentId: c.id }).catch(() => {});
        }
      });
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
          mut("/api/boxes/rename-tag", {
            boxId: box.id,
            tag: t.trim(),
            compartmentId: c.id,
          }).catch(() => {});
        }
      });
      tabs.appendChild(tab);
    }
    root.appendChild(tabs);
    const c = (box.compartments || []).find((x) => x.id === box.activeCompartmentId);
    items = c ? [...(c.items || [])] : [];
    activeCmp = box.activeCompartmentId || "";
  } else {
    items = [...(box.items || [])];
  }

  const body = document.createElement("div");
  body.className = "fence-body";
  if (!items.length && !recycleInThis(box, activeCmp)) {
    body.innerHTML = `<div class="fence-empty">拖入快捷方式图标</div>`;
  } else {
    for (const ph of items) {
      body.appendChild(appIcon(ph, { inBox: true }));
    }
    if (recycleInThis(box, activeCmp)) {
      body.appendChild(recycleIconEl());
    }
  }
  root.appendChild(body);

  const foot = document.createElement("div");
  foot.className = "fence-foot";
  foot.innerHTML = `<span>${items.length} 个项目</span>`;
  root.appendChild(foot);

  root.addEventListener("contextmenu", (ev) => {
    if (ev.target.closest(".app-icon")) return;
    ev.preventDefault();
    ev.stopPropagation();
    const clip = state.desk.clipboard;
    showCtx(ev.clientX, ev.clientY, [
      {
        label: "粘贴",
        disabled: !clip?.placeholderIds?.length,
        action: () =>
          mut("/api/clipboard/paste-box", {
            boxId: box.id,
            compartmentId: activeCmp || "",
          }).catch(() => {}),
      },
      { sep: true },
      {
        label: "删除盒子",
        action: () => {
          if (
            confirm(
              `删除盒子「${box.kind === "simple" ? box.tag : box.title}」？盒内图标将回到桌面。`
            )
          ) {
            mut("/api/boxes/delete", { boxId: box.id }).catch(() => {});
          }
        },
      },
    ]);
  });

  return root;
}

function recycleInThis(box, compartmentId) {
  const loc = state.desk.recycleIcon.location;
  if (loc.kind !== "box" || loc.boxId !== box.id) return false;
  if (box.kind === "simple") return true;
  return loc.compartmentId === compartmentId;
}

function renderDesktop() {
  const desk = $("desktop");
  desk.innerHTML = "";

  for (const ph of state.desk.placeholders) {
    if (ph.location?.kind === "desktop") {
      desk.appendChild(appIcon(ph));
    }
  }

  for (const box of state.desk.boxes) {
    desk.appendChild(renderBox(box));
  }

  if (state.desk.recycleIcon?.location?.kind === "desktop") {
    desk.appendChild(recycleIconEl());
  }

  desk.oncontextmenu = (ev) => {
    if (ev.target.closest(".app-icon, .fence-box, .ctx-menu")) return;
    ev.preventDefault();
    const pt = deskPoint(ev.clientX, ev.clientY);
    const cell = pxToCell(pt.x, pt.y);
    const clip = state.desk.clipboard;
    showCtx(ev.clientX, ev.clientY, [
      {
        label: "粘贴",
        disabled: !clip?.placeholderIds?.length,
        action: () =>
          mut("/api/clipboard/paste-desktop", { row: cell.row, col: cell.col }).catch(() => {}),
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
              mut("/api/boxes/create-simple", {
                tag: tag.trim(),
                x: pt.x,
                y: pt.y,
              }).catch(() => {});
            },
          },
          {
            label: "创建组合盒子",
            action: () => {
              const title = prompt("组合盒标题", "主题");
              if (!title?.trim()) return;
              const tagsStr = prompt("隔间标签（逗号分隔，至少2个）", "a,b");
              if (!tagsStr?.trim()) return;
              const tags = tagsStr
                .split(/[,，]/)
                .map((s) => s.trim())
                .filter(Boolean);
              mut("/api/boxes/create-composite", {
                title: title.trim(),
                tags,
                x: pt.x,
                y: pt.y,
              }).catch(() => {});
            },
          },
        ],
      },
    ]);
  };
}

async function showTrashModal() {
  const ids = pendingTrashIds || [];
  let plan;
  try {
    plan = await api("POST", "/api/trash/plan", { placeholderIds: ids });
  } catch (e) {
    msg = String(e.message || e);
    pendingTrashIds = null;
    render();
    return;
  }

  const iconOnly = new Set(plan.iconOnlyIds || []);
  const bodyByPh = new Map();
  for (const b of plan.bodyItems || []) {
    for (const pid of b.placeholderIds || []) {
      bodyByPh.set(pid, b);
    }
  }

  const rows = ids.map((id) => {
    const ph = phById(id);
    const body = bodyByPh.get(id);
    if (body) {
      return {
        name: body.name || ph?.name || id,
        detail: `最后一枚 → 本体进回收站\n${body.path || ""}`,
        last: true,
      };
    }
    return {
      name: ph?.name || id,
      detail: iconOnly.has(id) ? "仅移除图标" : "删除",
      last: false,
    };
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
                `<li><strong>${escapeHtml(r.name)}</strong> — ${escapeHtml(r.detail).replace(
                  /\n/g,
                  "<br>"
                )}</li>`
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
  root.querySelector("#ok-trash").onclick = async () => {
    const confirmIds = ids;
    pendingTrashIds = null;
    root.innerHTML = "";
    await mut("/api/trash/confirm", { placeholderIds: confirmIds }).catch(() => {});
  };
}

function bindToolbar() {
  $("btn-rescan").onclick = () => mut("/api/rescan", {}).catch(() => {});
}

bindToolbar();
loadState().catch((e) => {
  msg = "无法加载工作台: " + (e.message || e);
  render();
});
