+++
title = "native_call"
chapter = false
weight = 105
hidden = false
+++

## Summary
Run an exported function from a module previously loaded with `native_import`.

- Needs Admin: False
- Version: 1
- Author: @atomiczsec

### Arguments

#### filename

- Description: Name of the module previously selected with `native_import`.
- Required Value: True
- Default Value: None

#### function_name

- Description: Name of the exported function to execute.
- Required Value: True
- Default Value: None

#### args

- Description: Array of string arguments passed to the exported function.
- Required Value: False
- Default Value: None

## Usage

```
native_call
```

## Detailed Summary

`native_call` resolves the selected module to its Mythic file ID, finds the loaded module handle in the agent, resolves `function_name` with `dlsym`, and calls it as:

```
char* function_name(int argc, char** argv)
```

The returned string is copied into task output.

## Examples

Call a function with no user arguments:

```json
{
  "filename": "native_ps_test-arm64.dylib",
  "function_name": "self_info",
  "args": []
}
```

Call a function with string arguments:

```json
{
  "filename": "native_env_test-arm64.dylib",
  "function_name": "get_env",
  "args": ["HOME"]
}
```

List environment variables with a limit:

```json
{
  "filename": "native_env_test-arm64.dylib",
  "function_name": "list_env",
  "args": ["PATH", "5"]
}
```

## Operator Notes

- `function_name` is required and must exactly match the exported C symbol name without the leading underscore shown by macOS tools such as `nm`.
- `args` is optional; when present, all values are passed as strings.
- `argv[0]` is set to the agent process path. User arguments begin at `argv[1]`.
- The module must already be loaded with `native_import`.
- The function runs inside the Poseidon process. A crash in module code can crash the callback.

## Visibility

`native_call` does not create a child process by itself. The main detection surface is in-process execution plus whatever the called function does. A passive function that reads environment variables is lower-noise than a function that touches files, spawns commands, opens network sockets, enumerates credentials, or calls sensitive OS APIs.
