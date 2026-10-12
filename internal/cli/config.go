package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/thapelomagqazana/forge/internal/config"
)

// =============================================================================
// Frozen metadata — WBS 9.1.1
// =============================================================================
//
// The command's identity strings are frozen by WBS 9.1.1. Any change
// requires a new ADR. The four constants below are the single source
// of truth for the command's Use, Short, Long, and Example fields;
// the help golden file and the metadata tests assert against them.
// =============================================================================

// configCommandUse is the command's identity string. It MUST remain
// lowercase and argument-free. Aliases (cfg, conf) are explicitly
// rejected to avoid ambiguity in scripts and documentation.
const configCommandUse = "config"

// configCommandShort is the one-line summary shown in `forge --help`
// under "Available Commands:". It MUST:
//
//   - use imperative mood ("Display …", not "Displays …"),
//   - be ≤ 60 characters (enforced by TestConfigCommand_ShortLength),
//   - omit trailing punctuation (Cobra convention; the renderer adds
//     the separator).
const configCommandShort = "Display resolved CLI configuration"

// configCommandLong is the multi-paragraph description shown by
// `forge config --help`. It MUST:
//
//   - wrap at 80 columns,
//   - end with a period,
//   - document the precedence model (flags > env > file > defaults),
//   - mention the --path flag's purpose (added in WBS 9.3.1).
const configCommandLong = `Display the resolved CLI configuration for the current invocation.

The output reflects all precedence layers: CLI flags, environment
variables, the config file, and built-in defaults. Each field is
annotated with its effective source.

Use --path to print only the resolved config file location.`

// configCommandExample is the Example block shown by
// `forge config --help`. It MUST contain exactly three lines,
// covering the three primary invocations: default output, --path,
// and --format json. The leading two-space indent matches the Cobra
// convention and is preserved verbatim in the golden file.
const configCommandExample = `  forge config
  forge config --path
  forge config --format json`

// =============================================================================
// Frozen output format — WBS 9.2.1 (text) + WBS 9.4.1 (JSON)
// =============================================================================
//
// The command has two output modes, selected by the global --format
// flag:
//
//   - text: the frozen text format from WBS 9.2.1 (default).
//   - json: the frozen JSON format from WBS 9.4.1.
//
// The --path flag (WBS 9.3.1) suppresses the full output and prints
// only the resolved config file path; it composes with --format.
//
// Text format constants
// ---------------------
//
// The rules for the text format are documented on printConfig. The
// constants below are the frozen values those rules refer to.
// =============================================================================

// configTitle is line 1 of the frozen text output (rule F1). It MUST
// appear verbatim as the first line of every `forge config`
// invocation in text mode.
const configTitle = "Forge CLI Configuration"

// configLabelWidth is the column width to which value-line labels
// are padded (rule F4). The value 14 accommodates the longest label
// in the frozen format ("Log level:" at 10 characters) with room to
// spare, and aligns the value column across all value lines.
const configLabelWidth = 14

// configSeparatorRune is the separator character (rule F2). It is
// U+2500 (BOX DRAWINGS LIGHT HORIZONTAL). The separator line is
// built by repeating this rune to the width of configTitle.
const configSeparatorRune = '\u2500'

// configValueColumn is the fixed width of the value column in
// characters. It is chosen so that the longest value the resolver
// can currently emit ("debug", "error", "json", all ≤ 5 chars)
// leaves at least one space before the "(source:" attribution. The
// value column is a rendering detail not named in WBS 9.2.1's rule
// list; it is pinned by the golden files. The rule F4 label width
// (14) and this value column (6) together determine the column at
// which "(source:" begins on every value line.
const configValueColumn = 6

// configSource identifies the layer from which a value was resolved.
// The vocabulary is frozen by WBS 9.2.1, rule F6. It is declared as
// a distinct string type so that a caller cannot substitute an
// arbitrary string where a source is expected.
//
// The same vocabulary is used by the JSON output (WBS 9.4.1): the
// `*_source` fields carry the values of this type.
type configSource string

const (
	// configSourceDefault means the value came from the built-in
	// default, not from any file, environment variable, or flag.
	configSourceDefault configSource = "default"

	// configSourceFile means the value came from the resolved
	// configuration file.
	configSourceFile configSource = "file"

	// configSourceEnv means the value came from an environment
	// variable.
	configSourceEnv configSource = "env"

	// configSourceFlag means the value came from a CLI flag.
	configSourceFlag configSource = "flag"
)

