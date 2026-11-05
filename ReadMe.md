# YAMLScript CSV parser plugin

A CSV and TSV implementation of the existing `yaml-parser` plugin API.
The plugin works with go-yaml and native YAMLStar.
Its maintained source is [YAMLScript](src/csv_parser/core.ys), compiled to Go
with Gloat.
Go's `encoding/csv` reader handles CSV syntax.
YS selects the delimiter, validates options and headers, and builds YAML events.
The Go adapters create a fresh reader and bridge the host interfaces.

```csv
name,namespace
api,production
worker,staging
```

```yaml
- name: api
  namespace: production
- name: worker
  namespace: staging
```

## Configuration

```yaml
plugin:
  yaml-parser:
    name: csv
    format: auto
    header: true
```

`format` accepts `csv`, `tsv`, or `auto`.
An explicit setting overrides `YAMLSTAR_CSV_FORMAT`, which overrides the
default `auto` mode.
The variable configures the CSV plugin after it has been selected.
Unknown options, invalid formats, and non-boolean header settings are errors.

Auto-detection examines the first nonempty logical record outside quoted fields.
Commas select CSV and tabs select TSV.
Neither selects CSV, suitable for single-column input.
Both require an explicit format.
Detection uses content, so stdin works just like a file.

`header: true` uses the first record as mapping keys.
Empty and duplicate headers are errors.
`header: false` returns a sequence of row sequences, including the first row.
Empty input and header-only input produce `[]`.
All records must have the same width in either mode.

Cells and header names remain strings, including empty strings, `001`, `true`,
and `null`.
Row and column order is preserved.
Quoted delimiters, escaped double quotes, multiline fields, and CRLF are
supported by Go's strict CSV reader.
Empty physical lines are skipped; whitespace is otherwise preserved.
TSV uses the same quoting rules with tabs as delimiters.

## Go use

```go
import (
    csv "github.com/yamlstar/yamlstar-plugin-parser-csv/goyaml"
    "go.yaml.in/yaml/v4"
)

plugin, err := csv.New(map[string]any{"format": "auto", "header": true})
if err != nil {
    return err
}
var value any
err = yaml.Load(input, &value, yaml.WithPlugin(plugin))
```

`New(nil)` selects defaults.
Configuration is copied, and the implementation supports concurrent calls.
Call `csv.Register()` once at startup to enable named selection through
`yaml.OptsYAML`.
Importing the package does not register it or change native defaults.

The go-yaml adapter is an ordinary Go package.
It needs no shared library, cgo, JVM, or runtime interpreter download.
The package exports `Register() error` for go-yaml's existing external-package
CLI build mechanism once a version is published.

## Local development and demo

This checkout is developed at `repos/yamlstar-plugin-parser-csv` within the
YAMLStar workspace, alongside `repos/go-yaml`.
Commands use Makes-managed dependencies under `.cache/`.

```bash
# From the YAMLStar workspace:
make cli

# From this plugin checkout:
make generate
make test test-race generate-check
make test-integration
make demo
```

`make test-integration` builds the shared library, stages the local go-yaml
source, and builds its CLI through `PLUGIN-PACKAGE` using a temporary local
Go module proxy.
It tests the same fixtures through both native CLIs.
The source go-yaml checkout and its executable are left untouched.
The resulting go-yaml executable is `.cache/integration/source/go-yaml`.
The integration helper defaults `YAMLSTAR_CLI` to `../../cli/bin/yaml`.

After successful integration tests, the helper records the tested YAMLStar
executable in `.cache/integration/yamlstar-path`.
`make demo` uses that executable, independently of the checkout layout.
If no tested executable has been recorded, it uses `yaml` from `PATH`.
Set `YAMLSTAR_CLI` to override either selection.
It uses the staged go-yaml executable above unless `GO_YAML_CLI` is set.
Both overrides accept an executable path or a command name on `PATH`.
Both executables need CSV plugin support.
The demo checks YAMLStar's CSV support before printing the demonstrations.
An older installed YAMLStar may lack the CSV registration even when its
version number matches a development build.

```bash
YAMLSTAR_CLI=/path/to/yaml GO_YAML_CLI=/path/to/go-yaml make demo
```

To use the shared library directly from the workspace:

```bash
YAMLSTAR_LIBRARY_PATH=$PWD/repos/yamlstar-plugin-parser-csv/lib \
  cli/bin/yaml --plugin=yaml-parser=csv --no-install \
  -y repos/yamlstar-plugin-parser-csv/example/services.csv
```

The YAMLStar shared library implements ABI v2 and accepts an EDN options map.
It returns the same events as the go-yaml adapter.
The host needs CSV in its shared-parser registration list.
JVM integration and automatic plugin installation are outside this version.

YAMLStar's current JSON writer does not escape every control character.
The plugin preserves those values in YAML output and parser events.

## Releases

Release `0.1.0` is prepared in the source metadata.
Commit the release tooling before running a real release.
Use a shell with Git publication and GitHub Actions write access.

```bash
make release d=1                  # Offline preview of the current version
make release                     # Release the already-versioned source
make release o=0.1.0 v=0.1.1 d=1 # Preview a version bump
make release o=0.1.0 v=0.1.1     # Bump, validate, commit, tag, and release
```

Real releases require a clean checkout on `main`.
Set `a=1` to release from another branch.
The helper keeps `Makefile`, `Meta`, `plugin.edn`, and `parser/parser.go`
in agreement and commits only those version files when a bump is needed.
It runs the release checks before creating annotated `VERSION` and `vVERSION`
tags at the same commit.
Existing tags are never moved.
Only the current branch and those two tags are published atomically.

The GitHub workflow builds and ABI-tests native shared libraries for Linux
x64 and ARM64, and macOS x64 and ARM64.
Each `yamlstar-plugin-parser-csv-VERSION-PLATFORM.tar.xz` contains the library,
ABI header, manifest, license, and ReadMe.
The GitHub release uses the unprefixed tag and includes `SHA256SUMS`.
The `v` tag publishes the Go module, including its go-yaml adapter.
There are no Python wheels or PyPI releases.
The helper waits for the exact workflow run, checks all release assets, and
downloads the exact Go module version before reporting success.

```bash
make release-archive             # Build and ABI-test this host's archive
make release-validate            # Run local release checks
make test-goyaml-cli             # Check the generic external-package build
make release-retry v=0.1.0       # Rerun failed jobs for the original tags
make release-list                # Show release commands
```

Retries preserve both tags and the original release commit.
If a release stops after committing the version bump but before starting a
workflow, resume with `make release v=VERSION`, omitting `o`.
For source changes after tagging, release a new version.
The workflow can resume an incomplete draft release and does not overwrite
assets in a published release.
Set `GO_YAML_DIR` to a go-yaml checkout for standalone CLI integration tests.
The release workflow uses its own pinned checkout under `repos/go-yaml`.

## License

MIT License.
