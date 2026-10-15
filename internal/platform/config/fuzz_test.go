package config

import "testing"

func FuzzValidate(f *testing.F) {
	f.Add("8080", "2097152", "1000", "INFO")
	f.Add("", "", "", "")
	f.Add("banana", "-1", "0", "shouty")
	f.Add(" 8080\n", "10485760", "100000", "debug")
	f.Add("0", "99999999999", "100001", "INFO+2")

	f.Fuzz(func(t *testing.T, port, maxBytes, maxRows, logLevel string) {
		config, err := validate(Raw{
			Port:     port,
			MaxBytes: maxBytes,
			MaxRows:  maxRows,
			LogLevel: logLevel,
		})
		if err != nil {
			if config != (Config{}) {
				t.Fatalf("validate(%q, %q, %q, %q) returned %+v alongside error %v; want the zero Config",
					port, maxBytes, maxRows, logLevel, config, err)
			}
			return
		}

		if config.Port < 1 || config.Port > 65535 {
			t.Fatalf("port %d outside 1-65535 with a nil error (input %q)", config.Port, port)
		}
		if config.MaxBytes < 1 || config.MaxBytes > ceilingMaxBytes {
			t.Fatalf("maxBytes %d outside 1-%d with a nil error (input %q)",
				config.MaxBytes, ceilingMaxBytes, maxBytes)
		}
		if config.MaxRows < 1 || config.MaxRows > ceilingMaxRows {
			t.Fatalf("maxRows %d outside 1-%d with a nil error (input %q)",
				config.MaxRows, ceilingMaxRows, maxRows)
		}
	})
}
