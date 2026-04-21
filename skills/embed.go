package contractskills

import "embed"

// FS contains the bundled Codex skills shipped with contract-cli.
//
//go:embed */SKILL.md */agents/* */references/*
var FS embed.FS
