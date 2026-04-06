.PHONY: build test clean

build:
	GOWORK=off GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm .

test:
	GOWORK=off go test ./... -v

clean:
	rm -f plugin.wasm
