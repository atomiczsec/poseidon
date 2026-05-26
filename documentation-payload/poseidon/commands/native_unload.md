+++
title = "native_unload"
chapter = false
weight = 106
hidden = false
+++

## Summary
Unload a module previously loaded with `native_import`.

- Needs Admin: False
- Version: 1
- Author: @atomiczsec

### Arguments

#### file_id

- Description: The native module previously imported with `native_import`, shown as `filename (file_id)`.
- Required Value: True
- Default Value: None

## Usage

```
native_unload
```

## Detailed Summary

`native_unload` resolves the selected module to its Mythic file ID, calls `dlclose` on the stored handle, removes any temporary file path used for loading, and deletes the in-agent module record.

## Operator Notes

- Use `native_unload` when a module is no longer needed.
- Unloading removes Poseidon's handle and any temporary path Poseidon created for the loaded module.
- Unloading does not remove historical task output, Mythic file records, EDR observations, or prior OS telemetry.
- If module code starts threads or leaves process-global state behind, unloading the handle may not fully undo that behavior.
