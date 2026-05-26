+++
title = "native_module"
chapter = false
weight = 102
hidden = false
+++

## Summary

`native_import`, `native_call`, and `native_unload` provide BOF-style native module execution for Poseidon. Operators can upload a macOS `.dylib` or Linux `.so`, load it once into the agent process, call exported C functions repeatedly, then unload it when finished.

- Author: @atomiczsec

The module ABI is:

```c
char* function_name(int argc, char** argv)
```

Returned strings are copied into Mythic task output.

## Workflow

1. Build a module for the callback's OS and CPU architecture.
2. Upload the `.dylib` or `.so` to Mythic.
3. Run `native_import` and select the uploaded file.
4. Run `native_call` with the module filename, exported function name, and string arguments.
5. Run `native_unload` when the module is no longer needed.

Example `native_call` parameters:

```json
{
  "filename": "native_env_test-arm64.dylib",
  "function_name": "get_env",
  "args": ["HOME"]
}
```

## How This Differs From Other Poseidon Execution Features

- `shell` and `run` create child processes. Native modules execute inside the existing Poseidon process.
- `execute_library` loads a library from a target-side file path for that task. Native modules are imported once and kept in an in-agent module table for later calls.
- `jsimport` / `jsimport_call` use JavaScript for Automation on macOS. Native modules use the platform dynamic loader and exported C symbols.
- Built-in Poseidon commands are compiled into the payload. Native modules let operators add compiled task logic after the payload is already running.

This gives better modularity and avoids rebuilding the payload for every small native capability, but it also concentrates module execution inside the agent process.

## Module Requirements

- Match target OS and architecture.
- Export plain C symbols; C++ functions should use `extern "C"`.
- Return a stable `char*`; a static buffer or allocated string is safest.
- Treat `argv[0]` as the agent process path. User-supplied arguments start at `argv[1]`.
- Keep output bounded. Large strings can be noisy and may stress task output handling.

## Operational Security And Telemetry

Native module execution is not invisible. It changes the agent process and can be visible to EDR, telemetry collectors, and memory inspection.

macOS:

- `native_import` writes the uploaded dylib to a temporary path and loads it with `dlopen`.
- Endpoint tools can observe temporary file creation, file read, image load, and memory mappings in the Poseidon process.
- Unified Logging, Endpoint Security clients, and EDR sensors may record process image loads, file events, code-signing metadata, quarantine state, and suspicious unsigned dylib loads.
- `native_call` does not create a child process by itself, but whatever the module function does can create its own telemetry.
- `native_unload` calls `dlclose` and removes the temporary file, but prior file, image-load, and memory evidence may remain in logs or EDR state.

Linux:

- The loader attempts anonymous `memfd` loading first and falls back to a temporary `.so` path if needed.
- Defenders may see `memfd_create`, writes to anonymous file descriptors, `dlopen` of `/proc/self/fd/<n>`, executable memory mappings, or temporary `.so` file activity.
- `/proc/<pid>/maps`, auditd, eBPF-based sensors, Sysmon for Linux, and EDR agents can expose loaded mappings and file-backed or memfd-backed executable regions.
- As with macOS, `native_call` itself stays in-process, but module behavior can create process, file, network, credential, or kernel telemetry.

Detection framing:

- This maps most closely to ATT&CK `T1620` Reflective Code Loading and may overlap `T1106` Native API depending on module behavior.
- The lower-noise part is avoiding a new child process for every task.
- The higher-risk part is dynamic loading of unsigned or unusual native code into an already-running agent process.
- Test modules such as environment listing or self/process inspection are useful for validating the ABI, but production modules should be reviewed for the APIs they touch and the telemetry they create.
