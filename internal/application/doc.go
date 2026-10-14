// Package application contains Forge's use-case orchestration layer.
//
// # Responsibility
//
// This layer is where a single user-facing operation is turned into a sequence
// of domain calls. Each command in the CLI corresponds to exactly one service
// in this package (or in a sub-package of it). A service:
//
//   - is a plain struct with injected dependencies (all interfaces);
//   - exposes a Run(ctx, Input) (Output, error) method;
//   - performs no I/O directly; it calls domain interfaces;
//   - returns errors unchanged (wrapping allowed per ADR-013).
//
// # Architectural placement
//
//	Presentation  ──imports──▶  Application  ──imports──▶  Domain
//	                                   │                        ▲
//	                                   │ (interfaces only)      │
//	                                   ▼                        │
//	                            Infrastructure ───implements────┘
//
// Application MUST NOT import internal/infrastructure or internal/cli.
// Concrete adapters are wired by the composition root in internal/cli.
//
// # Interface ownership
//
// "Consumer defines, implementer implements." If a service needs to read the
// filesystem, it declares the narrow interface it needs (or reuses one declared
// by the domain) and accepts it via constructor injection. Infrastructure
// provides the concrete type.
//
// # Testing
//
// Because every dependency is an interface, services are tested with fakes.
// See example_check_test.go for the canonical pattern: a fake domain port, a
// table-driven suite covering positive, negative, edge, and boundary cases, and
// no use of the real filesystem or clock.
//
// # Stability
//
// This package is internal. Its API may evolve; the layer contract is
// versioned by ADR-040.
package application
