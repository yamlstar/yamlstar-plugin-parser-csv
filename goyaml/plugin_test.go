package goyaml_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/yamlstar/yamlstar-plugin-parser-csv/goyaml"
	"go.yaml.in/yaml/v4"
)

type fixture struct {
	name, input, want, error string
	config                   map[string]any
	environment              string
	yamlOutput               bool
}

var fixtures = []fixture{
	{name: "csv", input: "name,namespace\napi,production\nworker,staging\n",
		want: `[{"name":"api","namespace":"production"},{"name":"worker","namespace":"staging"}]`},
	{name: "tsv", input: "name\tnamespace\napi\tproduction\n",
		want: `[{"name":"api","namespace":"production"}]`},
	{name: "raw rows", input: "name,port\napi,80\n",
		config: map[string]any{"header": false}, want: `[["name","port"],["api","80"]]`},
	{name: "strings", input: "id,enabled,nothing,empty\n001,true,null,\n",
		want: `[{"id":"001","enabled":"true","nothing":"null","empty":""}]`},
	{name: "quoted comma in tsv", input: "name\tnote\napi\t\"hello, world\"\n",
		want: `[{"name":"api","note":"hello, world"}]`},
	{name: "quoted tab in csv header", input: "name,\"some\tnote\"\napi,text\n",
		want: `[{"name":"api","some\tnote":"text"}]`},
	{name: "quoted comma in tsv header", input: "name\t\"some,note\"\napi\ttext\n",
		want: `[{"name":"api","some,note":"text"}]`},
	{name: "quotes and newlines", input: "name,note\napi,\"hello\n\"\"world\"\"\"\n",
		want: `[{"name":"api","note":"hello\n\"world\""}]`},
	{name: "multiline header", input: "\"some\nname\"\tvalue\napi\ttext\n",
		want: `[{"some\nname":"api","value":"text"}]`},
	{name: "crlf and blank lines", input: "\r\nname,port\r\n\r\napi,80\r\n",
		want: `[{"name":"api","port":"80"}]`},
	{name: "unicode", input: "名前,city\n雪,Montréal\n",
		want: `[{"名前":"雪","city":"Montréal"}]`},
	// YAMLStar's current JSON writer does not escape every control character.
	// Check construction through YAML output, which preserves these values.
	{name: "control and supplementary characters in YAML", yamlOutput: true,
		input: "name,note\napi,\"\x00\x01\b\f\v\t😀\"\n",
		want:  `[{"name":"api","note":"\u0000\u0001\b\f\u000b\t😀"}]`},
	{name: "single column", input: "name\napi\n", want: `[{"name":"api"}]`},
	{name: "empty", want: `[]`},
	{name: "only blank lines", input: "\n\r\n", want: `[]`},
	{name: "header only", input: "name,port\n", want: `[]`},
	{name: "empty raw cells", input: ",\n,\n", config: map[string]any{"header": false},
		want: `[["",""],["",""]]`},
	{name: "env tsv", input: "name\tnote\napi\ttext\n", environment: "tsv",
		want: `[{"name":"api","note":"text"}]`},
	{name: "env csv", input: "name\tnote\napi\ttext\n", environment: "csv",
		want: `[{"name\tnote":"api\ttext"}]`},
	{name: "env auto", input: "name\tnote\napi\ttext\n", environment: "auto",
		want: `[{"name":"api","note":"text"}]`},
	{name: "config overrides env", input: "name\tnote\napi\ttext\n",
		environment: "invalid", config: map[string]any{"format": "tsv"},
		want: `[{"name":"api","note":"text"}]`},
	{name: "explicit auto overrides env", input: "name\tnote\napi\ttext\n",
		environment: "csv", config: map[string]any{"format": "auto"},
		want: `[{"name":"api","note":"text"}]`},
	{name: "ambiguous", input: "name,note\textra\n", error: "ambiguous delimiter"},
	{name: "explicit csv resolves ambiguity", input: "name,note\textra\napi,text\n",
		config: map[string]any{"format": "csv"}, want: `[{"name":"api","note\textra":"text"}]`},
	{name: "explicit tsv resolves ambiguity", input: "name,note\textra\napi\ttext\n",
		config: map[string]any{"format": "tsv"}, want: `[{"name,note":"api","extra":"text"}]`},
	{name: "duplicate headers", input: "name,name\napi,text\n", error: "must be unique"},
	{name: "empty header", input: "name,\napi,text\n", error: "must not be empty"},
	{name: "quoted empty header", input: "\"\"\napi\n", error: "must not be empty"},
	{name: "short row", input: "name,port\napi\n", error: "wrong number of fields"},
	{name: "long row", input: "name\napi,80\n", error: "wrong number of fields"},
	{name: "raw width", input: "name,port\napi\n", config: map[string]any{"header": false},
		error: "wrong number of fields"},
	{name: "unclosed quote", input: "name,note\napi,\"text\n", error: "quoted-field"},
	{name: "bare quote", input: "name,note\napi,te\"xt\n", error: "bare"},
	{name: "invalid format", config: map[string]any{"format": "ssv"}, error: "format must"},
	{name: "null format", config: map[string]any{"format": nil}, error: "format must"},
	{name: "invalid header", config: map[string]any{"header": "false"}, error: "header must"},
	{name: "null header", config: map[string]any{"header": nil}, error: "header must"},
	{name: "unknown option", config: map[string]any{"headers": true}, error: "unknown configuration"},
	{name: "invalid env", environment: "invalid", error: "format must"},
}

