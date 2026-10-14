// Package shared is the root of Forge's cross-cutting primitives.
//
// Contents (present or planned):
//
//	errors   — the closed error taxonomy and formatting (today: internal/forgeerr)
//	logging  — the logger interface (today: the interface half of internal/logger)
//	version  — build/version metadata (today: internal/version)
//
// Shared imports nothing above it: no Domain, no Application, no
// Infrastructure, no Presentation. If a primitive needs upward knowledge, it
// belongs in that outer layer instead.
//
// This file exists so that the package is classified by internal/arch from
// day one. Relocation is tracked under WBS 14.3.x.
package shared