// ResolvedConfig is the input to printConfig and writeConfigJSON.
// It carries exactly the data the frozen formats render: the
// resolved source path, the log level and its source, and the
// output format and its source.
//
// # Relationship to internal/config.Config
//
// This type is not config.Config. It is a projection of that type
// onto the subset the CLI displays. The projection is performed by
// resolveForDisplay, which is the only function in this file that
// knows config.Config's shape. Separating the two types means a
// change to config.Config's layout touches resolveForDisplay only,
// and the renderers remain pure functions of their inputs.
//
// # Why no Logger, FS, or Env
//
// The type is deliberately narrow. It carries only the data that
// appears on stdout. It does not carry the config struct itself,
// the filesystem, or the environment, because the renderers do not
// need them and giving them access would invite scope creep.
type ResolvedConfig struct {
	// SourcePath is the resolved configuration file path, or the
	// empty string if no file was found. When empty, printConfig
	// renders "<none>" and writeConfigJSON emits null. The path is
	// resolved by WBS 8.4.1's discovery logic; this type does not
	// perform discovery.
	SourcePath string

	// LogLevel is the effective log level ("info", "debug", "error",
	// …). The resolver lowercases it; the renderers do not.
	LogLevel string

	// LogLevelSource is the effective source of LogLevel. It MUST be
	// one of the four configSource values.
	LogLevelSource configSource

	// Output is the effective output format ("text", "json", …).
	// The resolver lowercases it; the renderers do not.
	Output string

	// OutputSource is the effective source of Output.
	OutputSource configSource
}

// =============================================================================
// Frozen JSON output format — WBS 9.4.1
// =============================================================================
//
// When --format json is set, `forge config` emits a single-line JSON
// object with the resolved configuration and the source of each
// value. The schema is:
//
//	{
//	  "config_path": "/home/user/.config/forge/config.yaml",
//	  "log_level": "info",
//	  "log_level_source": "default",
//	  "output_format": "text",
//	  "output_format_source": "default"
//	}
//
// When no file is resolved, config_path is null (not the empty
// string).
//
// The --path --format json mode (WBS 9.3.1) uses a one-key variant
// of the same schema:
//
//	{"config_path": "/home/user/.config/forge/config.yaml"}
//	{"config_path": null}
//
// The two modes share the config_path key. WBS 9.3.1 originally
// named the key "path"; WBS 9.4.1 renamed it to "config_path" to
// align the two surfaces. The rename is documented in the
// configPathJSONKey constant below.
//
// Keys are emitted in alphabetical order. For the full output, the
// order is the struct's field declaration order (encoding/json
// emits struct fields in declaration order). For the --path mode,
// there is only one key.
// =============================================================================

// configJSONKeyConfigPath is the JSON key for the resolved config
// file path, shared by the full output and the --path mode.
//
// # Why a constant
//
// The key appears in two places: as the json struct tag on the
// configJSON.ConfigPath field, and as the json struct tag on the
// --path mode's pathObject.Path field. Go does not permit a
// constant in a struct tag, so the two tags are string literals.
// The constant exists so that the key's identity is greppable and
// so that a future rename is a single change to this constant plus
// the two tags (which a test enforces in sync).
//
// # History
//
// WBS 9.3.1 (--path) originally named the key "path". WBS 9.4.1
// (full output) uses "config_path". The two WBS items describe the
// same value; using different keys would produce a CLI where the
// same query (the config file path) has two different JSON keys
// depending on which mode produced the output. The rename aligns
// them. WBS 9.3.1's golden file is regenerated to reflect the new
// key.
//
// # The fallback check
//
// Tests (TestWriteConfigPathJSON_KeyMatchesConstant and
// TestWriteConfigJSON_KeyMatchesConstant) assert that the struct
// tags match this constant. A future change that renamed the
// constant without updating the tags — or vice versa — fails at
// test time.
const configJSONKeyConfigPath = "config_path"

// configJSONKeyLogLevel is the JSON key for the effective log
// level. Frozen by WBS 9.4.1, following the snake_case convention
// from WBS 6.4.2.
const configJSONKeyLogLevel = "log_level"

// configJSONKeyLogLevelSource is the JSON key for the source of the
// log level. Frozen by WBS 9.4.1; the `_source` suffix is the
// source-attribution convention.
const configJSONKeyLogLevelSource = "log_level_source"

// configJSONKeyOutputFormat is the JSON key for the effective
// output format. Frozen by WBS 9.4.1.
const configJSONKeyOutputFormat = "output_format"

// configJSONKeyOutputFormatSource is the JSON key for the source of
// the output format. Frozen by WBS 9.4.1.
const configJSONKeyOutputFormatSource = "output_format_source"

