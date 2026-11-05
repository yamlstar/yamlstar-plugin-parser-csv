M := .cache/makes
$(shell test -d $M || git clone -q https://github.com/makeplus/makes $M)

GLOAT-VERSION := 0.1.93
YAMLSCRIPT-VERSION := 0.3.6
GO-VERSION := 1.27.1
include $M/init.mk
include $M/gloat.mk
include $M/go.mk
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
	$(SHELLCHECK) util/demo

test-race: generate $(GO)
	env -u GOROOT CGO_ENABLED=1 $(GO) test -race ./parser ./goyaml

test-integration: build $(GO)
	env -u GOROOT $(GO) run ./util/integration

demo:
	util/demo

clean::
	rm -fr lib $(WORK) $(CHECK-WORK) .cache/gloat-srcs .cache/check-srcs
