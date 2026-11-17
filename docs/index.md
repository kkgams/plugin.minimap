---
title: plugin.minimap
description: Generate tile maps from trees.
status: reference
---

# plugin.minimap

A [GAMS](https://github.com/kkgams/gams) WASM plugin for generating tile maps from trees.

| Unit | Source / artifact | Original work |
| --- | --- | --- |
| Tree-to-tilemap generator | `plugins/minimap.comp/main.go` / `plugin.minimap.wasm` | GAMS |

## API

`plugins/minimap.comp/wit/package.wit` defines `gams:minimap/minimap@1.0.0`: `gen(tree)` returning a tile map or error. Nodes carry string-pair properties and parent IDs; maps contain layers with widths, flat tile arrays and properties. The `generator` world uses Host-provided WASI imports.

## Development

Use `nix develop` (or direnv) for pinned Go 1.25, TinyGo, wasm-tools and documentation tools. No sibling checkout is required.

| Command | Purpose |
| --- | --- |
| `make build` | Compile and validate the component with embedded notices. |
| `make package` | Prepare `build.nosync/plugin.minimap.wasm`, `LICENSES.zip` and `SHA256SUMS` incrementally. |
| `make test` | Run the native Go package checks and existing SDK tilemap tests. |
| `make check` | Check Python tooling syntax; native tests are separate from WASM runtime coverage. |
| `make open` | Build and open this local Project in the released macOS Apple Silicon Host. |
| `make docs` | Build the wiki to `build.nosync/site`. |
| `make serve-docs` | Preview the wiki; `DOCS_PORT` defaults to 8080. |
| `make validate-docs` | Validate wiki content. |
| `make clean` | Remove all of `build.nosync/`, `tools/` and `gams_modules/`, including unmarked contents; refuse tracked files and symlinks before deleting anything. |

The local Project browses files with pinned published Units; it is not a tilemap editor or runtime test. Downloads preserve accompanying notices and existing Host tools. Packaging does not verify native Host/API behavior.
