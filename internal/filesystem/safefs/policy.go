package safefs

import (
	"errors"
	"fmt"

	"github.com/thapelomagqazana/forge/internal/filesystem"
)

// This file defines the configuration surface of a Root: the
// SymlinkPolicy enum, the functional Option type, the unexported
// config struct that carries validated settings, and the With*
// functions that mutate it.
//
// The configuration is applied exactly once, in New. After
// construction, a Root is immutable. This is a deliberate choice:
// a Root whose policy can change at runtime is a Root whose
// invariant cannot be reasoned about statically, and the
// enforcement-by-construction model (ADR-035) depends on the
// invariant holding for the Root's entire lifetime.

// -----------------------------------------------------------------------------
// SymlinkPolicy
// -----------------------------------------------------------------------------

// SymlinkPolicy controls how a Root treats symbolic links
// encountered during name resolution.
//
// The set of policies is closed. Adding a policy requires an ADR
// and a corresponding test matrix; removing or renumbering one is a
// breaking change. This mirrors the closure argument for the error
// category taxonomy (ADR-011) and the redaction policy (ADR-034):
// a closed set is auditable, an open set is not.
type SymlinkPolicy uint8

const (
	// NoFollow rejects any path that traverses a symbolic link.
	//
	// NoFollow is the default. It is the safest choice for a
	// foundation manager that never intends to read outside its
	// root, and it is the only policy under which the root
	// confinement invariant holds without inspecting the link
	// target.
	//
	// A name that resolves through a symlink produces an error
	// wrapping ErrSymlinkDenied.
	NoFollow SymlinkPolicy = iota

	// FollowInRoot allows symbolic links whose resolved target
	// remains inside the Root. Targets outside the Root are
	// rejected with an error wrapping ErrSymlinkDenied.
	//
	// FollowInRoot is intended for project layouts that use
	// symlinks internally — for example, a template that links
	// to a shared directory elsewhere in the same project.
	// It is never appropriate for names derived from untrusted
	// input.
	//
	// FollowInRoot performs one level of resolution: if the
	// target is itself a symlink, resolution continues until a
	// non-symlink is reached or the root is escaped. Chains
	// longer than MaxSymlinkChain are rejected with
	// ErrSymlinkDenied to bound the work.
	FollowInRoot

	// FollowAnywhereForbidden is a documentation alias for
	// NoFollow. It exists so call sites can express intent
	// explicitly — "we have considered symlinks and we forbid
	// following them" — without relying on the reader's memory of
	// what the default is.
	//
	// The numerical value is identical to NoFollow. The two are
	// interchangeable in comparisons and in errors.Is checks
	// against the resulting errors.
	FollowAnywhereForbidden = NoFollow
)

// String implements fmt.Stringer for diagnostics.
//
// The returned strings are stable: they are logged, asserted in
// tests, and may appear in error messages. Renaming one is a
// breaking change, even though the enum value is unchanged.
func (p SymlinkPolicy) String() string {
	switch p {
	case NoFollow:
		return "no-follow"
	case FollowInRoot:
		return "follow-in-root"
	default:
		// Do not panic on an unknown value. A policy that was
		// constructed from an out-of-range integer is a
		// programming error, but it must not crash the process;
		// rendering it as a number is the least-surprising
		// behaviour and it is greppable in logs.
		return fmt.Sprintf("symlink-policy(%d)", uint8(p))
	}
}

// Validate reports whether p is a recognised policy.
//
// Validate exists so that configuration loaders — which may read a
// policy from a YAML file — can reject an unknown value before it
// reaches New. New itself does not call Validate; it trusts its
// caller. Callers that accept policy strings from outside the
// process must call Validate first.
func (p SymlinkPolicy) Validate() error {
	switch p {
	case NoFollow, FollowInRoot:
		return nil
	default:
		return fmt.Errorf("%w: unknown symlink policy %d",
			ErrInvalidOption, uint8(p))
	}
}

// -----------------------------------------------------------------------------
// Option
// -----------------------------------------------------------------------------

