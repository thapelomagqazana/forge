package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// WBS 9.2.2 — Source attribution semantics.
//
// The Source type itself is frozen by WBS 8.1.2 and tested in
// precedence_test.go. These tests pin the read/write pair that
// WBS 9.2.2 adds to Config: SourceFor reads, setSource writes.
// ---------------------------------------------------------------------------

// TestConfig_SourceFor_ZeroValue asserts that a zero-value Config
// reports SourceDefault for any field, including unknown ones.
func TestConfig_SourceFor_ZeroValue(t *testing.T) {
	t.Parallel()

	var c Config // zero value

	assert.Equal(t, SourceDefault, c.SourceFor("LogLevel"))
	assert.Equal(t, SourceDefault, c.SourceFor("OutputFormat"))
	assert.Equal(t, SourceDefault, c.SourceFor("UnknownField"))
}

// TestConfig_SourceFor_RecordedSource asserts that a recorded
// source is returned by SourceFor.
func TestConfig_SourceFor_RecordedSource(t *testing.T) {
	t.Parallel()

	c := Config{}
	c.setSource("LogLevel", SourceFile)

	assert.Equal(t, SourceFile, c.SourceFor("LogLevel"))
	// Unrecorded fields still return SourceDefault.
	assert.Equal(t, SourceDefault, c.SourceFor("OutputFormat"))
}

// TestConfig_SourceFor_LastWriteWins asserts that calling setSource
// twice for the same field records the second call. The loader
// relies on this: it calls setSource in ascending precedence order,
// so the last call is the highest-priority one.
func TestConfig_SourceFor_LastWriteWins(t *testing.T) {
	t.Parallel()

	c := Config{}
	c.setSource("LogLevel", SourceDefault)
	c.setSource("LogLevel", SourceFile)
	c.setSource("LogLevel", SourceEnv)
	c.setSource("LogLevel", SourceFlag)

	assert.Equal(t, SourceFlag, c.SourceFor("LogLevel"))
}

// TestConfig_SourceFor_UnknownFieldIsDefault asserts the documented
// semantics for unknown field names.
func TestConfig_SourceFor_UnknownFieldIsDefault(t *testing.T) {
	t.Parallel()

	c := Config{}
	c.setSource("LogLevel", SourceFile)

	assert.Equal(t, SourceDefault, c.SourceFor("NoSuchField"),
		"unknown field names must return SourceDefault")
}

// TestConfig_SourceFor_PerFieldIndependence asserts that recording
// a source for one field does not affect another. The loader sets
// sources field by field; a bug that used a shared key would be
// caught here.
func TestConfig_SourceFor_PerFieldIndependence(t *testing.T) {
	t.Parallel()

	c := Config{}
	c.setSource("LogLevel", SourceEnv)
	c.setSource("OutputFormat", SourceFile)

	assert.Equal(t, SourceEnv, c.SourceFor("LogLevel"))
	assert.Equal(t, SourceFile, c.SourceFor("OutputFormat"))
}

// TestConfig_SourceFor_DoesNotMutateConfig is a non-functional
// test: reading a source must not alter the Config. The method has
// a value receiver, so this is guaranteed by the language; the test
// documents the guarantee.
func TestConfig_SourceFor_DoesNotMutateConfig(t *testing.T) {
	t.Parallel()

	c := Config{}
	c.setSource("LogLevel", SourceFile)

	// Snapshot.
	before := c

	// Read.
	_ = c.SourceFor("LogLevel")
	_ = c.SourceFor("OutputFormat")
	_ = c.SourceFor("UnknownField")

	// Compare field-by-field. The sources map is not directly
	// comparable (maps are reference types), so we compare the
	// fields and then the sources via SourceFor.
	assert.Equal(t, before.LogLevel, c.LogLevel)
	assert.Equal(t, before.OutputFormat, c.OutputFormat)
	assert.Equal(t, before.SourcePath, c.SourcePath)
	assert.Equal(t, SourceFile, c.SourceFor("LogLevel"))
}