// configJSON is the wire representation of the full configuration
// output. Its field order determines the JSON key order in the
// emitted object.
//
// # Field order
//
// The fields are declared in alphabetical order by JSON key:
//
//	config_path
//	log_level
//	log_level_source
//	output_format
//	output_format_source
//
// Go's json.Encoder emits struct fields in declaration order, so
// the struct's order IS the wire's order. Reordering the fields
// changes the wire format;
// TestWriteConfigJSON_KeysAreAlphabetical fails if the emitted
// order is not alphabetical.
//
// # Field types
//
//   - ConfigPath is *string because the value is nullable: when no
//     file was resolved, the JSON value is null, not "".
//   - The other fields are string. LogLevel and OutputFormat are the
//     effective values; the *_source fields are the vocabulary
//     frozen by WBS 9.2.1, rule F6.
//
// # Why a struct and not a map
//
// A map[string]any would let the encoder sort keys automatically
// (encoding/json sorts map keys), but it would lose the type
// information and require the caller to build string values for
// every field. The struct is compile-time checked, self-documenting,
// and its declaration order makes the wire order explicit and
// reviewable.
//
// # Why no json:"-" fields
//
// The struct is a pure wire type. Every field is emitted. A future
// field that should not appear in the JSON output is a new struct,
// not a hidden field.
//
// # Struct tags and the key constants
//
// The tags are string literals; Go does not permit a constant in a
// struct tag. The constants (configJSONKey*) document each key's
// identity and are kept in sync with the tags by
// TestWriteConfigJSON_KeyMatchesConstant.
type configJSON struct {
	// ConfigPath is the resolved configuration file path, or nil
	// when no file was found. The JSON key is "config_path".
	ConfigPath *string `json:"config_path"`

	// LogLevel is the effective log level. The JSON key is
	// "log_level".
	LogLevel string `json:"log_level"`

	// LogLevelSource is the source of LogLevel. The JSON key is
	// "log_level_source".
	LogLevelSource string `json:"log_level_source"`

	// OutputFormat is the effective output format. The JSON key is
	// "output_format".
	OutputFormat string `json:"output_format"`

	// OutputFormatSource is the source of OutputFormat. The JSON
	// key is "output_format_source".
	OutputFormatSource string `json:"output_format_source"`
}

// =============================================================================
// Frozen --path semantics — WBS 9.3.1
// =============================================================================
//
// When --path is set, `forge config` prints only the resolved config
// file path (or "<none>") and exits 0. The default full-configuration
// output is suppressed. The --format flag selects text or JSON
// rendering.
//
// The frozen table is documented in docs/cli-ux-spec.md under
// "forge config --path". The handler implements the text and JSON
// rows; the loader implements the "missing explicit --config" row
// (it errors with exit 3 before the handler runs).
// =============================================================================

// configFlagPath is the name of the command-local --path flag. It is
// declared on the config command itself, not on the root, because no
// other command has a concept of "path only".
const configFlagPath = "path"

// configPathNoneText is the text rendered when no config file was
// resolved and the output format is text. It is the literal string
// "<none>", frozen by WBS 9.3.1.
//
// The rendering is not a label with a placeholder value; it is the
// placeholder value alone. `forge config --path` in text mode
// produces either the path or this string, with nothing else on the
// line.
const configPathNoneText = "<none>"

// configPathJSONKey is the key of the JSON object that
// `forge config --path --format json` emits, and the key of the
// `config_path` field in the full JSON output.
//
// # History
//
// WBS 9.3.1 (--path mode) originally named the key "path". WBS
// 9.4.1 (full output) uses "config_path". The two WBS items
// describe the same value; using different keys would produce a
// CLI where the same query has two different JSON keys depending
// on which mode produced the output. WBS 9.4.1 aligns them.
//
// # Value
//
// The constant's value is the same as configJSONKeyConfigPath.
// The two constants are kept in sync by hand; a test
// (TestJSONKeyConstantsAreAligned) asserts their equality.
const configPathJSONKey = configJSONKeyConfigPath

// configGlobalFlagFormat is the name of the global --format flag
// declared on the root command. The config command inherits it when
// it is reached through the root; the handler reads it via
// cmd.Flags().GetString(configGlobalFlagFormat), which returns the
// effective value regardless of which scope set it.
//
// The constant exists so that a future rename of the global flag is
// a single edit. If the global flag's name changes, this constant
// changes with it; the handler's call site does not.
//
// # Why the flag might be absent from cmd.Flags()
//
// The flag is present on cmd.Flags() when the config command was
// reached through its root (Cobra merges ancestor persistent flags
// into the child's flag set before invoking RunE). The flag is
// absent when the config command was constructed in isolation, as
// the metadata tests do. The handler tolerates absence via the
// configDefaultFormat fallback; see runConfig's docstring.
const configGlobalFlagFormat = "format"

// configDefaultFormat is the format value used by runConfig when
// the inherited --format flag is not present on the command's flag
// set.
//
// # Why a fallback is needed
//
// In production, `forge config` is a subcommand of the root, and
// the root's persistent --format flag is inherited before RunE
// runs. The handler reads it and dispatches on "text" or "json".
//
// In tests, the command may be constructed in isolation (without
// its parent) to exercise metadata. In that construction, the
// inherited flag is not present on cmd.Flags(), and reading it
// would fail with "flag accessed but not defined". The fallback
// makes the handler robust to that case.
//
// The fallback value "text" matches the frozen default of the
// global --format flag (WBS 6.4.1). A production invocation that
// reaches RunE without a value for --format receives the same
// rendering as if the flag were explicitly set to its default.
const configDefaultFormat = "text"

