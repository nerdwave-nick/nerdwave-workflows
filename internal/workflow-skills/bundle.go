// Package workflowskills holds the offline, deployable workflow skill suite.
package workflowskills

import (
	"embed"
	"io/fs"
)

// Explicit file patterns keep historical observations, tests, and obsolete
// runtime programs out of the distributable bundle. New asset formats must be
// deliberately added here and covered by the bundle contract test.
//
//go:embed AGENTS_MD_TEMPLATE.md skills/*/*.md skills/*/agents/*.yaml skills/*/references/*.md
var assets embed.FS

// Bundle returns a read-only filesystem rooted at the instruction template and
// skills directory. It needs no source checkout or runtime downloads.
func Bundle() fs.FS { return assets }
