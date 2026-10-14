// Package domain is the root of Forge's pure business logic.
//
// It will be split into bounded contexts (one sub-package each):
//
//	internal/domain/blueprint
//	internal/domain/template
//	internal/domain/component
//	internal/domain/policy
//	internal/domain/foundation
//
// Domain code:
//
//   - defines the interfaces it needs (consumer-owned, per ADR-040);
//   - imports only internal/shared and the standard library;
//   - never imports internal/infrastructure, internal/application, or
//     internal/cli.
//
// This file exists so that the package is classified by internal/arch from
// day one. The bounded contexts are added in Phase 3 (WBS 14.2.x).
package domain
