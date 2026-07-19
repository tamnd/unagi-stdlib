// Package unagistdlib embeds the CPython 3.14.6 pure-Python standard library,
// copied verbatim, and exposes it as a single embed.FS. It is the runtime
// surface of tamnd/unagi-stdlib: the compiler depends on this module, calls
// FS(), and treats the result as its stdlib search root, so the exact stdlib
// bytes travel inside the compiled binary and the go.mod pin is the stdlib pin.
//
// Nothing under Lib is our work. It is the standard library of CPython, under
// the PSF License Agreement; see LICENSE and NOTICE. The MANIFEST.sha256 and
// the drift check prove the tree is byte-for-byte upstream.
package unagistdlib

import "embed"

//go:embed all:Lib
var lib embed.FS

// FS returns the embedded verbatim CPython Lib subtree, rooted so that an
// FS-relative path like "Lib/json/__init__.py" is the vendored json package
// init, matching the way CPython resolves an import against its Lib directory.
func FS() embed.FS { return lib }

// Version is the stdlib pin this build embeds: the CPython release plus a
// vendoring revision, for example "3.14.6-1".
const Version = "3.14.6-1"

// CPythonTag and CPythonCommit record the exact upstream provenance the
// embedded tree was vendored from.
const (
	CPythonTag    = "v3.14.6"
	CPythonCommit = "8594736f5057fdc979d42d2135895d56274589a8"
)
