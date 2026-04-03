package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInput_ReturnsNilOutsideWASM(t *testing.T) {
	// Outside of WASM the stub returns nil.
	result := Input()
	assert.Nil(t, result)
}

func TestOutput_DoesNotPanicOutsideWASM(t *testing.T) {
	// Ensure Output does not panic when called outside WASM.
	assert.NotPanics(t, func() {
		Output(HookResult{Action: ActionContinue})
	})
}
