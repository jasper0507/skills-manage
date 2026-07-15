// Package workbench is the primary application facade for skills-manage.
// Callers (CLI, HTTP, tests) speak only to Workbench for product behavior.
//
// File layout (same package, split for cohesion):
//
//	types.go      — view/model types and constants
//	workbench.go  — construct, open/rescan, desk view, persistence helpers
//	placement.go  — durable placement write primitives + multi-item desktop place
//	mutate.go     — document snapshot / single-persist transaction
//	rehome.go     — load-only membership repair + membership claim / projection
//	layout.go     — icon grid geometry and free-cell placement
//	desk.go       — placeholder desktop moves and scan reconcile
//	box.go        — 普通/组合盒子 compose, eject, create, delete
//	clipboard.go  — copy/cut/paste and multi-select
//	recycle.go    — 回收站 system icon, trash, restore, empty
package workbench