func setEnvironment(t *testing.T, value string) {
	t.Helper()
	if value != "" {
		t.Setenv("YAMLSTAR_CSV_FORMAT", value)
		return
	}
	old, present := os.LookupEnv("YAMLSTAR_CSV_FORMAT")
	os.Unsetenv("YAMLSTAR_CSV_FORMAT")
	t.Cleanup(func() {
		if present {
			os.Setenv("YAMLSTAR_CSV_FORMAT", old)
		} else {
			os.Unsetenv("YAMLSTAR_CSV_FORMAT")
		}
	})
}

func equalJSON(t *testing.T, got []byte, want string) {
	t.Helper()
	var actual, expected any
	if err := json.Unmarshal(got, &actual); err != nil {
		t.Fatalf("invalid JSON %q: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestFixtures(t *testing.T) {
	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			setEnvironment(t, tc.environment)
			plugin, err := goyaml.New(tc.config)
			var value any
			if err == nil {
				err = yaml.Load([]byte(tc.input), &value, yaml.WithPlugin(plugin))
			}
			if tc.error != "" {
				if err == nil || !strings.Contains(err.Error(), tc.error) {
					t.Fatalf("got %v, want error containing %q", err, tc.error)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			equalJSON(t, encoded, tc.want)
		})
	}
}

func TestSnapshotAndConcurrency(t *testing.T) {
	setEnvironment(t, "auto")
	config := map[string]any{"header": true}
	plugin, err := goyaml.New(config)
	if err != nil {
		t.Fatal(err)
	}
	config["header"] = false
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var value []map[string]string
			if err := yaml.Load([]byte("name,port\napi,80\n"), &value,
				yaml.WithPlugin(plugin)); err != nil || len(value) != 1 ||
				value[0]["name"] != "api" {
				t.Errorf("concurrent load: %#v, %v", value, err)
			}
		}()
	}
	wg.Wait()
}

func TestRegistration(t *testing.T) {
	if os.Getenv("CSV_REGISTER_TEST") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestRegistration$")
		cmd.Env = append(os.Environ(), "CSV_REGISTER_TEST=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("registration: %v\n%s", err, output)
		}
		return
	}
	setEnvironment(t, "auto")
	if _, err := yaml.OptsYAML("plugin: {yaml-parser: csv}"); err == nil {
		t.Fatal("import registered the plugin implicitly")
	}
	if err := goyaml.Register(); err != nil {
		t.Fatal(err)
	}
	for _, selection := range []string{"csv", "csv@v" + goyaml.Version,
		"{name: csv, format: tsv, header: false}"} {
		if _, err := yaml.OptsYAML("plugin: {yaml-parser: " + selection + "}"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := yaml.OptsYAML("plugin: {yaml-parser: csv@v9.8.7}"); err == nil {
		t.Fatal("accepted version mismatch")
	}
	var native map[string]any
	if err := yaml.Load([]byte("native: true"), &native); err != nil || native["native"] != true {
		t.Fatalf("native defaults: %#v, %v", native, err)
	}
	if err := goyaml.Register(); err == nil {
		t.Fatal("accepted duplicate registration")
	}
}

func TestNativeHosts(t *testing.T) {
	for _, host := range []string{"GO_YAML_CLI", "YAMLSTAR_CLI"} {
		binary := os.Getenv(host)
		if binary == "" {
			t.Logf("%s not set; skipping this host", host)
			continue
		}
		for _, tc := range fixtures {
			t.Run(host+"/"+tc.name, func(t *testing.T) {
				setEnvironment(t, tc.environment)
				config, err := yaml.Dump(map[string]any{
					"plugin": map[string]any{"yaml-parser": pluginConfig(tc.config)},
				})
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(t.TempDir(), "options.yaml")
				if err := os.WriteFile(path, config, 0600); err != nil {
					t.Fatal(err)
				}
				args := []string{"-j", "--config=" + path}
				if tc.yamlOutput {
					args[0] = "-y"
				}
				if host == "YAMLSTAR_CLI" {
					args = append(args, "--no-install")
				}
				cmd := exec.Command(binary, args...)
				cmd.Stdin = strings.NewReader(tc.input)
				output, err := cmd.CombinedOutput()
				if tc.error != "" {
					if err == nil || !strings.Contains(string(output), tc.error) {
						t.Fatalf("got %v: %s; want %q", err, output, tc.error)
					}
					return
				}
				if err != nil {
					t.Fatalf("%v: %s", err, output)
				}
				if tc.yamlOutput {
					var value any
					if err := yaml.Load(output, &value); err != nil {
						t.Fatalf("invalid YAML %q: %v", output, err)
					}
					output, err = json.Marshal(value)
					if err != nil {
						t.Fatal(err)
					}
				}
				equalJSON(t, output, tc.want)
			})
		}
	}
}

func pluginConfig(config map[string]any) map[string]any {
	copy := map[string]any{"name": "csv"}
	for key, value := range config {
		copy[key] = value
	}
	return copy
}
