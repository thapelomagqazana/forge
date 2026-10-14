package application

import "context"

// Service is the minimal contract every use-case service in Forge satisfies.
//
// It exists so that composition code (and tests) can treat services
// uniformly, and so that the "one service per command" rule has a name.
//
// The generic parameters keep the contract type-safe without forcing services
// to share an Input/Output shape. Most services will be instantiated as
// Service[CheckInput, CheckOutput].
//
// # Why generics and not `any`
//
// Using `any` would push type assertions into the composition root and into
// tests, which is exactly the kind of cast-heavy code that hides bugs. With
// generics, a wiring mistake is a compile error.
//
// # Context
//
// Run MUST honor ctx cancellation. It MUST NOT derive a context that outlives
// ctx. It MUST NOT call context.Background() internally.
type Service[In any, Out any] interface {
	// Run executes the use case. It is safe for concurrent use by multiple
	// goroutines if and only if the concrete service is documented as such;
	// by default services are not assumed to be concurrency-safe and are
	// called once per command invocation.
	//
	// Implementations must return a non-nil error on failure and a zero Out
	// on error. They must not return both a zero error and a zero Out for
	// success unless the use case legitimately produces no output.
	Run(ctx context.Context, in In) (Out, error)
}