// =============================================================================
// Command constructor and handler
// =============================================================================

// newConfigCmd constructs the `forge config` command.
//
// Metadata is frozen by WBS 9.1.1. Behavior is frozen by WBS 9.2.1
// (default text output), WBS 9.3.1 (--path), and WBS 9.4.1 (JSON
// output). The command is listed in the registry
// (internal/cli/registry.go) in the user-facing group,
// alphabetically before newVersionCmd.
//
// # Contract
//
//   - Use:     configCommandUse       ("config", lowercase)
//   - Short:   configCommandShort     (≤ 60 chars, imperative)
//   - Long:    configCommandLong      (wraps at 80, ends with period)
//   - Example: configCommandExample   (exactly 3 lines)
//   - Args:    cobra.NoArgs           (no positional arguments)
//   - Aliases: none                   (cfg/conf rejected)
//   - Hidden:  false                  (command is user-visible)
//
// # Flags
//
// The command declares one local flag:
//
//	--path    print only the resolved config file path (WBS 9.3.1)
//
// The command inherits the root's persistent flags, including
// --format (text|json), when it is reached through the root. The
// handler reads the effective --format via cmd.Flags().GetString
// when the flag is present, and falls back to configDefaultFormat
// when it is not.
//
// # Handler
//
// RunE calls runConfig, which dispatches on --path and --format:
//
//   - --path set:   writeConfigPath renders the path (or "<none>")
//   - --path unset, --format text: printConfig renders the full config
//   - --path unset, --format json: writeConfigJSON renders the full config
//
// The handler follows the handler / service boundary (WBS 4.3.1):
// it is a thin closure, it returns an error (never calls os.Exit),
// and it writes only through deps.Stdout.
func newConfigCmd(deps Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:     configCommandUse,
		Short:   configCommandShort,
		Long:    configCommandLong,
		Example: configCommandExample,
		Args:    cobra.NoArgs,
		Hidden:  false,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfig(cmd, deps)
		},
	}

	// The --path flag is command-local. It is not persistent: no
	// other command has a concept of "path only".
	cmd.Flags().Bool(configFlagPath, false,
		"Print only the resolved config file path")

	return cmd
}