// Option configures a Root at construction time.
//
// Options are applied in order. Later options override earlier ones
// for scalar settings; the order of application is visible to the
// caller and is the caller's responsibility to get right. There is
// no "last write wins" ambiguity hidden inside New.
//
// An Option has no return value. This is deliberate: an Option that
// can fail would force New to collect and report errors, which in
// turn would force callers to write error handling for cases that
// can be caught by validating the inputs to the With* functions
// instead. The With* functions do not validate; validation happens
// in New against the assembled config, so error reporting is
// centralised in one place.
type Option func(*config)

// -----------------------------------------------------------------------------
// config
// -----------------------------------------------------------------------------

// config is the assembled, unvalidated settings for a Root.
//
// config is never exported. It exists so that Option can be a
// function on an unexported type, which prevents callers from
// constructing an Option that reaches into Root's internals.
//
// A config is created with defaultConfig, mutated by the With*
// functions, and consumed by New, which validates it and copies the
// relevant fields into the Root. After New returns, the config is
// discarded; no Root holds a reference to it.
type config struct {
	// fs is the underlying filesystem. When nil, New constructs
	// an OSFS around the resolved root. Tests supply a
	// fstest.MapFS or an in-memory adapter via WithFS.
	fs filesystem.FS

	// policy is the symlink policy. Defaults to NoFollow.
	policy SymlinkPolicy

	// maxDepth bounds recursive walks. Zero disables the bound.
	// Defaults to DefaultMaxDepth.
	maxDepth int

	// maxEntries bounds the total number of entries visited by a
	// walk. Zero disables the bound. Defaults to DefaultMaxEntries.
	maxEntries int

	// maxSymlinkChain bounds the number of symlink hops followed
	// under FollowInRoot. Zero disables the bound. Defaults to
	// DefaultMaxSymlinkChain.
	//
	// This is separate from maxDepth: depth bounds directory
	// nesting, chain bounds link-following. A pathological
	// layout can exceed either without exceeding the other.
	maxSymlinkChain int

	// detectCaseCollisions enables case-collision detection. It is
	// off by default and should be turned on for macOS and
	// Windows, where the filesystem is case-insensitive.
	detectCaseCollisions bool

	// denyPatterns is the effective deny-list. It is set by New
	// from the defaults, or replaced by WithDenyList, or
	// extended by WithAdditionalDenyPatterns.
	denyPatterns []string

	// additionalDenyPatterns accumulates patterns added by
	// WithAdditionalDenyPatterns. It is appended to
	// denyPatterns in New.
	additionalDenyPatterns []string

	// denyExplicit is true if WithDenyList was called. It is
	// used to distinguish "the caller wants the defaults" from
	// "the caller wants an empty list".
	denyExplicit bool

	// allowPatterns is the allow-list. It is empty by default,
	// which means "no allow-list; the deny-list is the only
	// filter".
	allowPatterns []string

	// allowExplicit is true if WithAllowList was called. It is
	// used to distinguish "the caller wants no allow-list" from
	// "the caller wants an empty allow-list" (which would
	// reject every name).
	allowExplicit bool
}

// Defaults. These are exported as constants so that documentation,
// tests, and configuration loaders can refer to them by name rather
// than by literal.
const (
	// DefaultMaxDepth is the default bound on recursive walk
	// depth. It is generous enough for any project layout a human
	// would create and small enough to bound the cost of a
	// pathological tree.
	DefaultMaxDepth = 64

	// DefaultMaxEntries is the default bound on the total number
	// of entries a single walk may visit. One million entries is
	// far more than any Forge project; the bound exists to fail
	// closed on a hostile tree rather than exhaust memory.
	DefaultMaxEntries = 1 << 20

	// DefaultMaxSymlinkChain is the default bound on the number
	// of symbolic links followed in a single resolution under
	// FollowInRoot. The value is chosen to be well above any
	// legitimate chain and well below the point at which
	// resolution cost becomes a denial-of-service vector.
	DefaultMaxSymlinkChain = 40
)

