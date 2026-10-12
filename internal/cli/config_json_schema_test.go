package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJSONSchema_MatchesStruct asserts that every JSON tag on the
// configJSON struct appears as a property in schemas/config.schema.json,
// and that every property in the schema corresponds to a struct
// field.
//
// # Why the test exists
//
// The struct is the runtime contract; the schema file is the
// published contract. They must agree. Go cannot derive one from
// the other at compile time, so the test checks the agreement at
// test time. A future addition to the struct without an update to
// the schema (or vice versa) fails the test.
//
// # What the test checks
//
//   - Every field of configJSON has a json tag.
//   - Every json tag appears as a top-level property in the
//     schema's "properties" object.
//   - Every property in the schema's "properties" object has a
//     corresponding field in configJSON.
//
// The test does not check the schema's "required" array; the
// required fields are all fields, and their presence is verified
// by the struct (all fields are always emitted).
func TestJSONSchema_MatchesStruct(t *testing.T) {
	t.Parallel()

	// Collect the struct's JSON tags.
	typ := reflect.TypeOf(configJSON{})
	structTags := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("json")
		require.NotEmpty(t, tag,
			"configJSON field %q has no json tag", field.Name)
		// Strip ",omitempty" and other options if present.
		name := tag
		if idx := strings.Index(tag, ","); idx >= 0 {
			name = tag[:idx]
		}
		structTags = append(structTags, name)
	}
	sort.Strings(structTags)

	// Load the schema.
	schemaPath := filepath.Join("..", "..", "schemas", "config.schema.json")
	// Actually, the test runs from internal/cli; the schema is at
	// ../../schemas/config.schema.json from there. See the
	// package-dir helper if the repo has one.
	data, err := os.ReadFile(schemaPath)
	require.NoError(t, err,
		"cannot read schema file; check the path: %s", schemaPath)

	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(data, &schema),
		"schema file is not valid JSON")

	// Collect the schema's properties.
	schemaKeys := make([]string, 0, len(schema.Properties))
	for k := range schema.Properties {
		schemaKeys = append(schemaKeys, k)
	}
	sort.Strings(schemaKeys)

	assert.Equal(t, structTags, schemaKeys,
		"the struct's JSON tags and the schema's properties must match")
}