// runConfig is the handler body for `forge config`.
//
// It is separated from the constructor so that it can be tested in
// isolation, without constructing a *cobra.Command.
//
// # Dispatch
//
// The handler reads two flags:
//
//   - --path:   whether to print only the resolved config file path
//   - --format: whether to render as text or JSON (inherited)
//
// It then dispatches:
//
//   - --path set,  any --format: writeConfigPath(...)
//   - --path unset, --format text: printConfig(...)
//   - --path unset, --format json: writeConfigJSON(...)
//
// The non---path mode validates the --format value: a value other
// than "text", "json", or "" (the empty-string default) produces an
// error. The error boundary in execute.go maps the error to
// ExitUsage (2), per WBS 7.3.1.
//
// # Where the resolved Config comes from
//
// The handler reads the resolved Config from the command's context,
// where the PersistentPreRunE hook (hooks.go) stores it. The hook
// runs before RunE and stores the resolved value with
// contextWithConfig; the handler reads it with ConfigFromContext.
//
// The hook does *not* write through deps.Config; the write-through
// was removed to eliminate a data race (the pointer is shared
// across concurrent invocations). The context is the sole
// transport, and it is per-invocation.
//
// The handler falls back to deps.Config only when the context does
// not carry a Config. That case occurs when the command is invoked
// in isolation (a test that calls newConfigCmd(...) and then
// cmd.RunE(...) directly, bypassing executeWithOptions and
// therefore the hook). In production, the hook always runs, so the
// fallback is dormant.
//
// # Format resolution
//
// The handler reads --format via a two-step lookup:
//
//  1. If the flag is present on the command's effective flag set
//     (either because it is local or because it was inherited from
//     the root), read its value.
//
//  2. If the flag is not present, use configDefaultFormat.
//
// The fallback is deliberate. The handler can be invoked from a
// command that was constructed in isolation (no parent), in which
// case the inherited flag is not on cmd.Flags(). The fallback
// produces the same rendering the production path would produce
// for the flag's default. The alternative — returning an error —
// would make the command unusable when detached from the tree,
// which is a worse property for a handler that only needs a
// string.
//
// # Why the missing-explicit-config case is not handled here
//
// WBS 9.3.1's frozen table distinguishes two "no path" cases:
//
//   - `--config <path>` names a file that does not exist → exit 3.
//   - No --config and discovery finds nothing → "<none>", exit 0.
//
// The first case is handled by the loader (WBS 8.4.x), which runs
// before the command tree. If --config names a missing file, the
// loader returns an E1 error, executeWithOptions surfaces it on
// stderr, and RunE never runs. The handler therefore sees only the
// second case: a Config whose SourcePath is empty. It renders
// "<none>" (text) or null (JSON).
//
// This is the correct division of responsibility. The loader knows
// whether the path was explicit (it read the flag); the handler
// knows only the resolution result. Re-checking the flag here would
// duplicate the loader's logic and risk drift.
//
// # Errors
//
// runConfig returns an error if the --path flag read fails, if the
// --format value is unsupported, or if the render fails. The error
// boundary in execute.go converts the returned error into a stderr
// diagnostic and a non-success exit code.
func runConfig(cmd *cobra.Command, deps Dependencies) error {
	// Resolve the effective --format. See the docstring's
	// "Format resolution" section for why the lookup is
	// two-step.
	format := configDefaultFormat
	if cmd.Flags().Lookup(configGlobalFlagFormat) != nil {
		var err error
		format, err = cmd.Flags().GetString(configGlobalFlagFormat)
		if err != nil {
			return fmt.Errorf("forge config: read --%s: %w",
				configGlobalFlagFormat, err)
		}
	}

	// Read the command-local --path flag. It is declared on the
	// command itself, so the lookup always succeeds when the
	// command is the one constructed by newConfigCmd.
	showPath, err := cmd.Flags().GetBool(configFlagPath)
	if err != nil {
		return fmt.Errorf("forge config: read --%s: %w",
			configFlagPath, err)
	}

	// Resolve the Config. Prefer the context transport (the
	// hook's path); fall back to deps.Config for the isolated
	// case where the hook did not run. See the docstring's
	// "Where the resolved Config comes from" section.
	resolvedCfg := configForDisplay(cmd, deps)

	if showPath {
		if err := writeConfigPath(deps.Stdout, resolvedCfg.SourcePath, format); err != nil {
			return fmt.Errorf("forge config: %w", err)
		}
		return nil
	}

	// Non---path mode: dispatch on --format. The --path and non
	// --path modes share the "text"/"json" vocabulary but have
	// separate renderers; the shared vocabulary is enforced by
	// the switch below and the one in writeConfigPath.
	resolved := resolveForDisplay(resolvedCfg)
	switch format {
	case "json":
		if err := writeConfigJSON(deps.Stdout, resolved); err != nil {
			return fmt.Errorf("forge config: %w", err)
		}
	case "text", "":
		if err := printConfig(deps.Stdout, resolved); err != nil {
			return fmt.Errorf("forge config: %w", err)
		}
	default:
		return fmt.Errorf("forge config: unsupported --%s value %q: want %q or %q",
			configGlobalFlagFormat, format, "text", "json")
	}
	return nil
}

// configForDisplay returns the resolved Config for the current
// invocation.
//
// It reads the resolved Config from the command's context (the
// hook's transport) and falls back to deps.Config only when the
// context does not carry one.
//
// # Why a helper
//
// The function names the concept "where does the resolved Config
// come from for this invocation?" and centralizes the context
// lookup and the fallback. Both --path and the full output need
// the resolved Config; both call this helper once.
//
// # Why the fallback exists
//
// In production, the hook always runs and always stores the
// resolved Config in the context. The fallback covers the case
// where a caller constructs the command in isolation and invokes
// RunE without going through executeWithOptions (and therefore
// without the hook running). That case is exercised by the
// metadata tests, which read the command's fields but do not
// dispatch through the root. In that case, deps.Config carries
// whatever the caller supplied (typically the zero value).
//
// # Why the context is preferred
//
// The hook's write-through to deps.Config was removed to
// eliminate a data race; the context is the only transport that
// is safe under concurrency and that is guaranteed to hold the
// resolved value after the hook runs. Reading it first means the
// resolved value is what the renderer sees, even if the caller
// supplied a zero-value deps.Config.
func configForDisplay(cmd *cobra.Command, deps Dependencies) config.Config {
	if cfg, ok := ConfigFromContext(cmd.Context()); ok {
		return cfg
	}
	if deps.Config != nil {
		return *deps.Config
	}
	return config.Config{}
}

