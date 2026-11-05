// Copyright 2026 YAMLStar Authors
// MIT License

// Package goyaml adapts the compiled YAMLScript parser to go-yaml.
package goyaml

import (
	"github.com/yamlstar/yamlstar-plugin-parser-csv/parser"
	"go.yaml.in/yaml/v4"
)

const Version = parser.Version

// Plugin parses CSV or TSV into a complete YAML event stream.
type Plugin struct {
	config map[string]any
}

var _ yaml.ParserPlugin = (*Plugin)(nil)

// New validates and snapshots configuration. Nil selects the defaults.
func New(config map[string]any) (*Plugin, error) {
	if _, err := parser.Parse(nil, config); err != nil {
		return nil, err
	}
	copy := make(map[string]any, len(config))
	for key, value := range config {
		copy[key] = value
	}
	return &Plugin{config: copy}, nil
}

// Register makes yaml-parser=csv available without changing native defaults.
// Call once during application startup; duplicate registration is an error.
func Register() error {
	return yaml.RegisterPlugin(yaml.PluginRegistration{
		API: "yaml-parser", Name: "csv", Version: Version,
		Factory: func(config map[string]any) (any, error) { return New(config) },
	})
}

func (p *Plugin) Parse(input []byte) ([]yaml.PluginEvent, error) {
	source, err := parser.Parse(input, p.config)
	if err != nil {
		return nil, err
	}
	events := make([]yaml.PluginEvent, len(source))
	for i, event := range source {
		events[i] = yaml.PluginEvent{
			Type: event.Type, Value: event.Value, Tag: event.Tag, Flow: event.Flow,
		}
	}
	return events, nil
}
