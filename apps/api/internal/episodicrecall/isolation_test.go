package episodicrecall

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/bodysense/api"

// This test is the structural half of the S4 hard isolation. Diagnosis,
// Safety and TrainingPlan inputs are assembled in internal/service and
// internal/domain/safety; neither may import this package, so no recall result
// can be constructed where those inputs are built. The only production
// importers are the consultation context assembly and its server wiring; the
// offline benchmark may import the tokenizer but has no runtime path.
func TestOnlyConsultationContextImportsEpisodicRecall(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	allowed := map[string]bool{
		"internal/consultation": true,
		"cmd/server":            true,
		"benchmarks/episodic":   true,
	}
	forbidden := []string{
		"internal/service",
		"internal/domain/safety",
		"internal/transport",
		"internal/agent",
	}

	var importers []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "vendor", "generated":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		parsed, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, spec := range parsed.Imports {
			imported, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil || imported != modulePath+"/internal/episodicrecall" {
				continue
			}
			rel, relErr := filepath.Rel(root, filepath.Dir(path))
			if relErr != nil {
				return relErr
			}
			importers = append(importers, filepath.ToSlash(rel))
			break
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan module imports: %v", err)
	}
	sort.Strings(importers)

	for _, importer := range importers {
		if !allowed[importer] {
			t.Errorf("unexpected importer of episodicrecall: %s", importer)
		}
	}
	for _, prefix := range forbidden {
		for _, importer := range importers {
			if importer == prefix || strings.HasPrefix(importer, prefix+"/") {
				t.Errorf("clinical input package imported episodicrecall: %s", importer)
			}
		}
	}
	if len(importers) == 0 {
		t.Fatalf("no importers found; the isolation test is not observing the module")
	}
}

// The recall package itself must stay below the business layer: importing the
// package that owns Diagnosis/Safety/TrainingPlan inputs would create the
// firewall in reverse.
func TestEpisodicRecallDoesNotImportClinicalServices(t *testing.T) {
	entries, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range entries {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, spec := range parsed.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			switch imported {
			case modulePath + "/internal/service", modulePath + "/internal/domain/safety":
				t.Fatalf("%s imports clinical service package %s", file, imported)
			}
		}
	}
}