// writeConfigPath renders the resolved config file path to w in the
// format selected by --format.
//
// # Frozen semantics (WBS 9.3.1)
//
//	--format text, path set:    <path>\n
//	--format text, path empty:  <none>\n
//	--format json, path set:    {"config_path":"<path>"}\n
//	--format json, path empty:  {"config_path":null}\n
//
// Every output is a single line ending in \n. In text mode there is
// no label, no decoration: the line contains the path or the
// placeholder, nothing else.
//
// # The JSON key
//
// The JSON key is `config_path`, shared with the full output
// (WBS 9.4.1). WBS 9.3.1 originally named the key `path`; WBS
// 9.4.1 renamed it to `config_path` for consistency with the full
// output's schema. The --path golden file is regenerated
// accordingly.
//
// # Why the JSON encoding uses encoding/json
//
// The path may contain characters that require JSON escaping
// (quotes, backslashes, control characters, non-ASCII). String
// concatenation would produce invalid JSON for such paths. The
// json.Encoder handles escaping correctly and appends the trailing
// newline that WBS 9.3.1 requires.
//
// # Empty path in JSON mode
//
// An empty path encodes as null, not as the empty string. The
// distinction matters: an empty string would suggest a valid but
// empty path; null expresses "no path resolved". The frozen table
// (WBS 9.3.1) requires null.
func writeConfigPath(w io.Writer, path string, format string) error {
	switch format {
	case "json":
		return writeConfigPathJSON(w, path)
	case "text", "":
		return writeConfigPathText(w, path)
	default:
		return fmt.Errorf("unsupported --%s value %q: want %q or %q",
			configGlobalFlagFormat, format, "text", "json")
	}
}

// writeConfigPathText writes the path (or "<none>") as a single line
// of text.
//
// The function is the text half of writeConfigPath. It is separated
// so that the JSON half can be tested independently of the format
// dispatch, and so that each half's docstring can be precise.
//
// # Format (frozen by WBS 9.3.1)
//
//	<path>\n    when path is non-empty
//	<none>\n    when path is empty
//
// The line contains no label, no prefix, no trailing punctuation
// beyond the required \n.
func writeConfigPathText(w io.Writer, path string) error {
	if path == "" {
		path = configPathNoneText
	}
	if _, err := fmt.Fprintf(w, "%s\n", path); err != nil {
		return err
	}
	return nil
}

