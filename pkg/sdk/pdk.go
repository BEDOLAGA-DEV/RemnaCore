package sdk

import "encoding/json"

// Input reads the hook context bytes from the host. In a real WASM environment
// this calls the pdk_input host function. Outside of WASM it returns nil.
//
// Plugin authors use this to receive the [HookContext] payload:
//
//	input := pdk.Input()
//	var ctx pdk.HookContext
//	json.Unmarshal(input, &ctx)
func Input() []byte {
	return pdkInput()
}

// Output writes the [HookResult] back to the host. In a real WASM environment
// this calls the pdk_output host function. Outside of WASM it is a no-op.
//
// Plugin authors use this to return their result:
//
//	result := pdk.HookResult{Action: pdk.ActionContinue}
//	pdk.Output(result)
func Output(result HookResult) {
	data, err := json.Marshal(result)
	if err != nil {
		return
	}
	pdkOutput(data)
}

// pdkInput is the platform-side implementation stub. When compiled to WASM
// the linker replaces this with the real host function import.
func pdkInput() []byte {
	return nil
}

// pdkOutput is the platform-side implementation stub. When compiled to WASM
// the linker replaces this with the real host function import.
func pdkOutput(_ []byte) {}
