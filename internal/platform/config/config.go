// Package config loads and validates the service's environment configuration.
// Loading is separated from exiting: nothing here calls os.Exit or logs. The caller decies what to do with a failure.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Environment variable names, export so callers and tests share one spelling.
const (
	EnvPort     = "PORT"
	EnvMaxBytes = "MAX_BYTES"
	EnvMaxRows  = "MAX_ROWS"
	EnvLogLevel = "LOG_LEVEL"
)

// Defaults per ADR-0005, sized for the expected workload of CSV Documents
// under ~1,000 Rows.
const (
	defaultMaxBytes int64 = 2 * 1024 * 1024
	defaultMaxRows        = 10_000
	defaultLogLevel       = slog.LevelInfo
)

// Ceilings on the two configurable caps.
const (
	ceilingMaxBytes int64 = 10 * 1024 * 1024
	ceilingMaxRows        = 100_000
)

// Failure kinds. Callers and tests match these with `errors.Is` rather than comparing message text.
var (
	ErrRequired = errors.New("required but not set")
	ErrInvalid  = errors.New("invalid value")
)

type (
	// Raw holds unvalidated strings exactly as read from the environment.
	Raw struct {
		Port     string
		MaxBytes string
		MaxRows  string
		LogLevel string
	}

	// Config is the validated result.
	// A Config that exists has passed every check in validate so no code downstream re-verifies these values.
	Config struct {
		Port     int
		MaxBytes int64
		MaxRows  int
		LogLevel slog.Level
	}
)

// Load reads the environment and validates it, returning every problem it finds rather than stopping at first.
// If no errors, the Config validated result is returned.
func Load() (Config, error) {
	return validate(readEnv())
}

func readEnv() Raw {
	return Raw{
		Port:     os.Getenv(EnvPort),
		MaxBytes: os.Getenv(EnvMaxBytes),
		MaxRows:  os.Getenv(EnvMaxRows),
		LogLevel: os.Getenv(EnvLogLevel),
	}
}

func validate(raw Raw) (Config, error) {
	var errs []error

	port, err := parsePort(raw.Port)
	if err != nil {
		errs = append(errs, err)
	}

	maxBytes, err := parseBoundedInt64(EnvMaxBytes, raw.MaxBytes, defaultMaxBytes, ceilingMaxBytes)
	if err != nil {
		errs = append(errs, err)
	}

	maxRows, err := parseBoundedInt(EnvMaxRows, raw.MaxRows, defaultMaxRows, ceilingMaxRows)
	if err != nil {
		errs = append(errs, err)
	}

	logLevel, err := parseLevel(raw.LogLevel, defaultLogLevel)
	if err != nil {
		errs = append(errs, err)
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}

	config := Config{
		Port:     port,
		MaxBytes: maxBytes,
		MaxRows:  maxRows,
		LogLevel: logLevel,
	}

	return config, nil
}

func parsePort(raw string) (int, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, fmt.Errorf("%s: %w", EnvPort, ErrRequired)
	}
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%s: %w: %q is not a number", EnvPort, ErrInvalid, s)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s: %w: %d is outside 1-65535", EnvPort, ErrInvalid, port)
	}
	return port, nil
}

func parseBoundedInt64(name, raw string, def, upper int64) (int64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return def, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w: %q is not a number", name, ErrInvalid, s)
	}
	if v < 1 {
		return 0, fmt.Errorf("%s: %w: must be positive, got %d", name, ErrInvalid, v)
	}
	if v > upper {
		return 0, fmt.Errorf("%s: %w: %d exceeds the maximum of %d", name, ErrInvalid, v, upper)
	}
	return v, nil
}

func parseBoundedInt(name, raw string, def, upper int) (int, error) {
	v, err := parseBoundedInt64(name, raw, int64(def), int64(upper))
	if err != nil {
		return 0, err
	}
	return int(v), nil
}

func parseLevel(raw string, def slog.Level) (slog.Level, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return def, nil
	}

	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(s)); err != nil {
		return def, fmt.Errorf("%s: %w: %w", EnvLogLevel, ErrInvalid, err)
	}
	return lvl, nil
}
