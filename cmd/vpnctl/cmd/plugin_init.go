package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/BEDOLAGA-DEV/RemnaCore/plugins/templates"
)

// Flag names and defaults for plugin init.
const (
	FlagInitLang  = "lang"
	FlagInitName  = "name"
	FlagInitHooks = "hooks"
	FlagInitKind  = "kind"

	DefaultInitLang = "go"
	DefaultInitKind = "hook"

	KindHook = "hook"
	KindBot  = "bot"
)

// HookInfo holds computed naming variants for a single hook.
type HookInfo struct {
	Snake  string // e.g. "pricing.calculate"
	Pascal string // e.g. "PricingCalculate"
}

// initData is the template context passed to every scaffold file.
type initData struct {
	Name        string
	Description string
	Hooks       []HookInfo
	HooksList   string // comma-separated quoted hook names for TOML
}

var pluginInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a new plugin project",
	Long: `Generate a plugin project scaffold with boilerplate code,
manifest, Makefile, tests, and documentation.

Example:
  vpnctl plugin init --lang go --name my-plugin --hooks pricing.calculate,invoice.created`,
	RunE: runPluginInit,
}

func init() {
	pluginCmd.AddCommand(pluginInitCmd)
	pluginInitCmd.Flags().String(FlagInitLang, DefaultInitLang, "plugin language (go)")
	pluginInitCmd.Flags().String(FlagInitName, "", "plugin name (required)")
	pluginInitCmd.Flags().String(FlagInitHooks, "", "comma-separated hook names (required for --kind hook)")
	pluginInitCmd.Flags().String(FlagInitKind, DefaultInitKind, "plugin kind: hook | bot")

	_ = pluginInitCmd.MarkFlagRequired(FlagInitName)
}

func runPluginInit(cmd *cobra.Command, _ []string) error {
	lang, _ := cmd.Flags().GetString(FlagInitLang)
	name, _ := cmd.Flags().GetString(FlagInitName)
	hooks, _ := cmd.Flags().GetString(FlagInitHooks)
	kind, _ := cmd.Flags().GetString(FlagInitKind)

	if lang != "go" {
		return fmt.Errorf("unsupported language %q; only \"go\" is supported", lang)
	}
	switch kind {
	case KindHook:
		// fall through to the hook flow below
	case KindBot:
		return scaffoldBot(cmd, name)
	default:
		return fmt.Errorf("unsupported kind %q; use \"hook\" or \"bot\"", kind)
	}

	hookNames := strings.Split(hooks, ",")
	for i := range hookNames {
		hookNames[i] = strings.TrimSpace(hookNames[i])
	}

	hookInfos := make([]HookInfo, 0, len(hookNames))
	quoted := make([]string, 0, len(hookNames))
	for _, h := range hookNames {
		if h == "" {
			continue
		}
		hookInfos = append(hookInfos, HookInfo{
			Snake:  h,
			Pascal: toPascal(h),
		})
		quoted = append(quoted, fmt.Sprintf("%q", h))
	}

	if len(hookInfos) == 0 {
		return fmt.Errorf("at least one hook name is required")
	}

	data := initData{
		Name:        name,
		Description: "A RemnaCore plugin.",
		Hooks:       hookInfos,
		HooksList:   strings.Join(quoted, ", "),
	}

	outDir := filepath.Join(".", name)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	files := []struct {
		tmpl string
		out  string
	}{
		{"go/main.go.tmpl", "main.go"},
		{"go/main_test.go.tmpl", "main_test.go"},
		{"go/Makefile.tmpl", "Makefile"},
		{"go/plugin.toml.tmpl", "plugin.toml"},
		{"go/README.md.tmpl", "README.md"},
	}

	for _, f := range files {
		if err := renderTemplate(f.tmpl, filepath.Join(outDir, f.out), data); err != nil {
			return fmt.Errorf("rendering %s: %w", f.out, err)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Plugin scaffolded in ./%s\n", name)
	return nil
}

// scaffoldBot generates a botsdk-based Telegram bot plugin project.
func scaffoldBot(cmd *cobra.Command, name string) error {
	data := initData{
		Name:        name,
		Description: "A RemnaCore Telegram bot plugin.",
	}

	outDir := filepath.Join(".", name)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	files := []struct {
		tmpl string
		out  string
		exec bool
	}{
		{"bot/main.go.tmpl", "main.go", false},
		{"bot/go.mod.tmpl", "go.mod", false},
		{"bot/plugin.toml.tmpl", "plugin.toml", false},
		{"bot/build.sh.tmpl", "build.sh", true},
		{"bot/README.md.tmpl", "README.md", false},
	}
	for _, f := range files {
		outPath := filepath.Join(outDir, f.out)
		if err := renderTemplate(f.tmpl, outPath, data); err != nil {
			return fmt.Errorf("rendering %s: %w", f.out, err)
		}
		if f.exec {
			_ = os.Chmod(outPath, 0o755)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Bot plugin scaffolded in ./%s (build with ./build.sh)\n", name)
	return nil
}

func renderTemplate(tmplPath, outPath string, data any) error {
	raw, err := templates.FS.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("reading embedded template %s: %w", tmplPath, err)
	}

	t, err := template.New(filepath.Base(tmplPath)).Parse(string(raw))
	if err != nil {
		return fmt.Errorf("parsing template %s: %w", tmplPath, err)
	}

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", outPath, err)
	}
	defer out.Close()

	return t.Execute(out, data)
}

// toPascal converts a dotted/underscored/dashed hook name to PascalCase.
// "pricing.calculate" -> "PricingCalculate"
// "invoice_created"   -> "InvoiceCreated"
func toPascal(s string) string {
	var b strings.Builder
	upper := true
	for _, r := range s {
		if r == '.' || r == '_' || r == '-' {
			upper = true
			continue
		}
		if upper {
			b.WriteRune(unicode.ToUpper(r))
			upper = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
