+++
title = "native_import"
chapter = false
weight = 104
hidden = false
+++

## Summary
Load a Linux `.so` or macOS `.dylib` into the Poseidon process for later use with `native_call`.

- Needs Admin: False
- Version: 1
- Author: @atomiczsec

### Arguments

#### file_id

- Description: File UUID of the `.so` or `.dylib` to load.
- Required Value: True
- Default Value: None

## Usage

```
native_import
```

## Detailed Summary

`native_import` downloads the selected file from Mythic and loads it into the agent process with `dlopen`. Linux attempts to load from an anonymous `memfd` first and falls back to a temporary `.so` path if needed. macOS loads from a temporary `.dylib` path. Use `native_unload` to close the handle and remove temporary files.

Modules must match the target OS and architecture. Exported functions called by `native_call` must use this C ABI:

```
char* function_name(int argc, char** argv)
```

When building manually, include `native_module` or the selected command tags (`native_import,native_call,native_unload`) with the normal C2 tags.

## Operator Notes

- Importing only loads the module; it does not run an exported function.
- The uploaded filename is later selected in `native_call` and `native_unload`.
- Importing the same file ID twice in one callback returns an already-loaded error.
- The module is kept in the agent process until `native_unload` or process exit.
- For macOS, upload a `.dylib` matching the callback architecture (`arm64` for Apple Silicon, `x86_64` for Intel, or a universal dylib).
- For Linux, upload an ELF `.so` matching the callback architecture.

## Visibility

`native_import` is the highest-signal step for defenders because it loads new native code into the agent process. On macOS this includes temporary dylib file creation and a dynamic loader event. On Linux this can include `memfd_create`, `/proc/self/fd` loading, executable memory mappings, or temporary `.so` fallback activity.
