package infra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractCountryCode(t *testing.T) {
	tests := []struct {
		name     string
		nodeName string
		want     string
	}{
		{name: "standard format", nodeName: "US-NewYork-01", want: "US"},
		{name: "two letter prefix", nodeName: "DE-Berlin-01", want: "DE"},
		{name: "short name", nodeName: "JP-1", want: "JP"},
		{name: "no dash", nodeName: "ABCDE", want: "XX"},
		{name: "empty string", nodeName: "", want: "XX"},
		{name: "single char", nodeName: "X", want: "XX"},
		{name: "two chars no dash", nodeName: "XY", want: "XX"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCountryCode(tt.nodeName)
			assert.Equal(t, tt.want, got)
		})
	}
}