// WithDenyList replaces the default deny-list with the given
// patterns.
//
// # When to use this
//
// A caller that needs a Root with no deny-list (for example, a
// test that writes to .git/config deliberately to verify that
// the operation is rejected) uses WithDenyList() with no
// arguments. A caller that needs a custom deny-list uses
// WithDenyList(patterns...).
//
// # Why this replaces rather than extends
//
// The semantics of "With" options in this package are
// "replace". The extending variant is named explicitly:
// WithAdditionalDenyPatterns. The asymmetry is deliberate: a
// caller who wants the safer behaviour (extend) uses the
// longer name, and a caller who wants to remove a default
// uses the shorter name, which reads as "I am taking
// responsibility for the full list."
func WithDenyList(patterns ...string) Option {
	return func(c *config) {
		c.denyPatterns = patterns
		c.denyExplicit = true
	}
}

// WithAdditionalDenyPatterns extends the deny-list with the
// given patterns. The defaults remain in effect.
//
// This is the option most callers use. It is idempotent: adding
// a pattern that is already present is a no-op at match time.
func WithAdditionalDenyPatterns(patterns ...string) Option {
	return func(c *config) {
		c.additionalDenyPatterns = append(c.additionalDenyPatterns, patterns...)
	}
}

// WithAllowList constrains the Root to the given patterns. A
// name that does not match any pattern is rejected with
// ErrNotAllowed.
//
// # When to use this
//
// The allow-list is the primary control for components (Phase
// 12). A component's manifest declares the paths it needs to
// write; the Root is constructed with the manifest's patterns
// as its allow-list.
//
// # Interaction with the deny-list
//
// The deny-list always wins. A name that matches the allow-list
// and the deny-list is rejected with ErrDenyList, not with
// ErrNotAllowed. This makes the deny-list a lower bound on the
// permitted set: no allow-list can widen it.
func WithAllowList(patterns ...string) Option {
	return func(c *config) {
		c.allowPatterns = patterns
		c.allowExplicit = true
	}
}

// defaultConfig returns the settings a Root has when no Options are
// supplied.
//
// The defaults are the safest settings, not the most permissive
// ones. A caller who forgets to configure a Root still gets
// NoFollow, bounded walks, and case-sensitive name handling. This
// is the opposite of the "convenience by default" convention that
// many libraries adopt, and it is deliberate: the failure mode of a
// permissive default is a security incident, and the failure mode
// of a strict default is a clear error message.
func defaultConfig() config {
	return config{
		policy:          NoFollow,
		maxDepth:        DefaultMaxDepth,
		maxEntries:      DefaultMaxEntries,
		maxSymlinkChain: DefaultMaxSymlinkChain,
	}
}

// -----------------------------------------------------------------------------
// With* functions
// -----------------------------------------------------------------------------

// WithFS injects the underlying filesystem.
//
// WithFS is intended for tests, which supply an in-memory
// implementation such as fstest.MapFS, and for embedders that wish
// to back a Root with something other than the operating system's
// filesystem. Production code does not call WithFS; New constructs
// an OSFS automatically when none is supplied.
//
// Passing a nil FS is a programming error. New detects it and
// returns an error wrapping ErrInvalidOption rather than
// dereferencing nil later.
//
// WithFS is a full replacement, not a decorator: it is the caller's
// responsibility to ensure the supplied FS is bound to the same
// directory as the Root. In tests this is a non-issue; in
// production, do not call WithFS.
func WithFS(fs filesystem.FS) Option {
	return func(c *config) {
		c.fs = fs
	}
}

// WithSymlinkPolicy sets the symlink policy. See SymlinkPolicy for
// the semantics of each value.
//
// The default is NoFollow. WithSymlinkPolicy is the only way to
// change it. An unknown policy value is caught by New, which
// returns an error wrapping ErrInvalidOption.
func WithSymlinkPolicy(p SymlinkPolicy) Option {
	return func(c *config) {
		c.policy = p
	}
}

