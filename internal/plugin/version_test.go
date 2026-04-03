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
		{"empty", "", 0, true},
		{"not a number", "abc", 0, true},
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
