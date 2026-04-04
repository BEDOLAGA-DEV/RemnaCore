package plugin

import "testing"

func FuzzParseManifest(f *testing.F) {
	// Seed corpus with a valid full manifest.
	f.Add([]byte(`[plugin]
id = "test"
name = "Test"
version = "1.0.0"
sdk_version = "1.0.0"

[hooks]
sync = ["payment.create_charge"]
`))

	// Seed with a minimal valid manifest.
	f.Add([]byte(`[plugin]
id = "x"
name = "X"
version = "0.0.1"
sdk_version = "1.0.0"
[hooks]
async = ["test.event"]
`))

	// Seed with clearly invalid data.
	f.Add([]byte(`not toml at all`))
	f.Add([]byte{})
	f.Add([]byte(`[plugin]`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// ParseManifest must never panic regardless of input.
		manifest, err := ParseManifest(data)
		if err != nil {
			return // expected for invalid input
		}
		// If parsing succeeded, public methods must not panic.
		manifest.Validate()
		manifest.EffectiveLimits()
		manifest.ParsePermissions()
		manifest.HookRegistrations("test-id")
	})
}
