// Copyright 2026 YAMLStar Authors
// MIT License

// Package parser exposes the compiled YAMLScript CSV parser to Go callers.
package parser

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	ysruntime "github.com/gloathub/ys-v0-glj/runtime"
	"github.com/glojurelang/glojure/pkg/glj"
	"github.com/glojurelang/glojure/pkg/lang"
	_ "github.com/yamlstar/yamlstar-plugin-parser-csv/pkg/csv_parser/core"
)

const Version = "0.1.0"

// Event carries a portable YAML event between adapters.
type Event struct {
	Type  string
	Value string
	Tag   string
	Flow  bool
}

var runtimeLock sync.Mutex

// Parse returns a complete YAML event stream. Configuration is copied per call.
func Parse(input []byte, config map[string]any) (events []Event, err error) {
	err = withRuntime(input, func() {
		var entries []any
		for key, value := range config {
			entries = append(entries, lang.NewKeyword(key), value)
		}
		options := lang.NewMap(entries...)
		events = decodeEvents(invoke(input, options))
	})
	if err != nil {
		return nil, err
	}
	return events, nil
}

// ParseEDN accepts YAMLStar options and returns its EDN event representation.
func ParseEDN(input, options []byte) (output []byte, err error) {
	err = withRuntime(input, func() {
		config := glj.Read(string(options))
		var encoded strings.Builder
		encoded.WriteByte('[')
		for _, event := range decodeEvents(invoke(input, config)) {
			fmt.Fprintf(&encoded, "{:event %s", quote(event.Type))
			switch event.Type {
			case "scalar":
				fmt.Fprintf(&encoded, " :value %s :tag %s", quote(event.Value), quote(event.Tag))
			case "sequence_start", "mapping_start":
				encoded.WriteString(" :flow false")
			}
			encoded.WriteString("} ")
		}
		encoded.WriteByte(']')
		output = []byte(encoded.String())
	})
	return output, err
}

// JSON string escaping is also valid EDN, including Unicode and control bytes.
// Serialize in Go because the current runtime's pr-str truncates Unicode.
func quote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func decodeEvents(value any) (events []Event) {
	for seq := lang.Seq(value); seq != nil; seq = seq.Next() {
		value := seq.First()
		event := Event{
			Type: lang.Get(value, lang.NewKeyword("event")).(string),
		}
		if event.Type == "scalar" {
			event.Value = lang.Get(value, lang.NewKeyword("value")).(string)
			event.Tag = lang.Get(value, lang.NewKeyword("tag")).(string)
		}
		events = append(events, event)
	}
	return events
}

func invoke(input []byte, config any) any {
	var environment any
	if value, found := os.LookupEnv("YAMLSTAR_CSV_FORMAT"); found {
		environment = value
	}
	text := string(input)
	reader := csv.NewReader(strings.NewReader(text))
	return glj.Var("csv-parser.core", "parse").Invoke(
		reader, text, config, environment)
}

func withRuntime(input []byte, call func()) (err error) {
	if !utf8.Valid(input) {
		return fmt.Errorf("csv parser: input is not valid UTF-8")
	}
	runtimeLock.Lock()
	defer runtimeLock.Unlock()
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("csv parser: %v", value)
		}
	}()
	ysruntime.Load()
	glj.Var("clojure.core", "require").Invoke(lang.NewSymbol("csv-parser.core"))
	call()
	return nil
}
