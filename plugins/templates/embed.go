// Package templates provides embedded plugin scaffold templates for the vpnctl CLI.
package templates

import "embed"

// FS contains the embedded plugin template files. Templates are organized by
// language under subdirectories (e.g., go/).
//
//go:embed go/*.tmpl
var FS embed.FS