// WithMaxDepth sets the bound on recursive walk depth.
//
// Zero means unbounded. A negative value is a programming error and
// is caught by New, which returns an error wrapping
// ErrInvalidOption. Callers that want the default should not call
// WithMaxDepth; callers that want a specific bound should pass a
// positive integer.
//
// The bound is enforced by Walk. It is a security control: a
// hostile tree with deep nesting must not cause unbounded
// recursion. Callers who need to walk deeper than
// DefaultMaxDepth should pass an explicit bound and document why.
func WithMaxDepth(n int) Option {
	return func(c *config) {
		c.maxDepth = n
	}
}

// WithMaxEntries sets the bound on the total number of entries a
// single walk may visit.
//
// Zero means unbounded. A negative value is a programming error and
// is caught by New, which returns an error wrapping
// ErrInvalidOption.
//
// The bound is enforced by Walk. Like WithMaxDepth, it is a
// security control. The units are directory entries, counted across
// the entire walk, not per directory.
func WithMaxEntries(n int) Option {
	return func(c *config) {
		c.maxEntries = n
	}
}

// WithMaxSymlinkChain sets the bound on the number of symbolic
// links followed in a single resolution.
//
// WithMaxSymlinkChain has no effect when the policy is NoFollow.
// It is meaningful only under FollowInRoot, where a chain of links
// can produce a cycle or a pathologically long resolution.
//
// Zero means unbounded. A negative value is a programming error and
// is caught by New. The default is DefaultMaxSymlinkChain.
func WithMaxSymlinkChain(n int) Option {
	return func(c *config) {
		c.maxSymlinkChain = n
	}
}

// WithCaseCollisionDetection enables or disables detection of names
// that collide under Unicode case folding.
//
// Detection should be enabled on case-insensitive filesystems
// (macOS, Windows) so that a project containing both "README.md"
// and "readme.md" is reported as a collision rather than silently
// resolved to one of the two by the operating system.
//
// Detection is off by default. Enabling it on a case-sensitive
// filesystem is harmless but pointless; the check never fires
// because no two distinct names can differ only by case.
//
// The check is performed by Walk and by ReadDir. It is not
// performed by Open or Stat, which operate on a single name and
// therefore have nothing to collide with.
func WithCaseCollisionDetection(enabled bool) Option {
	return func(c *config) {
		c.detectCaseCollisions = enabled
	}
}

// -----------------------------------------------------------------------------
// Validation
// -----------------------------------------------------------------------------

// validate checks the assembled config and returns the first
// problem found.
//
// validate is called by New after all Options have been applied. It
// is not called by the With* functions, so a With* call never
// fails; the failure surfaces at New with a message that names the
// setting. This keeps error reporting in one place and keeps the
// With* surface free of error returns, which is a common source of
// awkward call sites.
//
// validate never inspects the filesystem. It is pure: it depends
// only on the config's fields.
func (c *config) validate() error {
	if err := c.policy.Validate(); err != nil {
		return err
	}
	if c.maxDepth < 0 {
		return fmt.Errorf("%w: MaxDepth must not be negative, got %d",
			ErrInvalidOption, c.maxDepth)
	}
	if c.maxEntries < 0 {
		return fmt.Errorf("%w: MaxEntries must not be negative, got %d",
			ErrInvalidOption, c.maxEntries)
	}
	if c.maxSymlinkChain < 0 {
		return fmt.Errorf("%w: MaxSymlinkChain must not be negative, got %d",
			ErrInvalidOption, c.maxSymlinkChain)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Errors local to option handling
// -----------------------------------------------------------------------------

// ErrInvalidOption is returned by New when an Option value is
// rejected. It is not a filesystem error; it is a programmer error
// surfaced as early as possible.
//
// ErrInvalidOption is deliberately separate from the sentinels in
// errors.go. Those describe conditions encountered while operating
// on a filesystem; ErrInvalidOption describes a mistake in the
// caller's configuration. Mixing the two would force the CLI to
// distinguish them by inspecting the message, which is exactly the
// string matching that ADR-011 exists to prevent.
var ErrInvalidOption = errors.New("safefs: invalid option")