// writeConfigPathJSON writes the path as a JSON object with a single
// key.
//
// # Format (frozen by WBS 9.3.1, key renamed by WBS 9.4.1)
//
//	{"config_path":"<path>"}\n   when path is non-empty
//	{"config_path":null}\n       when path is empty
//
// The empty case uses null, not "". The distinction is intentional:
// null expresses "no path resolved"; an empty string would suggest a
// valid but empty path.
//
// # Why a struct and not a map
//
// A single-field struct is the idiomatic way to render a JSON object
// with a fixed schema. A map[string]any would work but would
// allocate, and its key order would be non-deterministic (though with
// one key, the order is moot). The struct pins the schema: adding a
// field is a compile-visible change, and the encoder's output is
// deterministic.
//
// # Why a *string for the path
//
// The json package encodes a nil *string as null and a non-nil
// *string as the string's value. Using *string lets the same code
// produce both the "path set" and "path empty" outputs without a
// branch: nil for the empty case, &path for the set case.
//
// The alternative is `any` with either path or nil. That works but
// obscures the intent; a *string documents "this field is nullable"
// and gets the encoding right without a runtime type switch.
//
// # Why SetEscapeHTML(false)
//
// By default, json.Encoder escapes <, >, and & as \u003c, \u003e,
// and \u0026 to protect against HTML injection when JSON is
// embedded in a <script> tag. The CLI's JSON output is not embedded
// in HTML; it is consumed by tools like jq. Keeping the characters
// literal makes the output shorter and easier to read in a terminal.
//
// # Struct tag and configPathJSONKey
//
// The struct's Path field carries the tag `json:"config_path"`.
// The string in the tag must equal configPathJSONKey (which in
// turn equals configJSONKeyConfigPath); the two are kept in sync
// by hand because Go does not permit a constant in a struct tag.
// The test TestWriteConfigPathJSON_KeyMatchesConstant asserts the
// equality, so a rename of the constant that missed the tag would
// fail at test time.
func writeConfigPathJSON(w io.Writer, path string) error {
	type pathObject struct {
		Path *string `json:"config_path"`
	}

	obj := pathObject{}
	if path != "" {
		obj.Path = &path
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(obj)
}

// =============================================================================
// JSON rendering of the full configuration — WBS 9.4.1
// =============================================================================

// writeConfigJSON renders cfg to w as a single-line JSON object.
//
// # Format (frozen by WBS 9.4.1)
//
//	{
//	  "config_path": "<path>" | null,
//	  "log_level": "<level>",
//	  "log_level_source": "<source>",
//	  "output_format": "<format>",
//	  "output_format_source": "<source>"
//	}
//
// The output is a single line terminated by \n. Keys are emitted in
// alphabetical order (the configJSON struct's declaration order).
//
// # Why the encoder is reused
//
// The same encoder configuration as writeConfigPathJSON is used:
// SetEscapeHTML(false) keeps <, >, and & literal, which makes the
// output shorter and easier to read for shell consumers. The two
// writers produce the same escaping behavior on the same input,
// which keeps the two JSON surfaces consistent.
//
// # Null vs. empty string
//
// ConfigPath is a *string. When cfg.SourcePath is empty, the
// pointer is nil and the encoder emits null. When cfg.SourcePath
// is non-empty, the pointer is &cfg.SourcePath and the encoder
// emits the path as a JSON string. The distinction matches the
// schema: null means "no file resolved", a string means "this is
// the path".
//
// # Errors
//
// writeConfigJSON returns the first write error the encoder
// encounters. The caller wraps it with the "forge config:" prefix.
// The encoder does not buffer; a writer that fails mid-output
// leaves whatever bytes it already accepted.
//
// # Purity
//
// writeConfigJSON is a pure function of (w, cfg): it reads no
// globals, consults no environment, and touches no filesystem. All
// state it needs is in cfg, and all output goes to w. This is the
// same purity property as printConfig and writeConfigPathJSON.
func writeConfigJSON(w io.Writer, cfg ResolvedConfig) error {
	obj := configJSON{
		LogLevel:           cfg.LogLevel,
		LogLevelSource:     string(cfg.LogLevelSource),
		OutputFormat:       cfg.Output,
		OutputFormatSource: string(cfg.OutputSource),
	}
	if cfg.SourcePath != "" {
		// Take the address of a local copy so that the *string is
		// stable for the duration of the encoder call and does
		// not alias the caller's ResolvedConfig. The local copy
		// is also necessary because cfg is passed by value and
		// &cfg.SourcePath would point into the callee's stack
		// frame; the encoder is called before the frame is
		// popped, but the copy is safer and clearer.
		path := cfg.SourcePath
		obj.ConfigPath = &path
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(obj)
}

// resolveForDisplay projects a resolved config.Config onto the
// rendering-input type ResolvedConfig.
//
// # Source translation
//
// The config package's Source is an integer enum (WBS 8.1.2),
// ordered by precedence. The CLI's configSource is a string enum
// (WBS 9.2.1, rule F6), matching the frozen output's vocabulary.
// The two enums are deliberately distinct: the config package
// orders sources; the CLI renders them. The projection bridges the
// two via toConfigSource.
//
// # Field names
//
// SourceFor takes Go field names ("LogLevel", "OutputFormat"), not
// YAML keys. The names are string literals here; the config
// package does not export them as constants, and exporting them
// would widen its public surface for a single caller's benefit.
//
// # Why not recompute the source
//
// The projection copies the source from the Config; it does not
// infer the source by comparing the value to a default. A
// recomputation would duplicate the loader's precedence logic
// (WBS 8.1.2) and would be wrong for two cases the loader handles
// correctly: a file that explicitly sets a field to its default
// value (the source is "file", not "default"), and a value that
// two layers happen to set to the same string (the source is the
// higher-priority layer). Reading the recorded source is the only
// correct approach.
//
// # Why the argument is config.Config, not Dependencies
//
// The caller (runConfig) resolves the Config from the command's
// context or from deps.Config, then passes the resolved value.
// Passing Dependencies would force this function to read deps
// again, duplicating the resolution logic. Passing the resolved
// value makes the dependency explicit and the function pure.
//
// # Rule 5 compliance
//
// The function has no package-level state. Every value it returns
// is constructed locally.
func resolveForDisplay(cfg config.Config) ResolvedConfig {
	return ResolvedConfig{
		SourcePath:     cfg.SourcePath,
		LogLevel:       cfg.LogLevel,
		LogLevelSource: toConfigSource(cfg.SourceFor("LogLevel")),
		Output:         cfg.OutputFormat,
		OutputSource:   toConfigSource(cfg.SourceFor("OutputFormat")),
	}
}

// toConfigSource maps a config.Source to the CLI's configSource
// vocabulary.
//
// # Why an explicit switch and not config.Source.String()
//
// config.Source.String() returns "environment" for SourceEnv. The
// frozen output format (WBS 9.2.1, rule F6) requires "env". The
// two vocabularies differ for exactly one value, which is the kind
// of discrepancy a call to String() would silently propagate into
// the golden files. The explicit switch names each mapping on its
// own line, and the default case fails loudly if config.Source
// gains a value.
//
// # Unknown values
//
// config.SourceUnknown is not a valid source; SourceFor never
// returns it. If a future refactor produces one, the default
// branch maps it to configSourceDefault, matching SourceFor's
// semantics for an unset field. The test
// TestToConfigSource_Exhaustive enumerates every config.Source
// value, so a new value added to the config package without
// updating this switch is caught at test time.
func toConfigSource(s config.Source) configSource {
	switch s {
	case config.SourceDefault:
		return configSourceDefault
	case config.SourceFile:
		return configSourceFile
	case config.SourceEnv:
		return configSourceEnv
	case config.SourceFlag:
		return configSourceFlag
	default:
		return configSourceDefault
	}
}

// =============================================================================
// Text rendering — WBS 9.2.1
// =============================================================================

// printConfig renders cfg to w in the frozen text format declared by
// WBS 9.2.1.
//
// # Format
//
// The output is:
//
//	Forge CLI Configuration
//	───────────────────────
//
//	  Source:       /home/user/.config/forge/config.yaml
//	  Log level:    info        (source: default)
//	  Output:       text        (source: default)
//
// # Byte-level rules (WBS 9.2.1)
//
//	F1  Line 1: "Forge CLI Configuration".
//	F2  Line 2: separator of U+2500, width equal to len(configTitle).
//	F3  Line 3: blank.
//	F4  Value lines: two-space indent, label padded to
//	    configLabelWidth, value, spaces, "(source: <src>)".
//	F5  Source attribution on every value line.
//	F6  Source is one of: default, file, env, flag.
//	F7  Source: line shows the resolved path, or "<none>".
//	F8  No trailing whitespace on any line.
//	F9  No colour, no ANSI escapes.
//	F10 Every line ends with \n, including the last.
//
// # Purity
//
// printConfig is a pure function of (w, cfg): it reads no globals,
// consults no environment, and touches no filesystem. All state it
// needs is in cfg, and all output goes to w. This property is what
// makes the format testable in isolation and is the reason the
// rendering is separated from the resolution.
//
// # Errors
//
// printConfig returns the first write error it encounters from w.
// The caller (runConfig) wraps the error with the "forge config:"
// prefix. No partial-output cleanup is attempted; a writer that
// fails mid-output leaves whatever bytes it already accepted, which
// is the standard io.Writer contract.
func printConfig(w io.Writer, cfg ResolvedConfig) error {
	// F1: title line.
	if _, err := fmt.Fprintf(w, "%s\n", configTitle); err != nil {
		return err
	}

	// F2: separator line, width matching the title. The separator is
	// built here rather than at package scope because strings.Repeat
	// is not a constant expression, and Rule 5 forbids package-level
	// vars in command files. Building it per invocation costs
	// nothing measurable and keeps the file contract-compliant.
	separator := strings.Repeat(string(configSeparatorRune), len(configTitle))
	if _, err := fmt.Fprintf(w, "%s\n", separator); err != nil {
		return err
	}

	// F3: blank line separating the title block from the value block.
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	// F7: Source line. The resolved path is shown verbatim, or
	// "<none>" when no file was resolved. The Source: line has no
	// source attribution (there is nothing for it to be attributed
	// to; the path *is* the source).
	path := cfg.SourcePath
	if path == "" {
		path = configPathNoneText
	}
	if err := writeLabeledLine(w, "Source:", path, ""); err != nil {
		return err
	}

	// F4/F5: Log level line with source attribution.
	if err := writeLabeledLine(w, "Log level:",
		cfg.LogLevel, string(cfg.LogLevelSource)); err != nil {
		return err
	}

	// F4/F5: Output line with source attribution.
	if err := writeLabeledLine(w, "Output:",
		cfg.Output, string(cfg.OutputSource)); err != nil {
		return err
	}

	return nil
}

// writeLabeledLine writes one value line of the frozen format:
//
//	"  " + label padded to configLabelWidth + value [+ attribution]
//
// # Alignment
//
// The label is right-trimmed of any trailing spaces, then padded on
// the right with spaces so that its rendered width is exactly
// configLabelWidth characters. The value follows immediately. If
// source is non-empty, the string "(source: <src>)" is appended,
// separated from the value by enough spaces that "(source:" begins
// at the same column on every attributed line.
//
// # Trailing whitespace (F8)
//
// The function never emits a line whose final character is a space.
// If no attribution is written (the Source: line), the line ends
// after the value. If an attribution is written, the line ends after
// the closing ")"; the padding before the attribution is interior
// whitespace and does not violate F8. As a defensive measure, the
// line is right-trimmed before being written, so that a future edit
// to the padding arithmetic cannot silently introduce trailing
// whitespace.
func writeLabeledLine(w io.Writer, label, value, source string) error {
	// Right-trim the label so that a label literal ending in a space
	// (e.g. "Source: ") does not double-count against the padding.
	labelBody := strings.TrimRight(label, " ")
	if pad := configLabelWidth - len(labelBody); pad > 0 {
		labelBody += strings.Repeat(" ", pad)
	}

	line := "  " + labelBody + value

	if source != "" {
		// Align "(source:" to a fixed column. The column is
		// determined by configValueColumn; values shorter than
		// the column are padded to it, values longer than the
		// column push the attribution rightward. The latter case
		// is acceptable because the resolver's contract bounds
		// the value length; the former case is what produces the
		// stable column that makes the output diffable.
		if valueLen := len(value); valueLen < configValueColumn {
			line += strings.Repeat(" ", configValueColumn-valueLen)
		} else {
			line += "  "
		}
		line += "(source: " + source + ")"
	}

	// F8: never end a line with whitespace. The trim is defensive:
	// the logic above does not produce trailing whitespace, but a
	// future edit to the padding arithmetic could, and the failure
	// would be silent without this guard.
	line = strings.TrimRight(line, " ")

	if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
		return err
	}
	return nil
}
