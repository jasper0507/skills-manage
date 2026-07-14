// Package ui holds the embedded 分类工作台 front-end assets.
package ui

import "embed"

// FS is the static workbench UI (index.html, app.js, styles.css).
//
//go:embed index.html app.js styles.css
var FS embed.FS
