// Build go-yaml through its external-package path without changing its checkout.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const module = "github.com/yamlstar/yamlstar-plugin-parser-csv"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	hostRoot := filepath.Dir(filepath.Dir(root))
	source := filepath.Join(hostRoot, "repos/go-yaml")
	if _, err := os.Stat(filepath.Join(source, "util/build-cli/main.go")); err != nil {
		return fmt.Errorf("required go-yaml checkout: %w", err)
	}
	work := filepath.Join(root, ".cache/integration")
	stage := filepath.Join(work, "source")
	if err := os.RemoveAll(stage); err != nil {
		return err
	}
	if err := os.MkdirAll(stage, 0755); err != nil {
		return err
	}
	files, err := exec.Command("git", "-C", source, "ls-files", "-z").Output()
	if err != nil {
		return err
	}
	for _, file := range strings.Split(string(files), "\x00") {
		if file == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(source, file))
		if err != nil {
			return err
		}
		path := filepath.Join(stage, file)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return err
		}
	}

	var names []string
	for _, name := range []string{"go.mod", "go.sum", "License", "parser", "pkg", "goyaml"} {
		err := filepath.WalkDir(name, func(path string, entry fs.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				names = append(names, path)
			}
			return err
		})
		if err != nil {
			return err
		}
	}
	hash := sha256.New()
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		hash.Write([]byte(name))
		hash.Write(data)
	}
	version := "v0.0.0-local-" + hex.EncodeToString(hash.Sum(nil))[:16]
	proxy := filepath.Join(work, "proxy")
	versions := filepath.Join(proxy, filepath.FromSlash(module), "@v")
	if err := os.MkdirAll(versions, 0755); err != nil {
		return err
	}
	archive, err := os.Create(filepath.Join(versions, version+".zip"))
	if err != nil {
		return err
	}
	writer := zip.NewWriter(archive)
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		file, err := writer.Create(module + "@" + version + "/" + filepath.ToSlash(name))
		if err != nil {
			return err
		}
		if _, err := file.Write(data); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := archive.Close(); err != nil {
		return err
	}
	mod, err := os.ReadFile("go.mod")
	if err != nil {
		return err
	}
	for suffix, data := range map[string][]byte{
		".mod": mod,
		".info": []byte(fmt.Sprintf(
			`{"Version":%q,"Time":"2000-01-01T00:00:00Z"}`, version)),
	} {
		if err := os.WriteFile(filepath.Join(versions, version+suffix), data, 0644); err != nil {
			return err
		}
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return err
	}
	build := exec.Command(goBin, "run", "./util/build-cli")
	build.Dir = stage
	build.Env = append(os.Environ(),
		"GOPROXY=file://"+filepath.ToSlash(proxy)+",https://proxy.golang.org",
		"GONOSUMDB="+module,
		"GO_YAML_BUILD_GO="+goBin,
		"GO_YAML_BUILD_CONFIG=", "GO_YAML_BUILD_PLUGIN=",
		"GO_YAML_BUILD_PLUGIN_PACKAGE="+module+"/goyaml@"+version)
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		return err
	}
	yamlstar := os.Getenv("YAMLSTAR_CLI")
	if yamlstar == "" {
		yamlstar = filepath.Join(hostRoot, "cli/bin/yaml")
	}
	if _, err := os.Stat(yamlstar); err != nil {
		return fmt.Errorf("build YAMLStar with make cli first: %w", err)
	}
	tests := exec.Command(goBin, "test", "./goyaml", "-run", "^TestNativeHosts$", "-count=1", "-v")
	tests.Env = append(os.Environ(),
		"GO_YAML_CLI="+filepath.Join(stage, "go-yaml"),
		"YAMLSTAR_CLI="+yamlstar,
		"YAMLSTAR_LIBRARY_PATH="+filepath.Join(root, "lib"))
	tests.Stdout, tests.Stderr = os.Stdout, os.Stderr
	return tests.Run()
}
