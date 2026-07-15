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

## 架构（v1）

| 模块 | 职责 |
|------|------|
| `internal/workbench` | 唯一产品门面（领域规则与测试主缝） |
| `internal/scanner` | 扫描根 → realpath 身份 |
| `internal/index` | 中央索引 JSON 原子读写 |
| `internal/quarantine` | 同盘 rename 隔离 / 还原 / 真删 |
| `internal/server` + `internal/ui` | 薄 HTTP + 嵌入静态前端 |

## 阶段状态

- **v1 领域 + CLI/HTTP**：#2–#7 已实现，可端到端使用。
- **前端**：可用但未定稿（交互/视觉/框架待另开讨论）；不以 `prototypes/` 为生产源码。
- **非 v1**：挑选器 / fzf / 市场 / 常驻 daemon。

## 原型

- 行为基准（已验收）：`prototypes/workbench-desktop/`
- 已否决路径：`prototypes/tag-pick-flow/`
