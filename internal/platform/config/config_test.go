package config

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestExpected struct {
	port     int
	maxBytes int64
	maxRows  int
	logLevel slog.Level
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name     string
		port     string
		maxBytes string
		maxRows  string
		logLevel string
		expected TestExpected
	}{
		{
			name:     "Validate Env Happy Path",
			port:     "8080",
			maxBytes: "2097152",
			maxRows:  "1000",
			logLevel: "DEBUG",
			expected: TestExpected{
				port:     8080,
				maxBytes: int64(2097152),
				maxRows:  1000,
				logLevel: slog.LevelDebug,
			},
		},
		{
			name: "Validate Env Default Values",
			port: "3000",
			expected: TestExpected{
				port:     3000,
				maxBytes: int64(2097152),
				maxRows:  10_000,
				logLevel: slog.LevelInfo,
			},
		},
		{
			name:     "Caps at their ceiling are accepted",
			port:     "8080",
			maxBytes: "10485760",
			maxRows:  "100000",
			expected: TestExpected{
				port:     8080,
				maxBytes: int64(10485760),
				maxRows:  100_000,
				logLevel: slog.LevelInfo,
			},
		},
		{
			name:     "Surrounding whitespace is trimmed",
			port:     " 8080\n",
			maxBytes: "  2097152  ",
			maxRows:  "\t1000",
			logLevel: " warn ",
			expected: TestExpected{
				port:     8080,
				maxBytes: int64(2097152),
				maxRows:  1000,
				logLevel: slog.LevelWarn,
			},
		},
		{
			name:     "Log level names are case insensitive",
			port:     "8080",
			logLevel: "error",
			expected: TestExpected{
				port:     8080,
				maxBytes: int64(2097152),
				maxRows:  10_000,
				logLevel: slog.LevelError,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			config, err := validate(Raw{
				Port:     tt.port,
				MaxBytes: tt.maxBytes,
				MaxRows:  tt.maxRows,
				LogLevel: tt.logLevel,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.expected.port, config.Port)
			assert.Equal(t, tt.expected.maxBytes, config.MaxBytes)
			assert.Equal(t, tt.expected.maxRows, config.MaxRows)
			assert.Equal(t, tt.expected.logLevel, config.LogLevel)
		})
	}
}

func TestConfigValidateErrors(t *testing.T) {
	tests := []struct {
		name     string
		raw      Raw
		wantIs   error
		wantText []string
	}{
		{
			name:     "PORT unset is required",
			raw:      Raw{},
			wantIs:   ErrRequired,
			wantText: []string{EnvPort},
		},
		{
			name:     "PORT empty is treated as unset",
			raw:      Raw{Port: "   "},
			wantIs:   ErrRequired,
			wantText: []string{EnvPort},
		},
		{
			name:     "PORT not a number",
			raw:      Raw{Port: "banana"},
			wantIs:   ErrInvalid,
			wantText: []string{EnvPort, "not a number"},
		},
		{
			name:     "PORT zero is rejected",
			raw:      Raw{Port: "0"},
			wantIs:   ErrInvalid,
			wantText: []string{EnvPort, "outside 1-65535"},
		},
		{
			name:     "PORT above the valid range",
			raw:      Raw{Port: "99999"},
			wantIs:   ErrInvalid,
			wantText: []string{EnvPort, "outside 1-65535"},
		},
		{
			name:     "MAX_BYTES not a number",
			raw:      Raw{Port: "8080", MaxBytes: "2MB"},
			wantIs:   ErrInvalid,
			wantText: []string{EnvMaxBytes, "not a number"},
		},
		{
			name:     "MAX_BYTES zero",
			raw:      Raw{Port: "8080", MaxBytes: "0"},
			wantIs:   ErrInvalid,
			wantText: []string{EnvMaxBytes, "must be positive"},
		},
		{
			name:     "MAX_BYTES negative",
			raw:      Raw{Port: "8080", MaxBytes: "-1"},
			wantIs:   ErrInvalid,
			wantText: []string{EnvMaxBytes, "must be positive"},
		},
		{
			name:     "MAX_BYTES above its ceiling",
			raw:      Raw{Port: "8080", MaxBytes: "10485761"},
			wantIs:   ErrInvalid,
			wantText: []string{EnvMaxBytes, "exceeds the maximum"},
		},
		{
			name:     "MAX_ROWS zero",
			raw:      Raw{Port: "8080", MaxRows: "0"},
			wantIs:   ErrInvalid,
			wantText: []string{EnvMaxRows, "must be positive"},
		},
		{
			name:     "MAX_ROWS above its ceiling",
			raw:      Raw{Port: "8080", MaxRows: "100001"},
			wantIs:   ErrInvalid,
			wantText: []string{EnvMaxRows, "exceeds the maximum"},
		},
		{
			name:     "LOG_LEVEL unknown name",
			raw:      Raw{Port: "8080", LogLevel: "shouty"},
			wantIs:   ErrInvalid,
			wantText: []string{EnvLogLevel},
		},
		{
			name:     "every bad field is reported, not just the first",
			raw:      Raw{Port: "banana", MaxBytes: "-1", MaxRows: "0", LogLevel: "shouty"},
			wantIs:   ErrInvalid,
			wantText: []string{EnvPort, EnvMaxBytes, EnvMaxRows, EnvLogLevel},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			config, err := validate(tt.raw)

			require.Error(t, err)
			assert.ErrorIs(t, err, tt.wantIs)
			for _, want := range tt.wantText {
				assert.ErrorContains(t, err, want)
			}
			assert.Equal(t, Config{}, config,
				"a failed validate must return the zero Config, never a partial one")
		})
	}
}

func TestReadEnv(t *testing.T) {
	t.Setenv(EnvPort, "9090")
	t.Setenv(EnvMaxBytes, "4194304")
	t.Setenv(EnvMaxRows, "500")
	t.Setenv(EnvLogLevel, "warn")

	assert.Equal(t, Raw{
		Port:     "9090",
		MaxBytes: "4194304",
		MaxRows:  "500",
		LogLevel: "warn",
	}, readEnv())
}

func TestLoad(t *testing.T) {
	t.Setenv(EnvPort, "8080")
	t.Setenv(EnvMaxBytes, "")
	t.Setenv(EnvMaxRows, "")
	t.Setenv(EnvLogLevel, "")

	config, err := Load()

	require.NoError(t, err)
	assert.Equal(t, Config{
		Port:     8080,
		MaxBytes: defaultMaxBytes,
		MaxRows:  defaultMaxRows,
		LogLevel: defaultLogLevel,
	}, config)
}

func TestLoadMissingPort(t *testing.T) {
	t.Setenv(EnvPort, "")

	_, err := Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRequired)
	assert.ErrorContains(t, err, EnvPort)
}
