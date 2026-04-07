package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckSDKCompatibility_Compatible(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"exact match", "1.0.0"},
		{"caret constraint", "^1.0.0"},
		{"tilde constraint", "~1.0.0"},
		{"higher minor", "1.2.3"},
		{"higher patch", "1.0.99"},
		{"major only", "1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.NoError(t, checkSDKCompatibility(tc.version))
		})
	}
}

func TestCheckSDKCompatibility_Incompatible(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"major version 2", "2.0.0"},
		{"caret major version 2", "^2.0.0"},
		{"major version 0", "0.9.0"},
		{"major version 3", "3.1.0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkSDKCompatibility(tc.version)
			assert.ErrorIs(t, err, ErrIncompatibleSDK)
		})
	}
}

func TestCheckSDKCompatibility_Empty(t *testing.T) {
	err := checkSDKCompatibility("")
	assert.ErrorIs(t, err, ErrInvalidManifest)
}

func TestCheckSDKCompatibility_InvalidFormat(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"not a number", "abc.1.0"},
		{"only constraint chars", "^~"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkSDKCompatibility(tc.version)
			assert.ErrorIs(t, err, ErrInvalidManifest)
		})
	}
}

func TestParseMajorVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected int
		wantErr  bool
	}{
		{"simple", "1.0.0", 1, false},
		{"caret", "^2.3.4", 2, false},
		{"tilde", "~0.1.0", 0, false},
		{"gte", ">=3.0.0", 3, false},
		{"major only", "5", 5, false},
		{"v prefix", "v1.2.3", 1, false},
		{"v prefix major only", "v3", 3, false},
		{"caret with v prefix", "^v2.0.0", 2, false},
		{"prerelease suffix", "1.0.0-beta.1", 1, false},
		{"build metadata", "1.0.0+build.123", 1, false},
		{"large major", "42.0.0", 42, false},
		{"zero major", "0.9.1", 0, false},
		{"major minor only", "3.4", 3, false},
		{"empty", "", 0, true},
		{"not a number", "abc", 0, true},
		{"only constraints", "^~>=", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMajorVersion(tc.version)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, got)
			}
		})
	}
}

func TestCheckSDKCompatibility_SemverEdgeCases(t *testing.T) {
	// CurrentSDKVersion is "1.0.0" -- all compatible versions must share major 1.
	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{"v prefix compatible", "v1.0.0", false},
		{"v prefix minor bump", "v1.5.0", false},
		{"prerelease compatible", "1.0.0-alpha.1", false},
		{"build metadata compatible", "1.0.0+20260407", false},
		{"high minor and patch", "1.99.99", false},
		{"v prefix incompatible", "v2.0.0", true},
		{"prerelease incompatible", "2.0.0-rc.1", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkSDKCompatibility(tc.version)
			if tc.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrIncompatibleSDK)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
