#include "yamlstar_plugin.h"

#include <dlfcn.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifndef PLUGIN_EXTENSION
#define PLUGIN_EXTENSION "so"
#endif

#ifndef PLUGIN_VERSION
#error PLUGIN_VERSION must be supplied by the archive test
#endif

typedef uint64_t (*abi_fn)(void);
typedef int32_t (*manifest_fn)(uint8_t **, size_t *);
typedef int32_t (*transform_fn)(const uint8_t *, size_t,
                               const uint8_t *, size_t,
                               uint8_t **, size_t *);
typedef void (*free_fn)(uint8_t *);

static void fail(const char *message) {
    fprintf(stderr, "%s\n", message);
    exit(1);
}

static char *take_output(free_fn free_output, uint8_t *output,
                         size_t length) {
    char *text = malloc(length + 1);
    if (text == NULL) {
        fail("malloc failed");
    }
    memcpy(text, output, length);
    text[length] = '\0';
    free_output(output);
    return text;
}

int main(void) {
    const char *directory = getenv("YAMLSTAR_LIBRARY_PATH");
    if (directory == NULL) {
        fail("YAMLSTAR_LIBRARY_PATH is not set");
    }

    char path[4096];
    snprintf(path, sizeof(path),
             "%s/libyamlstar-plugin-parser-csv.%s",
             directory, PLUGIN_EXTENSION);
    void *handle = dlopen(path, RTLD_NOW | RTLD_LOCAL);
    if (handle == NULL) {
        fail(dlerror());
    }

    abi_fn abi = (abi_fn)dlsym(handle, "yamlstar_plugin_v2_abi");
    manifest_fn manifest = (manifest_fn)dlsym(
        handle, "yamlstar_plugin_v2_manifest");
    transform_fn transform = (transform_fn)dlsym(
        handle, "yamlstar_plugin_v2_transform");
    free_fn free_output = (free_fn)dlsym(
        handle, "yamlstar_plugin_v2_free");
    if (abi == NULL || manifest == NULL || transform == NULL ||
        free_output == NULL) {
        fail("plugin ABI symbol is missing");
    }
    if (abi() != 2) {
        fail("plugin ABI version is not 2");
    }

    uint8_t *output = NULL;
    size_t length = 0;
    if (manifest(&output, &length) != 0) {
        fail("manifest call failed");
    }
    char *text = take_output(free_output, output, length);
    if (strstr(text, ":api \"yaml-parser\"") == NULL ||
        strstr(text, ":name \"csv\"") == NULL ||
        strstr(text, ":kind \"event-source\"") == NULL ||
        strstr(text, ":version \"" PLUGIN_VERSION "\"") == NULL) {
        fail("plugin manifest is incorrect");
    }
    free(text);

    static const char input[] = "name,port\napi,80\n";
    static const char options[] = "{}";
    output = NULL;
    length = 0;
    int32_t status = transform(
        (const uint8_t *)input, strlen(input),
        (const uint8_t *)options, strlen(options), &output, &length);
    text = take_output(free_output, output, length);
    if (status != 0 || strstr(text, ":event \"mapping_start\"") == NULL ||
        strstr(text, ":value \"80\"") == NULL ||
        strstr(text, ":tag \"tag:yaml.org,2002:str\"") == NULL) {
        fail("plugin event response is incorrect");
    }
    free(text);

    static const char bad_options[] = "{:unknown true}";
    output = NULL;
    length = 0;
    status = transform(
        (const uint8_t *)input, strlen(input),
        (const uint8_t *)bad_options, strlen(bad_options),
        &output, &length);
    text = take_output(free_output, output, length);
    if (status != 1 || strstr(text, "unknown configuration keys") == NULL) {
        fail("plugin configuration error is incorrect");
    }
    free(text);

    dlclose(handle);
    puts("shared parser plugin ABI test passed");
    return 0;
}
