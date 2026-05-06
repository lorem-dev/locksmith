// render-install renders .scripts/install-template/install.sh.tmpl into
// dist/install.sh, substituting owner/repo/binary/platform list at
// build time. The rendered script is published as a GitHub-release
// asset and is the canonical install path documented in README.md.
//
// Usage:
//
//	go run ./.scripts/render-install [-template <path>] [-o <path>]
//
// Defaults: -template .scripts/install-template/install.sh.tmpl,
// -o dist/install.sh.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// templateData is what the template uses. Values are constant across
// versions of locksmith - the install script itself resolves the
// version at runtime.
type templateData struct {
	Owner              string
	Repo               string
	BinaryName         string
	GoModulePath       string
	SupportedPlatforms string
}

func defaultData() templateData {
	platforms := []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64"}
	return templateData{
		Owner:              "lorem-dev",
		Repo:               "locksmith",
		BinaryName:         "locksmith",
		GoModulePath:       "github.com/lorem-dev/locksmith/cmd/locksmith",
		SupportedPlatforms: strings.Join(platforms, " "),
	}
}

func main() {
	var (
		tmplFlag = flag.String("template", ".scripts/install-template/install.sh.tmpl", "path to template")
		outFlag  = flag.String("o", "dist/install.sh", "output path")
	)
	flag.Parse()

	if err := renderTemplate(*tmplFlag, *outFlag, defaultData()); err != nil {
		fmt.Fprintf(os.Stderr, "render-install: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("rendered %s\n", *outFlag)
}

// renderTemplate parses the template at tmplPath, executes it with
// data, and writes the result to outPath with mode 0755.
func renderTemplate(tmplPath, outPath string, data templateData) error {
	body, err := os.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	tmpl, err := template.New("install").Option("missingkey=error").Parse(string(body))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(outPath), err)
	}

	out, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	defer out.Close()

	if err := tmpl.Execute(out, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}
	return nil
}
