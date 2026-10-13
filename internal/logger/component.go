package logger

import "fmt"

// Component identifies the subsystem that emitted a log record.
//
// The enumeration is closed. Adding a component is a contract
// change: it requires an ADR and a corresponding update to
// schemas/log-record.schema.json and
// docs/reference/log-record-format.md. component_test.go asserts
// the Go enum and the schema's enum list are identical.
//
// # Why a closed list
//
// An open component tag would let every package invent its own
// name, and consumers would have to enumerate components by
// observing traffic. A closed list lets a consumer write
// "component in {cli, config, blueprint, template, renderer,
// filesystem, policy}" once and trust it. The list is small
// because it identifies subsystems, not functions: a record about
// a filesystem error is ComponentFilesystem whether the error
// came from a load, a stat, or a walk.
type Component int

const (
	// ComponentInvalid is the zero value. It is not a valid
	// component for a real record; a Record whose Component is
	// ComponentInvalid is a programming error at the call site.
	//
	// Sinks should accept it without panicking (the zero Record
	// uses it), but they are not required to render it
	// meaningfully.
	ComponentInvalid Component = iota

	// ComponentCLI is the command dispatcher, flag parsing,
	// argument validation, and help rendering.
	ComponentCLI

	// ComponentConfig is configuration discovery, loading,
	// validation, precedence resolution, and the resolved
	// configuration's accessors.
	ComponentConfig

	// ComponentBlueprint is blueprint reading, parsing, and
	// validation.
	ComponentBlueprint

	// ComponentTemplate is the template engine's invocation:
	// parsing templates, resolving variables, and producing the
	// rendered output as an in-memory artifact (before it is
	// written to the filesystem).
	ComponentTemplate

	// ComponentRenderer is the process of turning a rendered
	// template artifact into the output bytes that will be
	// written: applying formatting, inserting headers, and
	// ordering.
	ComponentRenderer

	// ComponentFilesystem is every filesystem operation: read,
	// write, walk, stat, mkdir, remove. A record about "could not
	// write file X" is ComponentFilesystem, regardless of which
	// higher-level operation requested the write.
	ComponentFilesystem

	// ComponentPolicy is policy evaluation and findings
	// computation: the code that decides whether a blueprint
	// conforms, what drift exists, and what to report.
	ComponentPolicy
)

// String returns the lowercase tag used in the human and JSON
// renderings.
//
// The values are part of the public contract; they match the
// schema's enum and the documentation's table. Do not change them
// without an ADR.
func (c Component) String() string {
	switch c {
	case ComponentCLI:
		return "cli"
	case ComponentConfig:
		return "config"
	case ComponentBlueprint:
		return "blueprint"
	case ComponentTemplate:
		return "template"
	case ComponentRenderer:
		return "renderer"
	case ComponentFilesystem:
		return "filesystem"
	case ComponentPolicy:
		return "policy"
	case ComponentInvalid:
		return "invalid"
	default:
		return fmt.Sprintf("Component(%d)", int(c))
	}
}

// ParseComponent converts a string tag into a Component.
//
// It is used by tests and by schema validation. The empty string
// returns ComponentInvalid with a nil error, matching the
// convention that an empty value is "unset".
func ParseComponent(s string) (Component, error) {
	switch s {
	case "":
		return ComponentInvalid, nil
	case "cli":
		return ComponentCLI, nil
	case "config":
		return ComponentConfig, nil
	case "blueprint":
		return ComponentBlueprint, nil
	case "template":
		return ComponentTemplate, nil
	case "renderer":
		return ComponentRenderer, nil
	case "filesystem":
		return ComponentFilesystem, nil
	case "policy":
		return ComponentPolicy, nil
	default:
		return ComponentInvalid, fmt.Errorf(
			"logger: unknown component %q", s)
	}
}

// AllComponents returns every canonical component in declaration
// order.
//
// The returned slice is a fresh copy; callers may sort or filter it
// without affecting the package's enum. It is the canonical
// enumeration used by tests, by schema generation, and by any
// future tooling that iterates components.
func AllComponents() []Component {
	return []Component{
		ComponentCLI,
		ComponentConfig,
		ComponentBlueprint,
		ComponentTemplate,
		ComponentRenderer,
		ComponentFilesystem,
		ComponentPolicy,
	}
}
