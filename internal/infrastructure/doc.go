// Package infrastructure is the root of Forge's adapter layer.
//
// Adapters implement interfaces declared by Domain (and, where the consumer is
// Application, by Application). Examples:
//
//	filesystem   — osfs, memfs, safefs policy
//	config       — loader, discovery, precedence
//	renderer     — human and JSON output
//	registry     — component/template registries
//	git          — repository operations
//	logger       — slog-backed implementation of the logger interface
//
// Infrastructure imports Domain and Shared. It never imports Application or
// Presentation. Concrete adapters are wired by the composition root in
// internal/cli.
//
// This file exists so that the package is classified by internal/arch from
// day one. Adapters are relocated here under WBS 14.2.x.
package infrastructure
