package logger

// Source identifies where a resolved level came from. It is used in
// diagnostics and in tests that assert which precedence rule fired.
//
// Sources are ordered by precedence, highest first:
//
//	SourceFlagExplicit  --log-level=<level>
//	SourceFlagVerbose   --verbose
//	SourceFlagQuiet     --quiet
//	SourceEnv           FORGE_LOG_LEVEL
//	SourceConfig        config file log.level
//	SourceDefault       (nothing set)
type Source int

const (
	// SourceDefault is the fallback when no source specifies a level.
	SourceDefault Source = iota

	// SourceConfig is the config file's log.level field.
	SourceConfig

	// SourceEnv is the FORGE_LOG_LEVEL environment variable.
	SourceEnv

	// SourceFlagQuiet is the --quiet flag (implies ERROR).
	SourceFlagQuiet

	// SourceFlagVerbose is the --verbose flag (implies DEBUG).
	SourceFlagVerbose

	// SourceFlagExplicit is the --log-level=<level> flag. Highest.
	SourceFlagExplicit
)

// String returns a short lowercase name of the source.
func (s Source) String() string {
	switch s {
	case SourceFlagExplicit:
		return "flag(--log-level)"
	case SourceFlagVerbose:
		return "flag(--verbose)"
	case SourceFlagQuiet:
		return "flag(--quiet)"
	case SourceEnv:
		return "env(FORGE_LOG_LEVEL)"
	case SourceConfig:
		return "config(log.level)"
	case SourceDefault:
		return "default"
	default:
		return "unknown"
	}
}

// Inputs collects every source that may specify a log level.
//
// A nil pointer or empty string means "source did not specify". Callers
// populate Inputs from the parsed flags, environment, and config, and
// pass it to ResolveLevel.
//
// Inputs is deliberately a plain struct (not an interface) so that
// resolution is cheap, testable, and free of hidden dependencies.
type Inputs struct {
	// LogLevel is the value of --log-level. Empty means unset.
	// Accepted: debug, info, warn, warning, error (case-insensitive).
	LogLevel string

	// Verbose is true if --verbose was passed.
	Verbose bool

	// Quiet is true if --quiet was passed.
	Quiet bool

	// EnvLevel is the value of FORGE_LOG_LEVEL. Empty means unset.
	EnvLevel string

	// ConfigLevel is the value of log.level in the config file.
	// Empty means unset.
	ConfigLevel string
}

// Decision is the result of level resolution: the chosen level and the
// source that provided it. Returning the source makes precedence
// observable, which is what tests need to assert the chain.
type Decision struct {
	// Level is the effective log level.
	Level Level

	// Source identifies which input won.
	Source Source
}

// ResolveLevel selects the effective log level from the given inputs.
//
// Precedence, highest first:
//
//  1. --log-level=<level>   (explicit; wins over implicating flags)
//  2. --verbose             (implies DEBUG)
//  3. --quiet               (implies ERROR)
//  4. FORGE_LOG_LEVEL       (env var)
//  5. log.level             (config file)
//  6. default               (INFO)
//
// An unparseable value at a higher-precedence source returns an error;
// the resolver does NOT fall through to a lower source. This is
// deliberate: a typo'd --log-level should be a usage error, not a silent
// fallback to the default.
//
// ResolveLevel does not itself enforce the --quiet/--verbose mutual
// exclusion; that is the CLI's responsibility (it needs to produce a
// usage error with the right exit code before command logic runs). If
// both are set, --verbose wins here, but callers should reject the
// combination upstream.
func ResolveLevel(in Inputs) (Decision, error) {
	// 1. Explicit flag.
	if in.LogLevel != "" {
		lvl, err := ParseLevel(in.LogLevel)
		if err != nil {
			return Decision{}, err
		}
		return Decision{Level: lvl, Source: SourceFlagExplicit}, nil
	}

	// 2. --verbose.
	if in.Verbose {
		return Decision{Level: LevelDebug, Source: SourceFlagVerbose}, nil
	}

	// 3. --quiet.
	if in.Quiet {
		return Decision{Level: LevelError, Source: SourceFlagQuiet}, nil
	}

	// 4. Env var.
	if in.EnvLevel != "" {
		lvl, err := ParseLevel(in.EnvLevel)
		if err != nil {
			return Decision{}, err
		}
		return Decision{Level: lvl, Source: SourceEnv}, nil
	}

	// 5. Config file.
	if in.ConfigLevel != "" {
		lvl, err := ParseLevel(in.ConfigLevel)
		if err != nil {
			return Decision{}, err
		}
		return Decision{Level: lvl, Source: SourceConfig}, nil
	}

	// 6. Default.
	return Decision{Level: LevelInfo, Source: SourceDefault}, nil
}
