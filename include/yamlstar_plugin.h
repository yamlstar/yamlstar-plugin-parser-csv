#ifndef YAMLSTAR_PLUGIN_H
#define YAMLSTAR_PLUGIN_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

uint64_t yamlstar_plugin_v2_abi(void);

int32_t yamlstar_plugin_v2_manifest(
    uint8_t **output,
    size_t *output_length);

int32_t yamlstar_plugin_v2_transform(
    const uint8_t *input,
    size_t input_length,
    const uint8_t *options_edn,
    size_t options_length,
    uint8_t **output,
    size_t *output_length);

void yamlstar_plugin_v2_free(uint8_t *output);

#ifdef __cplusplus
}
#endif

#endif
