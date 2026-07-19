// Command unagi-stdlib-verify is the drift check that enforces the zero-patch
// invariant: it recomputes the sha256 of every file under Lib and compares it
// to MANIFEST.sha256, and it compares the recorded git blob hashes to the
// upstream CPython objects, failing on any mismatch on either axis.
//
// It exits 0 when every file matches on both axes, and non-zero with a
// per-file report otherwise. CI runs it as a required status.
//
// Usage:
//
//	unagi-stdlib-verify [--out .] [--cpython <path>] [--offline]
//
// --offline skips the upstream-identity axis for environments without a
// CPython checkout, checking only local self-consistency against the manifest.
package main

import (
	"fmt"
	"os"

	"github.com/tamnd/unagi-stdlib/stdlibvendor"
)

func main() {
	out := "."
	cpython := ""
	offline := false
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out":
			i++
			if i < len(args) {
				out = args[i]
			}
		case "--cpython":
			i++
			if i < len(args) {
				cpython = args[i]
			}
		case "--offline":
			offline = true
		case "-h", "--help":
			fmt.Println("usage: unagi-stdlib-verify [--out .] [--cpython <path>] [--offline]")
			return
		}
	}

	if err := stdlibvendor.RunVerify(out, cpython, offline, os.Stdout); err != nil {
		os.Exit(1)
	}
}
