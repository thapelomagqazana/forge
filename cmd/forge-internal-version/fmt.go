package main

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// cmdFmt loads version-metadata.yaml, sorts the sortable lists
// (channels by name, prerelease_identifiers by precedence_position,
// decision_table unchanged because order is semantic), and rewrites the
// file in place.
//
// It is idempotent: running it twice produces byte-identical output.
// It uses a temp file plus os.Rename so a crash mid-write cannot
// corrupt the source.
func cmdFmt(stdout, stderr *os.File) int {
	root, err := findModuleRoot()
	if err != nil {
		fmt.Fprintf(stderr, "forge-internal-version: %v\n", err)
		return 1
	}
	path := root + "/docs/reference/version-metadata.yaml"

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "forge-internal-version: read %s: %v\n", path, err)
		return 1
	}

	// Decode into a yaml.Node so we can reorder sequence nodes while
	// preserving the document's structure, comments, and field order.
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		fmt.Fprintf(stderr, "forge-internal-version: parse %s: %v\n", path, err)
		return 1
	}

	// Sort channels by name.
	sortTopLevelSequence(&doc, "channels", func(n *yaml.Node) string {
		return scalarAt(n, "name")
	})

	// Sort prerelease_identifiers by precedence_position ascending.
	sortTopLevelSequenceByInt(&doc, "prerelease_identifiers", func(n *yaml.Node) int {
		return intAt(n, "precedence_position")
	})

	tmp := path + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		fmt.Fprintf(stderr, "forge-internal-version: create %s: %v\n", tmp, err)
		return 1
	}
	enc := yaml.NewEncoder(out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		out.Close()
		os.Remove(tmp)
		fmt.Fprintf(stderr, "forge-internal-version: encode: %v\n", err)
		return 1
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		fmt.Fprintf(stderr, "forge-internal-version: close: %v\n", err)
		return 1
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		fmt.Fprintf(stderr, "forge-internal-version: rename: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "OK: rewrote %s\n", path)
	return 0
}

// sortTopLevelSequence sorts the sequence under the given top-level key
// by the string returned by key. It is a no-op if the key is not
// present or is not a sequence.
func sortTopLevelSequence(doc *yaml.Node, key string, getKey func(*yaml.Node) string) {
	seq := topLevelSequence(doc, key)
	if seq == nil {
		return
	}
	sort.SliceStable(seq.Content, func(i, j int) bool {
		return getKey(seq.Content[i]) < getKey(seq.Content[j])
	})
}

func sortTopLevelSequenceByInt(doc *yaml.Node, key string, getKey func(*yaml.Node) int) {
	seq := topLevelSequence(doc, key)
	if seq == nil {
		return
	}
	sort.SliceStable(seq.Content, func(i, j int) bool {
		return getKey(seq.Content[i]) < getKey(seq.Content[j])
	})
}

func topLevelSequence(doc *yaml.Node, key string) *yaml.Node {
	root := documentRoot(doc)
	if root == nil {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key && root.Content[i+1].Kind == yaml.SequenceNode {
			return root.Content[i+1]
		}
	}
	return nil
}

func documentRoot(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		return n.Content[0]
	}
	if n.Kind == yaml.MappingNode {
		return n
	}
	return nil
}

func scalarAt(n *yaml.Node, key string) string {
	if n.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1].Value
		}
	}
	return ""
}

func intAt(n *yaml.Node, key string) int {
	s := scalarAt(n, key)
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}
