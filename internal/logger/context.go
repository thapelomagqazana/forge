package logger

import "context"

// contextKey is the context key type for the Logger.
//
// It is unexported so that no other package can collide with it.
// The caller uses IntoContext and FromContext; the key is an
// implementation detail.
type contextKey struct{}

// decisionContextKey is the context key type for the resolved
// log-level Decision.
//
// It is distinct from contextKey so that the logger and the
// decision are two independent context values. A caller may
// carry either, both, or neither:
//
//   - A context with a Decision but no Logger is a context whose
//     level has been resolved but whose logger has not yet been
//     constructed. The CLI's boundary (runWithOptions) produces
//     such a context; the PersistentPreRunE hook consumes it.
//
//   - A context with a Logger but no Decision is a context whose
//     logger has been constructed from a default level (or from
//     a decision that the consumer did not need to record). The
//     CLI's hook produces such a context for downstream commands.
//
//   - A context with both is the state after the hook runs: the
//     logger is bound to the decision's level, and the decision
//     is still readable for diagnostics.
//
//   - A context with neither is a context with no logging
//     configuration; the package-level Debug/Info/Warn/Error
//     functions fall back to Nop.
//
// The two keys are unexported for the same reason as contextKey:
// no other package can collide with them, and the accessors
// (ContextWithDecision, DecisionFromContext) are the only
// sanctioned read/write paths.
type decisionContextKey struct{}

// IntoContext returns a new context with l stored under the
// package's context key.
//
// # Usage
//
// A caller at an application boundary (typically a CLI hook, an
// HTTP middleware, or a worker's start) stores the logger:
//
//	ctx = logger.IntoContext(ctx, logger.New(opts))
//
// Downstream code retrieves it with FromContext. The pattern
// avoids threading the logger through every function signature.
//
// # Why the helper is the only writer
//
// The context key is unexported; only IntoContext can write it.
// The asymmetry (any code can read; only IntoContext can write) is
// deliberate: a subsystem that could overwrite the logger could
// hide records from the invocation's intended sink.
//
// # Composition with the correlation ID
//
// The logger stored by IntoContext carries whatever correlation ID
// the context already holds. If the context is later wrapped by
// WithCorrelationID, the previously stored logger does not see
// the new ID. Callers should establish the correlation ID
// *before* calling IntoContext; the CLI does (see
// internal/cli/hooks.go).
func IntoContext(ctx context.Context, l Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext returns the Logger stored in ctx, or Nop() if no
// Logger is present.
//
// # Why Nop and not nil
//
// Returning nil would require every caller to check for nil;
// returning Nop() lets callers write:
//
//	logger.FromContext(ctx).Info("hello")
//
// without a nil check. The Nop logger is allocation-free and safe.
//
// # Correlation ID propagation
//
// FromContext does not itself read the correlation ID. It returns
// the logger stored by IntoContext, which is already bound to
// whatever ID the context held at storage time. A caller who
// wants a logger bound to a *newly* added ID must call
// IntoContext again with a logger constructed from the new
// context.
//
// In practice this is not needed: the CLI establishes the
// correlation ID once, before IntoContext, and never changes it
// during the invocation.
func FromContext(ctx context.Context) Logger {
	if ctx == nil {
		return Default()
	}
	v := ctx.Value(contextKey{})
	if v == nil {
		return Default()
	}
	l, ok := v.(Logger)
	if !ok || l == nil {
		return Default()
	}
	return l
}

// ContextWithDecision returns a new context with the log-level
// Decision stored under the package's decision key.
//
// # What the helper is for
//
// The CLI's boundary (runWithOptions) resolves the log level
// before the command tree runs, so that the command tree's
// PersistentPreRunE hook can construct the invocation's logger at
// the correct level. The resolver produces a Decision; the hook
// needs it. The Decision travels on the context, under this key.
//
// The transport is per-invocation: each call to Run creates a new
// context, attaches the decision, and passes the context to
// ExecuteContext. Concurrent invocations do not share state.
//
// # Why the Decision travels on the context and not on a struct
//
// The alternative — attaching the Decision to a struct that is
// captured by the hook's closure — does not work when the
// resolver runs after the struct is constructed. In the CLI's
// flow, buildDependencies constructs the Dependencies struct (and
// newRootCmd captures it in the hook's closure) before the
// resolver runs. A write to the struct's field after construction
// is a write to the *local copy* in runWithOptions, not to the
// captured copy in the hook's closure; the two are different
// memory.
//
// The context is the only transport that survives the ordering:
// it is created and passed at the call site, and it is threaded
// through every layer between the resolver and the hook
// (ExecuteContext stores it on the command; the hook reads it via
// cmd.Context()). No struct capture is involved.
//
// # Composition with IntoContext
//
// The two helpers are independent. A caller may attach a decision
// without attaching a logger (the CLI's boundary does this) or
// attach a logger without attaching a decision (a test that wants
// a logger but does not care about the level). The two keys are
// distinct; the reads are independent.
//
// In the CLI's flow, the boundary attaches the decision first,
// then ExecuteContext runs, then the hook reads the decision and
// attaches a logger. A downstream caller that reads the context
// after the hook sees both.
func ContextWithDecision(ctx context.Context, d Decision) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, decisionContextKey{}, d)
}

// DecisionFromContext returns the log-level Decision stored in
// ctx, or the default decision (LevelInfo, SourceDefault) and
// false if no Decision is present.
//
// # The two-result form
//
// The boolean distinguishes "the decision is the default" from
// "the decision is absent". A caller that does not distinguish
// the two — and uses the default in both cases — sees the same
// behavior either way. A caller that wants to know whether the
// boundary resolved a level (the CLI's hook, for example) uses
// the boolean to branch.
//
// # The default value
//
// The default is Decision{Level: LevelInfo, Source:
// SourceDefault}: the same values the resolver produces for an
// invocation with no flags, no env var, and no config file. It is
// the CLI's declared default level (see
// docs/reference/log-levels.md §1.2), and it is the safe value
// for a caller that reads the decision without one having been
// stored.
//
// # Why the helper exists
//
// The CLI's PersistentPreRunE hook (initializeCommandEnvironment,
// internal/cli/hooks.go) constructs the invocation's logger. The
// logger's level is the resolved decision's Level. The hook reads
// the decision with this function, falling back to the
// environment-derived level when the decision is absent (a
// caller that invokes the hook outside runWithOptions).
func DecisionFromContext(ctx context.Context) (Decision, bool) {
	if ctx == nil {
		return Decision{Level: LevelInfo, Source: SourceDefault}, false
	}
	v := ctx.Value(decisionContextKey{})
	if v == nil {
		return Decision{Level: LevelInfo, Source: SourceDefault}, false
	}
	d, ok := v.(Decision)
	if !ok {
		return Decision{Level: LevelInfo, Source: SourceDefault}, false
	}
	return d, true
}
