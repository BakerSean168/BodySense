package main

import (
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var allowedRuntimeProtoConsumers = map[string]bool{
	"apps/api/internal/service/runtime_proto_adapter.go":      true,
	"apps/api/internal/service/consultation_runtime_event.go": true,
	"apps/api/internal/runtimeproto/contract_test.go":         true,
}

func normalize(path string) string {
	return filepath.ToSlash(path)
}

func allowedOpenAPIConsumer(rel string) bool {
	return strings.HasPrefix(rel, "apps/api/internal/transport/httpapi/") || rel == "apps/api/cmd/server/main.go"
}

func isDomainPackage(rel string) bool {
	return strings.HasPrefix(rel, "apps/api/internal/model/") ||
		strings.HasPrefix(rel, "apps/api/internal/repository/") ||
		strings.HasPrefix(rel, "apps/api/internal/service/")
}

func analyzeGoSource(rel string, source []byte) []string {
	rel = normalize(rel)
	if strings.HasPrefix(rel, "apps/api/internal/generated/") {
		return nil
	}

	file, err := parser.ParseFile(token.NewFileSet(), rel, source, parser.ImportsOnly)
	if err != nil {
		return []string{fmt.Sprintf("%s: parse error: %v", rel, err)}
	}

	var violations []string
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		switch {
		case strings.Contains(importPath, "/internal/generated/openapi/v1"):
			if !allowedOpenAPIConsumer(rel) {
				violations = append(violations, fmt.Sprintf("%s: generated OpenAPI types leaked outside HTTP transport boundary (%s)", rel, importPath))
			}
		case strings.Contains(importPath, "/internal/generated/runtimeproto/v1"):
			if !allowedRuntimeProtoConsumers[rel] {
				violations = append(violations, fmt.Sprintf("%s: generated runtime Proto leaked outside approved Go boundary adapters (%s)", rel, importPath))
			}
		case strings.Contains(importPath, "/internal/transport/httpapi"):
			if isDomainPackage(rel) {
				violations = append(violations, fmt.Sprintf("%s: domain package imports HTTP transport (%s)", rel, importPath))
			}
		}
	}
	return violations
}

func scanGoBoundaries(root string) ([]string, error) {
	apiRoot := filepath.Join(root, "apps/api")
	var violations []string
	err := filepath.WalkDir(apiRoot, func(full string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "generated" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, full)
		if err != nil {
			return err
		}
		source, err := os.ReadFile(full)
		if err != nil {
			return err
		}
		violations = append(violations, analyzeGoSource(rel, source)...)
		return nil
	})
	return violations, err
}

func main() {
	root := flag.String("root", ".", "repository root")
	flag.Parse()
	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	violations, err := scanGoBoundaries(absRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(violations) > 0 {
		fmt.Fprintln(os.Stderr, "Go architecture violations:")
		for _, violation := range violations {
			fmt.Fprintln(os.Stderr, violation)
		}
		os.Exit(1)
	}
	fmt.Println("GO_GENERATED_BOUNDARY=PASS")
}
