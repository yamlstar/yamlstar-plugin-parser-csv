//go:build cgo

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/yamlstar/yamlstar-plugin-parser-csv/parser"
)

func TestManifest(t *testing.T) {
	data, err := os.ReadFile("../plugin.edn")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`:abi 2`, `:api "yaml-parser"`, `:name "csv"`,
		`:version "` + parser.Version + `"`, `:kind "event-source"`,
		`:event-format "yamlstar-events-edn-v1"`} {
		if !strings.Contains(manifest, field) || !strings.Contains(string(data), field) {
			t.Fatalf("manifest missing %s", field)
		}
	}
}

func TestConfiguredEvents(t *testing.T) {
	t.Setenv("YAMLSTAR_CSV_FORMAT", "csv")
	encoded, err := parseEvents([]byte("api\t80\n"), []byte(`{:format "tsv" :header false}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "mapping_start") ||
		!strings.Contains(string(encoded), `:value "80"`) {
		t.Fatalf("unexpected events: %s", encoded)
	}
}
