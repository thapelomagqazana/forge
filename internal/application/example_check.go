package application

import (
	"context"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// CheckInput is the input to CheckService.
//
// It is a value type so that a service cannot mutate caller state through
// the input it is handed. Every field is a scalar or an immutable value; a
// slice or map field would reintroduce aliasing and is deliberately absent.
type CheckInput struct {
	// Root is the project root directory the check operates on. It must be
	// non-empty; an empty Root is rejected by CheckService.Run as a usage
	// error (CategoryCLI, CodeCLIInvalidArg), because it reflects a caller
	// contract violation, not a domain condition.
	Root string

	// Strict enables stricter validation. Its semantics are owned by the
	// domain; the application layer only forwards it. The zero value
	// (false) is the lenient mode.
	Strict bool
}

// CheckOutput is the result of CheckService.
//
// It reports domain findings without assuming any particular presentation.
// It carries no exit code, no ANSI, no JSON tags: mapping findings to a
// process exit status is the presentation layer's responsibility (ADR-019,
// ADR-040 §2.1).
type CheckOutput struct {
	// Findings is the ordered list of issues discovered. An empty or nil
	// slice means the check found nothing. The order is domain-defined;
	// the application layer preserves it.
	Findings []Finding
}

// Finding is a single domain finding.
//
// It is deliberately presentation-neutral: no ANSI, no JSON tags, no exit
// codes. Presentation maps it to whatever the user sees.
type Finding struct {
	// Path is the file or directory the finding applies to, relative to
	// CheckInput.Root. Absolute paths are a domain bug: they would leak
	// the host's filesystem layout into output.
	Path string

	// Message is a human-readable description of the finding. It is
	// produced by the domain, not by the application layer, and must not
	// contain user secrets (see ADR-016 and ADR-033).
	Message string

	// Severity is one of "info", "warn", "error". It is a string rather
	// than a closed enum so the domain can add severities without a
	// breaking change to this package; validation lives in the domain.
	Severity string
}

// CheckPort is the domain port CheckService depends on.
//
// It is declared here because the *consumer* (this service) owns the
// interface it needs, per the interface-ownership rule in ADR-040 §4. If
// the interface lived in the infrastructure package that implements it,
// Application would have to import Infrastructure to name it, inverting
// the dependency arrow.
//
// The interface is deliberately the smallest one CheckService can use.
// Larger interfaces are harder to fake and drift toward god-interfaces.
//
// Implementations must:
//
//   - honor ctx cancellation and return ctx.Err() unwrapped on cancellation;
//   - return a *forgeerr.Error (or one wrapped by it) on failure, so that
//     the service can propagate the category unchanged;
//   - never return a non-nil error together with a non-nil findings slice.
type CheckPort interface {
	// Check runs the domain check over root and returns findings.
	Check(ctx context.Context, root string, strict bool) ([]Finding, error)
}

// CheckService is the canonical example service, and the template every
// future service in this package should follow.
//
// Construction is via NewCheckService, which panics on a nil port. A nil
// port is a wiring bug, and failing fast at construction gives a stack
// trace that names the composition root rather than a nil dereference deep
// inside a request handler.
type CheckService struct {
	port CheckPort
}

// NewCheckService constructs a CheckService.
//
// It panics if port is nil. The panic message names the dependency so that
// the failure is unambiguous in a stack trace from the composition root.
func NewCheckService(port CheckPort) *CheckService {
	if port == nil {
		panic("application: NewCheckService: port is nil")
	}
	return &CheckService{port: port}
}

// Run executes the check use case.
//
// # Error contract
//
//   - If ctx is already canceled or has expired, Run returns ctx.Err()
//     unwrapped. context.Canceled and context.DeadlineExceeded carry their
//     own meaning and are not classified as Forge errors; the CLI decides
//     how to surface them (see ADR-019 and ADR-028).
//
//   - If in.Root is empty, Run returns a *forgeerr.Error in CategoryCLI
//     with code CodeCLIInvalidArg. An empty root is a caller contract
//     violation, not a domain condition; classifying it as a check
//     failure would mislead scripts that branch on the process exit code.
//
//   - If the port returns an error, Run returns it wrapped so that the
//     presentation layer can map its category to an exit code without
//     inspecting the port's concrete type. The wrapping is *category-
//     preserving*: if the port already returned a *forgeerr.Error,
//     forgeerr.Wrap returns it unchanged (see constructors.go). If the
//     port returned a bare error, Run classifies it as CategoryInternal
//     with code CodeInternalUnexpected, because the registry's taxonomy
//     has no "other" member and an unclassified failure is, by
//     definition, an internal one.
//
//   - Run never returns a non-nil error together with a non-zero
//     CheckOutput. Callers may rely on this: on error, the output is the
//     zero value.
func (s *CheckService) Run(ctx context.Context, in CheckInput) (CheckOutput, error) {
	if err := ctx.Err(); err != nil {
		return CheckOutput{}, err
	}

	if in.Root == "" {
		// Caller contract violation. CategoryCLI is the correct
		// category: the failure is about how the command was
		// invoked, not about the domain the command operates on.
		// The code is registered (codes.go) and its prefix matches
		// the category, so New does not panic.
		return CheckOutput{}, forgeerr.New(
			forgeerr.CategoryCLI,
			forgeerr.CodeCLIInvalidArg,
			"check: root must not be empty",
		)
	}

	findings, err := s.port.Check(ctx, in.Root, in.Strict)
	if err != nil {
		// If the port already classified the error, Wrap returns
		// it unchanged — the classification is preserved and the
		// caller sees the port's code. If the port returned a
		// bare error, Wrap attaches the fallback classification.
		//
		// The message is fixed by this call site, not derived
		// from err, so that the user-facing message is stable
		// across port implementations. The cause retains err, so
		// errors.Is and errors.As still traverse to it.
		var classified *forgeerr.Error
		if errorsAs(err, &classified) {
			// Port already classified. Return as-is; the
			// application layer does not reclassify.
			return CheckOutput{}, err
		}
		return CheckOutput{}, forgeerr.Wrap(
			forgeerr.CategoryInternal,
			forgeerr.CodeInternalUnexpected,
			"check: port returned an unclassified error",
			err,
		)
	}

	return CheckOutput{Findings: findings}, nil
}

// errorsAs is a local alias for errors.As, kept private so that this file's
// import set stays minimal. The alias is not a micro-optimisation: the
// compiler emits the same call. It exists so a reader sees "we are
// checking whether the error chain already carries a *forgeerr.Error"
// at the call site, rather than having to recall the shape of errors.As.
func errorsAs(err error, target **forgeerr.Error) bool {
	for err != nil {
		if e, ok := err.(*forgeerr.Error); ok {
			*target = e
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// Compile-time proof that CheckService satisfies the Service contract
// defined in service.go. If the contract changes incompatibly, this line
// fails to compile — the earliest possible signal.
var _ Service[CheckInput, CheckOutput] = (*CheckService)(nil)
