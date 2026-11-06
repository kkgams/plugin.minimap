# plugin.minimap

GAMS WASM Project Unit for generating tile maps from trees.
Distribution and WIT interface versions are independent.

## API

Exports `gams:minimap/minimap@1.0.0` through the `generator` world.
`gen(tree)` returns a tile map or an error. Tree nodes contain string-pair
properties and a parent id. Returned maps contain layers with widths, flat tile
arrays, and properties. The Host supplies the world's WASI imports.

The contract is in `plugins/minimap.comp/wit/package.wit`.
Plugin Manager resolves imports; distribution filenames do not rename WIT identities.

## Build and test

All source/build inputs are owned by this repository; no sibling checkout is
required for its supported build/test commands. Use the pinned Nix environment:

```sh
nix develop --command make build
nix develop --command make test
```

Output: `dist/plugin.minimap.wasm`. Install it as
`plugins/minimap.comp.wasm` in an external GAMS Project. Configure that Project separately.

The flake and lockfiles pin the toolchain. Network access may be required to
fetch tools/modules. `.envrc` supports `direnv allow`.

`make test` runs offline integrity/publication regression tests, builds the
components, validates WASM, extracts WIT, and checks the exact input inventory.
It also runs native Go unit tests for the plugin and its local SDK. Native Go
tests are not WASM runtime tests; there is no standalone component runtime E2E.

## Licensing and releases

GAMS-authored contributions are Apache-2.0. Third-party code retains its own
terms; see `LICENSING.md`, `THIRD-PARTY-REVIEW.md`, and `LICENSES/`.
Source inventory and notice evidence are checked before candidate packaging.
Changed inputs require separately authorized evidence refreshes; source snapshot
verification remains strict and is not refreshed by release tooling.

Distribution is through GitHub Releases, not npm or OCI. Pushing a matching
`v<version.txt>` tag authorizes publication from that commit; no approval digest
variables or required publication branch are used. Manual workflow dispatch
verifies without publishing. Build/tests, canonical repository and tag/version
identity, legal texts and embedded notices, evidence/source inventory, artifact
checksums, candidate integrity, and no-overwrite checks must pass. Never reuse
or move a published or failed tag. An existing release blocks creation.

Enable immutable releases in the repository settings before publication. The
automatic GitHub token is sufficient; no administrative settings API or extra
policy token is used. Public release verification requires `immutable: true`.

The publication workflow rehearses a private draft's bytes before publication
and checks the anonymous public bytes afterward. Keep LICENSE, NOTICE, evidence,
review documents, and applicable LICENSES with distributed artifacts. See the
repository's release workflow and scripts for the enforced gates.

## Local Project and documentation

```sh
nix develop --command make open  # build, then open this repository's gams.json
make serve-docs                 # http://localhost:8080; DOCS_PORT overrides
make validate-docs
make docs                       # portable site under build/site/
```

`make open` builds `dist/plugin.minimap.wasm` and explicitly sets `GAMS_APP_CWD`
to this repository. The local Project loads that component and the pinned
released Files View to inspect Project files; it is not an API demo.
The shell uses the released layout provider.
No sibling source checkout is required.

`make tools/gams.app` downloads the released macOS Apple Silicon Host only
when absent and retains its notices under `tools/gams-release-notices/`.
Existing downloads are not updated implicitly; macOS security checks remain
intact. The first download and Project launch require network access.
Pinned dependencies are declared in `gams.json`; Host caches live in ignored
`gams_modules/`. Native GUI behavior requires separate verification.

Documentation uses pinned justgook/wiki v1.4.0, with ignored engine/output
under `build/`. `build-docs` remains a build alias. GitHub Pages runs `make docs`
on every push to `release`, independently of component release tags.
Enable **Settings → Pages → Source → GitHub Actions**. Existing component
build, runtime tests, candidate checks and matching-tag release rules are unchanged.
