# unagi-stdlib

The CPython 3.14.6 pure-Python standard library, vendored verbatim, embedded as
a Go `embed.FS`, and proven byte-for-byte upstream by a manifest and a drift
check. It is the companion repo of [tamnd/unagi](https://github.com/tamnd/unagi),
the Python-to-Go compiler: unagi depends on this module and treats the embedded
`Lib` tree as its standard-library search root.

The whole idea is to run the standard library **without modifying it**. Files
under `Lib/` are copied exactly as the Python Software Foundation released them,
under the PSF License Agreement (see `LICENSE` and `NOTICE`). We author only the
machinery around the mirror: the manifest, the sync tool, the drift check, and
the embed wrapper.

## Layout

```
Lib/                      verbatim CPython Lib subtree (in-scope modules only)
MANIFEST.sha256           per-file sha256 + upstream git blob hash, sorted by path
PROVENANCE.toml           the pin (tag, commit, subtree) and the vendor scope rules
embed.go                  //go:embed Lib -> exported FS(), Version, CPythonTag
LICENSE / NOTICE          PSF License Agreement and attribution
cmd/unagi-vendor/         sync tool: turn a CPython tag into the mirror + manifest
cmd/unagi-stdlib-verify/  drift check: enforce the zero-patch invariant
stdlibvendor/             shared vendoring machinery (no third-party deps)
```

## Verify

The drift check recomputes the sha256 of every file under `Lib` and compares it
to the manifest, and compares the recorded git blob hashes to the upstream
CPython objects. It is a required CI status.

```sh
# Local self-consistency only (no CPython checkout needed):
go run ./cmd/unagi-stdlib-verify --offline

# Full check, including upstream identity against a CPython source tree:
go run ./cmd/unagi-stdlib-verify --cpython /path/to/cpython
```

A green run means every file under `Lib` is byte-identical to CPython v3.14.6.
Any edit, added file, or deleted file turns it red. The remedy is never to
hand-edit toward green: re-run the sync tool, or revert.

## Bump to a new CPython patch

A patch bump touches only this repo. Edit the pin in `PROVENANCE.toml`, then:

```sh
go run ./cmd/unagi-vendor sync --cpython /path/to/cpython   # re-copy + re-manifest
go run ./cmd/unagi-vendor diff --cpython /path/to/cpython   # preview churn first
```

Review the churn report against CPython's changelog, update `Version` and the
pin constants in `embed.go`, confirm the drift check is green, and open a PR.
Because we carry no local edits, the diff under `Lib` is exactly upstream's diff
for the in-scope subtree, with no merge to resolve.

## License

`Lib/` is CPython under the PSF License Agreement, redistributed verbatim. The
surrounding machinery is authored by Duc-Tam Nguyen and released as part of this
repository. See `LICENSE` and `NOTICE`.
