M := .cache/makes
$(shell test -d $M || git clone -q https://github.com/makeplus/makes $M)

GLOAT-VERSION := 0.1.93
YAMLSCRIPT-VERSION := 0.3.6
GO-VERSION := 1.27.1
include $M/init.mk
include $M/gloat.mk
include $M/go.mk
include $M/gh.mk
include $M/yamlscript.mk
include $M/perl.mk
include $M/shellcheck.mk
include $M/clean.mk
include $M/shell.mk

MODULE := github.com/yamlstar/yamlstar-plugin-parser-csv
SOURCE := src/csv_parser/core.ys
CLJ := generated/clj/csv_parser/core.clj
LINK := .cache/gloat-srcs/csv_parser/core.clj
GENERATED := pkg/csv_parser/core
WORK := $(abspath .cache/go-generate)
CHECK-WORK := $(abspath .cache/go-check)
SO ?= $(if $(filter macos,$(OS-NAME)),dylib,so)
LIB := lib/libyamlstar-plugin-parser-csv.$(SO)
VERSION := 0.1.0
PLUGIN := yamlstar-plugin-parser-csv
RELEASE-ARCH := $(if $(IS-INTEL),x64,$(if $(IS-LINUX),aarch64,arm64))
RELEASE-PLATFORM ?= $(OS-NAME)-$(RELEASE-ARCH)
RELEASE-NAME := $(PLUGIN)-$(VERSION)-$(RELEASE-PLATFORM)
RELEASE-DIR := dist/$(RELEASE-NAME)
RELEASE-LIB := .cache/release/lib/lib$(PLUGIN).$(SO)
ARCHIVE := dist/$(RELEASE-NAME).tar.xz
SOURCE-DATE-EPOCH ?= $(shell git log -1 --format=%ct)
TAR ?= $(if $(IS-MACOS),gtar,tar)
RELEASE-CMD = PERL=$(PERL) GH=$(GH) GO=$(GO) util/release

export OLD_VERSION := $o
export NEW_VERSION := $(or $v,$(VERSION))
export YS_RELEASE_DRYRUN := $d
export YS_RELEASE_ALLOW_BRANCH := $a

build: $(LIB)

$(CLJ): $(SOURCE) $(YS) $(PERL)
	mkdir -p $(dir $@)
	$(YS) --compile $< | $(PERL) -0777 -pe \
	  'BEGIN { print "(ns csv-parser.core (:require ys.v0))\n"; \
	  print "(ys.v0/init)\n" }' > $@.tmp
	mv $@.tmp $@

$(LINK): $(CLJ)
	mkdir -p $(dir $@)
	ln -sf $(abspath $<) $@

generate: $(CLJ) $(LINK) $(GLOAT)
	rm -fr $(WORK)
	env -u GOROOT $(GLOAT) --force --module $(MODULE) \
	  -o $(WORK)/ $(LINK)
	test -f $(WORK)/pkg/csv_parser/core/loader.go
	rm -fr $(GENERATED)
	mkdir -p $(dir $(GENERATED))
	cp -R $(WORK)/pkg/csv_parser/core $(GENERATED)

generate-check: $(YS) $(PERL) $(GLOAT)
	mkdir -p .cache/check-srcs/csv_parser
	$(YS) --compile $(SOURCE) | $(PERL) -0777 -pe \
	  'BEGIN { print "(ns csv-parser.core (:require ys.v0))\n"; \
	  print "(ys.v0/init)\n" }' > .cache/check-srcs/csv_parser/core.clj
	diff $(CLJ) .cache/check-srcs/csv_parser/core.clj
	rm -fr $(CHECK-WORK)
	env -u GOROOT $(GLOAT) --force --module $(MODULE) \
	  -o $(CHECK-WORK)/ .cache/check-srcs/csv_parser/core.clj
	diff -r $(GENERATED) $(CHECK-WORK)/pkg/csv_parser/core

$(LIB): generate $(GO)
	mkdir -p $(dir $@)
	env -u GOROOT CGO_ENABLED=1 $(GO) build -buildmode=c-shared \
	  -o $@ ./shared

test: generate $(GO) $(SHELLCHECK)
	env -u GOROOT CGO_ENABLED=0 $(GO) test ./parser ./goyaml
	env -u GOROOT CGO_ENABLED=1 $(GO) test ./shared
	$(SHELLCHECK) util/demo util/release util/test-release util/test-archive

test-race: generate $(GO)
	env -u GOROOT CGO_ENABLED=1 $(GO) test -race ./parser ./goyaml

test-integration: build $(GO)
	env -u GOROOT $(GO) run ./util/integration

test-goyaml-cli: $(GO)
	env -u GOROOT $(GO) run ./util/integration --go-yaml-only

test-release: $(PERL) $(SHELLCHECK)
	$(SHELLCHECK) util/release util/test-release util/test-archive
	PERL=$(PERL) util/test-release

test-workflows: $(GO) $(SHELLCHECK)
	$(GO) run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 \
	  .github/workflows/test.yaml .github/workflows/release.yaml

release-check: $(PERL)
	$(RELEASE-CMD) check

release-list: $(PERL)
	$(RELEASE-CMD) list

release: $(PERL) $(if $(d),,$(GH))
	$(RELEASE-CMD) release

release-sanity-check release-version-bump release-commit release-tag: $(PERL)
	$(RELEASE-CMD) $(@:release-%=%)

release-build-github release-retry: $(PERL) $(if $(d),,$(GH) $(GO))
	$(RELEASE-CMD) $(@:release-%=%)

release-validate: release-check generate-check test test-race \
  test-release test-workflows

$(RELEASE-LIB): generate $(GO)
	mkdir -p $(dir $@)
	env -u GOROOT CGO_ENABLED=1 $(GO) build -trimpath \
	  -ldflags='-s -w' -buildmode=c-shared -o $@ ./shared

release-archive: release-check test $(RELEASE-LIB) $(SHELLCHECK)
	case $(RELEASE-PLATFORM) in \
	  linux-x64|linux-aarch64|macos-x64|macos-arm64) ;; \
	  *) echo 'Unsupported release platform: $(RELEASE-PLATFORM)' >&2; exit 1 ;; \
	esac
	test '$(RELEASE-PLATFORM)' = '$(OS-NAME)-$(RELEASE-ARCH)' || \
	  { echo 'Release platform must match this build host' >&2; exit 1; }
	rm -fr $(RELEASE-DIR) $(ARCHIVE)
	install -d $(RELEASE-DIR)/lib $(RELEASE-DIR)/include/yamlstar \
	  $(RELEASE-DIR)/share/yamlstar/plugins/parser-csv
	install -m 755 $(RELEASE-LIB) $(RELEASE-DIR)/lib/
	install -m 644 include/yamlstar_plugin.h $(RELEASE-DIR)/include/yamlstar/
	install -m 644 plugin.edn \
	  $(RELEASE-DIR)/share/yamlstar/plugins/parser-csv/
	install -m 644 License ReadMe.md $(RELEASE-DIR)/
	COPYFILE_DISABLE=1 $(TAR) --sort=name --mtime=@$(SOURCE-DATE-EPOCH) \
	  --owner=0 --group=0 --numeric-owner -C dist \
	  -cJf $(ARCHIVE) $(RELEASE-NAME)
	util/test-archive $(ARCHIVE) $(RELEASE-PLATFORM) $(VERSION)

demo:
	util/demo

clean::
	rm -fr lib dist .cache/release $(WORK) $(CHECK-WORK) \
	  .cache/gloat-srcs .cache/check-srcs
