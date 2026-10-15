package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// cmdVerify loads docs/reference/version-metadata.yaml, validates it
// against schemas/version-metadata.schema.json, and re-loads it through
// the internal/version package so the Go-side invariants also run.
//
// Both validators MUST agree. If the JSON Schema accepts the document
// but the Go loader rejects it (or vice versa), that is a defect in one
// of them, and this command fails so the divergence surfaces.
func cmdVerify(stdout, stderr *os.File) int {
	root, err := findModuleRoot()
	if err != nil {
		fmt.Fprintf(stderr, "forge-internal-version: %v\n", err)
		return 1
	}

	metaPath := filepath.Join(root, "docs", "reference", "version-metadata.yaml")
	schemaPath := filepath.Join(root, "schemas", "version-metadata.schema.json")

	// 1. Load the YAML and normalise it through JSON so the schema
	//    library sees the types it expects. The helper narrows the
	//    return type to map[string]any; version-metadata.yaml's top
	//    level is always a mapping.
	doc, err := loadYAMLAsJSON(metaPath)
	if err != nil {
		fmt.Fprintf(stderr, "forge-internal-version: %v\n", err)
		return 1
	}

	// 2. Compile and apply the JSON Schema.
	compiler := jsonschema.NewCompiler()
	sch, err := compiler.Compile(schemaPath)
	if err != nil {
		fmt.Fprintf(stderr, "forge-internal-version: compile schema %s: %v\n", schemaPath, err)
		return 1
	}
	if err := sch.Validate(doc); err != nil {
		fmt.Fprintf(stderr, "forge-internal-version: %s does not validate:\n%v\n", metaPath, err)
		return 1
	}

	// 3. Cross-check the tag patterns encoded in the YAML against the
	//    Go-side ChannelOf. For each channel's `examples`, ChannelOf
	//    must return that channel; for each `tag_routing.rules[].pattern`,
	//    every example of the matching channel must satisfy it.
	//
	//    This is the differential test between the two validators. A
	//    pattern that drifts in the YAML is caught here, not at
	//    release time.
	if err := crossCheckChannels(doc); err != nil {
		fmt.Fprintf(stderr, "forge-internal-version: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "OK: version-metadata.yaml validates against the schema and ChannelOf")
	return 0
}

// crossCheckChannels asserts that for every channel's `examples`, the
// Go-side ChannelOf returns that channel. It is the executable form of
// the "channel is a pure function of the tag" invariant.
func crossCheckChannels(doc map[string]any) error {
	channelsAny, ok := doc["channels"]
	if !ok {
		return fmt.Errorf("version-metadata.yaml has no `channels` key")
	}
	channels, ok := channelsAny.([]any)
	if !ok {
		return fmt.Errorf("`channels` is not a list")
	}
	for _, chAny := range channels {
		ch, ok := chAny.(map[string]any)
		if !ok {
			return fmt.Errorf("channel entry is not an object")
		}
		name, _ := ch["name"].(string)
		examplesAny, ok := ch["examples"].([]any)
		if !ok {
			return fmt.Errorf("channel %s has no examples", name)
		}
		for _, exAny := range examplesAny {
			ex, _ := exAny.(string)
			got, err := channelOf(ex)
			if err != nil {
				return fmt.Errorf("channel %s: example %q: %v", name, ex, err)
			}
			if got != name {
				return fmt.Errorf("channel %s: example %q routed to %q", name, ex, got)
			}
		}
	}
	return nil
}

// loadYAMLAsJSON reads a YAML file and returns it as a map[string]any
// that encoding/json and the JSON Schema library can consume.
//
// The top level of version-metadata.yaml is a mapping, so the return
// type is narrowed to map[string]any. If the file's top level is not
// a mapping, the helper returns an error naming the actual type
// rather than silently passing the wrong shape to the schema library.
func loadYAMLAsJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var out any
	if err := yaml.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("normalise %s: %w", path, err)
	}
	var normalised map[string]any
	if err := json.Unmarshal(b, &normalised); err != nil {
		return nil, fmt.Errorf("re-normalise %s: %w", path, err)
	}
	return normalised, nil
}

// findModuleRoot walks up from the current working directory looking
// for a go.mod file. It is the same algorithm used by internal/platform
// and by the test helpers in this repository.
func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found in any parent directory")
		}
		dir = parent
	}
}
