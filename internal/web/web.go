// Package web embeds the dashboard HTML into the binary.
package web

import (
	"embed"
)

//go:embed dashboard.html
var DashboardFS embed.FS
