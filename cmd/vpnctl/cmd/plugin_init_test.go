package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToPascal(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"pricing.calculate", "PricingCalculate"},
		{"invoice_created", "InvoiceCreated"},
		{"user-signup", "UserSignup"},
		{"simple", "Simple"},
		{"a.b.c", "ABC"},
		{"already_PascalCase", "AlreadyPascalCase"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toPascal(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderTemplate(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "plugin.toml")

	data := initData{
		Name:        "test-plugin",
		Description: "A test plugin.",
		Hooks: []HookInfo{
			{Snake: "pricing.calculate", Pascal: "PricingCalculate"},
		},
		HooksList: `"pricing.calculate"`,
	}

	err := renderTemplate("go/plugin.toml.tmpl", outPath, data)
	require.NoError(t, err)

	content, err := os.ReadFile(outPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), `id          = "test-plugin"`)
	assert.Contains(t, string(content), `"pricing.calculate"`)
}

func TestRenderTemplate_MainGo(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "main.go")

	data := initData{
		Name:        "my-plugin",
		Description: "My plugin.",
		Hooks: []HookInfo{
			{Snake: "pricing.calculate", Pascal: "PricingCalculate"},
			{Snake: "invoice.created", Pascal: "InvoiceCreated"},
		},
		HooksList: `"pricing.calculate", "invoice.created"`,
	}

	err := renderTemplate("go/main.go.tmpl", outPath, data)
	require.NoError(t, err)

	content, err := os.ReadFile(outPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "//export on_pricing.calculate")
	assert.Contains(t, string(content), "func onPricingCalculate()")
	assert.Contains(t, string(content), "//export on_invoice.created")
	assert.Contains(t, string(content), "func onInvoiceCreated()")
	assert.Contains(t, string(content), "func main() {}")
}

func TestRunPluginInit_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	rootCmd.SetArgs([]string{"plugin", "init", "--name", "test-scaffold", "--hooks", "event.fire"})
	err = rootCmd.Execute()
	require.NoError(t, err)

	expectedFiles := []string{
		"main.go",
		"main_test.go",
		"Makefile",
		"plugin.toml",
		"README.md",
	}
	for _, f := range expectedFiles {
		path := filepath.Join(dir, "test-scaffold", f)
		_, statErr := os.Stat(path)
		assert.NoError(t, statErr, "expected file %s to exist", f)
	}
}
