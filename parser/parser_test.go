package parser_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/yamlstar/yamlstar-plugin-parser-csv/parser"
)

func TestInvalidUTF8(t *testing.T) {
	if _, err := parser.Parse([]byte{0xff}, nil); err == nil {
		t.Fatal("accepted invalid UTF-8")
	}
}

func TestEmptyEnvironment(t *testing.T) {
	t.Setenv("YAMLSTAR_CSV_FORMAT", "")
	if _, err := parser.Parse(nil, nil); err == nil {
		t.Fatal("accepted an empty format environment variable")
	}
	if _, err := parser.Parse(nil, map[string]any{"format": "csv"}); err != nil {
		t.Fatalf("explicit format did not override environment: %v", err)
	}
}

func TestEventOrderAndTags(t *testing.T) {
	t.Setenv("YAMLSTAR_CSV_FORMAT", "auto")
	events, err := parser.Parse([]byte("z,a\n001,true\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	var values []string
	for _, event := range events {
		if event.Type == "scalar" {
			if event.Tag != "tag:yaml.org,2002:str" {
				t.Fatalf("scalar tag: %#v", event)
			}
			values = append(values, event.Value)
		}
	}
	if strings.Join(values, "|") != "z|001|a|true" {
		t.Fatalf("column order: %v", values)
	}
}

func TestEDNOptionsAndEmptyValues(t *testing.T) {
	t.Setenv("YAMLSTAR_CSV_FORMAT", "auto")
	encoded, err := parser.ParseEDN([]byte("name,value\napi,\n"), []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `:value ""`) {
		t.Fatalf("lost empty value: %s", encoded)
	}
	for _, options := range []string{"nil", "[]", "{:format", "{:unknown true}"} {
		if _, err := parser.ParseEDN(nil, []byte(options)); err == nil {
			t.Fatalf("accepted options %q", options)
		}
	}
}

func TestEDNUnicode(t *testing.T) {
	t.Setenv("YAMLSTAR_CSV_FORMAT", "auto")
	encoded, err := parser.ParseEDN([]byte("名前,city\n雪,Montréal\n"), []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.Valid(encoded) {
		t.Fatalf("invalid UTF-8 event stream: %q", encoded)
	}
	for _, value := range []string{"名前", "雪", "Montréal"} {
		if !strings.Contains(string(encoded), value) {
			t.Fatalf("lost %q: %s", value, encoded)
		}
	}
}
