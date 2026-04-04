package plugin

import "testing"

func FuzzCheckSDKCompatibility(f *testing.F) {
	f.Add("1.0.0")
	f.Add("^1.0.0")
	f.Add("2.0.0")
	f.Add("")
	f.Add("not-a-version")
	f.Add("^~>=1.2.3")
	f.Add("99999.99999.99999")

	f.Fuzz(func(t *testing.T, version string) {
		// checkSDKCompatibility must never panic regardless of input.
		_ = checkSDKCompatibility(version)
	})
}
