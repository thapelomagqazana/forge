package logger

import "os"

// EnvVar is the environment variable that sets the log level.
//
// It is read at process start and passed to ResolveLevel as
// Inputs.EnvLevel. The CLI owns the read; the logger package owns the
// parsing (via ParseLevel). See docs/reference/log-levels.md §4.
const EnvVar = "FORGE_LOG_LEVEL"

// EnvVarFormat is the environment variable that sets the log format.
//
// It is the env-var counterpart of --log-format. Accepted values are
// the same as the flag: "text" and "json". Empty or unset means "use
// the default (text) or the flag's value".
const EnvVarFormat = "FORGE_LOG_FORMAT"

// ReadEnv returns the log-related environment inputs.
//
// It reads FORGE_LOG_LEVEL and FORGE_LOG_FORMAT from the process
// environment. Unset variables are returned as empty strings, which
// ResolveLevel treats as "not specified".
//
// ReadEnv exists so that tests can substitute a fake environment by
// constructing Inputs directly, without touching os.Setenv (which is
// process-global and racy under -race with parallel tests).
func ReadEnv() (level, format string) {
	return os.Getenv(EnvVar), os.Getenv(EnvVarFormat)
}
