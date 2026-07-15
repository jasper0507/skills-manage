# skills-manage

本机 agent skills 的 **分类工作台**（v1）：扫描本机 Skill → 桌面式归类 → 用户级中央索引 → 回收站隔离删除。

领域语言与产品规则见根目录 [`CONTEXT.md`](./CONTEXT.md)。

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

**删除 / 清空回收站会动磁盘上的 skill 包。** 测删除请用临时 `-root` 与 `-index`，不要直接对生产 skill 树乱点确认。

## 目录结构（v1）

参考常见 Go 分层，按**本产品**裁剪（无 MySQL/Redis/用户业务模块）：

```
cmd/skills-manage/main.go     # 入口，只调用 internal/app
config/                       # 配置解析与默认值（扫描根、索引路径、listen）
internal/
  app/                        # 组装：Run + inventory/desk/serve 命令
  workbench/                  # 领域门面（唯一产品主缝 / 主测缝）
    types.go, workbench.go, layout.go, desk.go, box.go, clipboard.go, recycle.go
  server/                     # 薄 HTTP：router + handlers_* + listen/run
  ui/                         # 嵌入静态前端
  infra/
    scanner/                  # 扫描根 → realpath 身份
    index/                    # 中央索引 JSON 原子读写
    quarantine/               # 同盘 rename 隔离 / 还原 / 真删
```

设计原则：包按依赖方向向下（cmd → app → workbench/server；workbench → infra）。**Workbench 保持深模块门面**，不拆成 handler/service/repo 三层。同包按关注点拆文件，便于阅读，不增加耦合。

| 你参考的模板 | 本仓库对应 |
|--------------|------------|
| `cmd` → `internal/app` 组装 | `cmd` + `internal/app` |
| `config/` | `config/`（当前为 flag/默认值，非 yaml 文件） |
| `infra/mysql|redis` | `infra/scanner|index|quarantine` |
| `router` + `server` | `internal/server`（`router.go` + `run.go`） |
| `user/handler|service|repo` | **不拆**：领域集中在 `workbench` 深模块门面 |

## 阶段状态

- **v1 领域 + CLI/HTTP**：#2–#7 已实现，可端到端使用。
- **前端**：可用但未定稿；不以 `prototypes/` 为生产源码。
- **非 v1**：挑选器 / fzf / 市场 / 常驻 daemon。

## 原型

- 行为基准（已验收）：`prototypes/workbench-desktop/`
- 已否决路径：`prototypes/tag-pick-flow/`
