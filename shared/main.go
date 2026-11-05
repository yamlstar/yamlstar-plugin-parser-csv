//go:build cgo

// Copyright 2026 YAMLStar Authors
// MIT License

package main

/*
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/yamlstar/yamlstar-plugin-parser-csv/parser"
)

const manifest = `{:abi 2 :api "yaml-parser" :kind "event-source" ` +
	`:name "csv" :version "` + parser.Version + `" ` +
	`:event-format "yamlstar-events-edn-v1"}`

//export yamlstar_plugin_v2_abi
func yamlstar_plugin_v2_abi() C.uint64_t {
	return 2
}

//export yamlstar_plugin_v2_manifest
func yamlstar_plugin_v2_manifest(
	output **C.uint8_t, length *C.size_t,
) C.int32_t {
	return writeOutput([]byte(manifest), output, length, 0)
}

//export yamlstar_plugin_v2_transform
func yamlstar_plugin_v2_transform(
	input *C.uint8_t,
	inputLength C.size_t,
	options *C.uint8_t,
	optionsLength C.size_t,
	output **C.uint8_t,
	outputLength *C.size_t,
) C.int32_t {
	inputBytes, ok := copyInput(input, inputLength)
	if !ok {
		return writeError(
			fmt.Errorf("CSV input exceeds platform limits"),
			output, outputLength, 2)
	}
	optionBytes, ok := copyInput(options, optionsLength)
	if !ok {
		return writeError(
			fmt.Errorf("CSV parser options exceed platform limits"),
			output, outputLength, 2)
	}
	events, err := parseEvents(inputBytes, optionBytes)
	if err != nil {
		return writeError(err, output, outputLength, 1)
	}
	return writeOutput(events, output, outputLength, 0)
}

//export yamlstar_plugin_v2_free
func yamlstar_plugin_v2_free(output *C.uint8_t) {
	C.free(unsafe.Pointer(output))
}

func copyInput(pointer *C.uint8_t, length C.size_t) ([]byte, bool) {
	if uint64(length) > uint64(^uint(0)>>1) {
		return nil, false
	}
	if length == 0 {
		return nil, true
	}
	if pointer == nil {
		return nil, false
	}
	return append([]byte(nil),
		unsafe.Slice((*byte)(unsafe.Pointer(pointer)), int(length))...), true
}

func parseEvents(input, options []byte) ([]byte, error) {
	return parser.ParseEDN(input, options)
}

func writeError(
	err error,
	output **C.uint8_t,
	length *C.size_t,
	status C.int32_t,
) C.int32_t {
	return writeOutput([]byte(err.Error()), output, length, status)
}

func writeOutput(
	data []byte,
	output **C.uint8_t,
	length *C.size_t,
	status C.int32_t,
) C.int32_t {
	*output = nil
	*length = C.size_t(len(data))
	if len(data) == 0 {
		return status
	}
	pointer := C.CBytes(data)
	if pointer == nil {
		*length = 0
		return 2
	}
	*output = (*C.uint8_t)(pointer)
	return status
}

func main() {}
