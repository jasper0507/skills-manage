# skills-manage

本机 agent skills 的 **分类工作台**（v1）：扫描本机 Skill → 桌面式归类 → 用户级中央索引 → **图标级**回收站（只管理占位/快捷方式，**不**删除磁盘上的 Skill 包）。

领域语言与产品规则见根目录 [`CONTEXT.md`](./CONTEXT.md)。v1 回收站边界见 [`docs/adr/0001-v1-icon-only-recycle-bin.md`](./docs/adr/0001-v1-icon-only-recycle-bin.md)。

## 构建与运行

```bash
go test ./...
go build -o ./skills-manage ./cmd/skills-manage

# 清单
./skills-manage inventory
./skills-manage desk

# 分类工作台（localhost HTTP + 嵌入 UI，进程随命令结束）
./skills-manage serve
./skills-manage serve -root ~/.agents/skills -addr 127.0.0.1:8765
./skills-manage serve -no-open   # 只打印 URL
```

默认中央索引：`$CONFIG/skills-manage/index.json`（可用 `-index` 覆盖）。

**说明：** 后端回收站已对齐 R2（图标级软回收）；`infra/quarantine` 已移除。E2 余下 Open 旧索引兼容与 rehome/快照（见 issues #10 / #11）。薄 UI 可能仍引用旧 body 字段——UI 不在 E2 范围。

## 目录结构（v1）

```
cmd/skills-manage/main.go     # 入口，只调用 internal/app
config/                       # 配置解析与默认值（扫描根、索引路径、listen）
internal/
  app/                        # 组装：Run + inventory/desk/serve 命令
  workbench/                  # 领域门面（唯一产品主缝 / 主测缝）
  server/                     # 薄 HTTP
  ui/                         # 嵌入静态前端（未定稿）
  infra/
    scanner/                  # 扫描根 → realpath 身份
    index/                    # 中央索引 JSON 原子读写
```

设计原则：包按依赖方向向下。**Workbench 保持深模块门面**，不拆成 handler/service/repo 三层。

## 阶段状态

- **产品共识：** R2 图标级回收站；禁止最后一枚活占位进站；禁止 Skill 本体删除。见 `CONTEXT.md` / ADR-0001。
- **代码：** #2–#7 骨架 + **E2.1** R2 回收 / 去 body-delete；E2.2–E2.3 待做。
- **前端：** 可用但未定稿；不以 `prototypes/` 为生产源码。
- **非 v1：** 挑选器 / fzf / 市场 / 常驻 daemon / Skill 包隔离真删。

## 原型

- 行为基准（已验收桌面/盒）：`prototypes/workbench-desktop/`（若与 `CONTEXT.md` 冲突，以 CONTEXT 为准，尤其删除语义）
- 已否决路径：`prototypes/tag-pick-flow/`
