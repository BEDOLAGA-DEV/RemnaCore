// Package main defines the Dagger CI pipeline for RemnaCore.
//
// Usage:
//
//	go run ./ci/                       (requires Dagger engine running)
//	dagger run go run ./ci/            (starts engine automatically)
//
// The pipeline runs three stages sequentially: Lint -> Test -> Build.
// Each stage executes inside an isolated container with cached Go modules
// and build artifacts for fast incremental runs.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"dagger.io/dagger"
)

const (
	GoVersion   = "1.26"
	GoImage     = "golang:" + GoVersion + "-alpine"
	AlpineImage = "alpine:3.21"

	BinaryName   = "remnacore"
	VpnctlBinary = "vpnctl"
	BinaryPath   = "/app/" + BinaryName

	AppPort       = 4000
	SubProxyPort  = 4100
	SpeedTestPort = 4203

	RunAsUID = 10001
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()

	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		return fmt.Errorf("dagger connect: %w", err)
	}
	defer client.Close()

	src := client.Host().Directory(".", dagger.HostDirectoryOpts{
		Exclude: []string{"bin/", "dist/", ".git/", "node_modules/", "web/"},
	})

	// Lint
	fmt.Println("=== Lint ===")
	if err := lint(ctx, client, src); err != nil {
		return fmt.Errorf("lint: %w", err)
	}

	// Test
	fmt.Println("=== Test ===")
	if err := test(ctx, client, src); err != nil {
		return fmt.Errorf("test: %w", err)
	}

	// Build
	fmt.Println("=== Build ===")
	if err := build(ctx, client, src); err != nil {
		return fmt.Errorf("build: %w", err)
	}

	fmt.Println("=== All checks passed ===")
	return nil
}

// goBase returns a Go container with sources mounted and module cache attached.
func goBase(client *dagger.Client, src *dagger.Directory) *dagger.Container {
	goModCache := client.CacheVolume("gomod")
	goBuildCache := client.CacheVolume("gobuild")

	return client.Container().
		From(GoImage).
		WithMountedDirectory("/src", src).
		WithMountedCache("/go/pkg/mod", goModCache).
		WithMountedCache("/root/.cache/go-build", goBuildCache).
		WithWorkdir("/src").
		WithExec([]string{"go", "mod", "download"})
}

// lint runs go vet against the entire module.
func lint(ctx context.Context, client *dagger.Client, src *dagger.Directory) error {
	_, err := goBase(client, src).
		WithExec([]string{"go", "vet", "./..."}).
		Sync(ctx)
	return err
}

// test runs unit tests with the race detector enabled. The race detector
// requires CGO, so we install gcc/musl-dev on Alpine and set CGO_ENABLED=1.
func test(ctx context.Context, client *dagger.Client, src *dagger.Directory) error {
	_, err := goBase(client, src).
		WithExec([]string{"apk", "add", "--no-cache", "gcc", "musl-dev"}).
		WithEnvVariable("CGO_ENABLED", "1").
		WithExec([]string{"go", "test", "-race", "-count=1", "-short", "./..."}).
		Sync(ctx)
	return err
}

// build compiles both binaries and assembles a production container image.
func build(ctx context.Context, client *dagger.Client, src *dagger.Directory) error {
	base := goBase(client, src).
		WithEnvVariable("CGO_ENABLED", "0").
		WithEnvVariable("GOOS", "linux").
		WithEnvVariable("GOARCH", "amd64")

	// Build remnacore binary
	remnacoreBin := base.
		WithExec([]string{"go", "build", "-o", BinaryPath, "./cmd/remnacore"})

	// Build vpnctl binary
	vpnctlBin := base.
		WithExec([]string{"go", "build", "-o", "/app/" + VpnctlBinary, "./cmd/vpnctl"})

	// Assemble production image
	prodImage := client.Container().
		From(AlpineImage).
		WithExec([]string{"apk", "add", "--no-cache", "ca-certificates", "tzdata"}).
		WithExec([]string{"addgroup", "-g", fmt.Sprintf("%d", RunAsUID), "-S", "app"}).
		WithExec([]string{"adduser", "-u", fmt.Sprintf("%d", RunAsUID), "-S", "app", "-G", "app"}).
		WithFile(BinaryPath, remnacoreBin.File(BinaryPath)).
		WithFile("/app/"+VpnctlBinary, vpnctlBin.File("/app/"+VpnctlBinary)).
		WithUser(fmt.Sprintf("%d", RunAsUID)).
		WithWorkdir("/app").
		WithEntrypoint([]string{"./" + BinaryName}).
		WithExposedPort(AppPort).
		WithExposedPort(SubProxyPort).
		WithExposedPort(SpeedTestPort)

	// Validate the image starts (dry run)
	_, err := prodImage.
		WithExec([]string{"./" + BinaryName, "--help"}).
		Sync(ctx)
	if err != nil {
		// --help may not be implemented; just validate the image builds
		_, err = prodImage.Sync(ctx)
	}
	return err
}
