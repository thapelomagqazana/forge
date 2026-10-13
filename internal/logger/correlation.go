package logger

import (
	"context"
	"crypto/rand"
	"fmt"
)

// correlationIDContextKey is the context key under which the
// invocation's correlation ID is stored.
//
// The key is an unexported type so that no other package can
// collide with it. External packages use CorrelationIDFromContext
// and WithCorrelationID; they must not reach into the context
// directly.
type correlationIDContextKey struct{}

// ZeroCorrelationID is the correlation ID used when a record is
// emitted outside a command invocation (for example, during
// package initialization).
//
// # Shape
//
// The sentinel is a syntactically valid v4 UUID whose value is
// reserved for the sentinel:
//
//	00000000-0000-4000-8000-000000000000
//
// Every v4 UUID has the shape
//
//	xxxxxxxx-xxxx-4xxx-[89ab]xxx-xxxxxxxxxxxx
//
// and the sentinel satisfies it. A consumer that parses a
// correlation_id with a v4-aware parser therefore succeeds on
// every value the logger emits, including the sentinel; the
// sentinel is recognized by its specific value, not by a
// version-nibble mismatch.
//
// # Why not an all-zero UUID
//
// An all-zero UUID ("00000000-0000-0000-0000-000000000000") is a
// valid UUID in the broader sense (it is the "nil UUID" of RFC
// 4122) but not a v4 UUID: its version nibble is 0. A consumer
// that validates correlation_id as a v4 UUID rejects the nil
// UUID and must special-case it. The package's contract is that
// a consumer can validate every correlation_id uniformly; the
// reserved-v4 sentinel upholds that contract.
//
// # Collision
//
// NewCorrelationID produces uniformly random v4 UUIDs. The
// probability that a real invocation's ID equals the sentinel is
// 2^-122, which is negligible. A consumer that wants to be
// certain treats the sentinel as reserved regardless of
// probability: the sentinel is the only v4 UUID whose low-order
// bits are all zero except for the version and variant nibbles,
// and a producer would have to construct it deliberately.
const ZeroCorrelationID = "00000000-0000-4000-8000-000000000000"

// NewCorrelationID returns a fresh UUID v4 as a canonical
// lowercase string (36 characters, hyphenated).
//
// The generator uses crypto/rand, not math/rand, because the ID is
// used for correlation across process boundaries (log aggregation,
// distributed tracing). Predictable IDs would let one invocation's
// records be spoofed by another. The performance cost is
// negligible: one ID is generated per invocation.
//
// The function panics if crypto/rand fails. A failure of the
// system's entropy source is not a recoverable condition for a
// process that cannot produce a correlation ID.
func NewCorrelationID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf(
			"logger: cannot generate correlation ID: %v", err))
	}
	// Set the version (4) and variant (RFC 4122) bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	// Encode as 8-4-4-4-12 hex, lowercase.
	const hexChars = "0123456789abcdef"
	var buf [36]byte
	dst := buf[:]
	j := 0
	for i, c := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			dst[j] = '-'
			j++
		}
		dst[j] = hexChars[c>>4]
		dst[j+1] = hexChars[c&0x0f]
		j += 2
	}
	return string(buf[:])
}

// WithCorrelationID returns a context that carries id.
//
// The ID is retrieved by CorrelationIDFromContext. The function
// does not validate id; callers that need validation should use
// NewCorrelationID, which always produces a valid v4 UUID.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, correlationIDContextKey{}, id)
}

// CorrelationIDFromContext returns the correlation ID stored in
// ctx, or ZeroCorrelationID if none is present.
//
// The zero value is a sentinel, not a valid invocation ID. A
// consumer may use it to detect records that were emitted outside
// an invocation.
//
// # Why this is not named FromContext
//
// FromContext is already the package's accessor for the Logger
// (see context.go). Go has one package-level namespace, so two
// functions cannot share the name. The correlation-ID accessor
// uses the longer name to disambiguate; the logger accessor keeps
// the shorter one because it is the more commonly called of the
// two.
//
// # Nil-context safety
//
// A nil ctx returns ZeroCorrelationID. The guard is the same
// pattern used by logger.FromContext; it makes the accessor safe
// to call from a deferred cleanup that may run after a context
// has been cleared.
func CorrelationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ZeroCorrelationID
	}
	if id, ok := ctx.Value(correlationIDContextKey{}).(string); ok {
		return id
	}
	return ZeroCorrelationID
}
