// Command unagi-vendor turns a CPython source tree at a pinned tag into the
// verbatim in-scope Lib mirror plus MANIFEST.sha256, deterministically. Given
// the same tag and the same scope rules it produces byte-identical output, so a
// reviewer can re-run it and get zero diff.
//
// Usage:
//
//	unagi-vendor sync   --cpython <path> [--out .] [--dry-run]
//	unagi-vendor diff   --cpython <path> [--out .]
//	unagi-vendor verify --cpython <path> [--out .] [--offline]
//
// The pin and the scope come from PROVENANCE.toml in the output directory;
// sync rewrites its [result] block and MANIFEST.sha256. verify is a thin
// wrapper over the same check the standalone unagi-stdlib-verify runs.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tamnd/unagi-stdlib/stdlibvendor"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := parseFlags(os.Args[2:])

	var err error
	switch cmd {
	case "sync":
		err = runSync(fs, false)
	case "diff":
		err = runSync(fs, true)
	case "verify":
		err = stdlibvendor.RunVerify(fs.out, fs.cpython, fs.offline, os.Stdout)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "unagi-vendor:", err)
		os.Exit(1)
	}
}

type flags struct {
	cpython string
	out     string
	dryRun  bool
	offline bool
}

func parseFlags(args []string) flags {
	f := flags{out: "."}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--cpython":
			i++
			if i < len(args) {
				f.cpython = args[i]
			}
		case "--out":
			i++
			if i < len(args) {
				f.out = args[i]
			}
		case "--dry-run":
			f.dryRun = true
		case "--offline":
			f.offline = true
		}
	}
	return f
}

// runSync copies the in-scope tree, writes the manifest and provenance result,
// and prints the churn versus the previous manifest. In dryRun mode it does the
// enumeration and the diff but writes nothing, to preview a bump.
func runSync(f flags, dryRun bool) error {
	if f.cpython == "" {
		return fmt.Errorf("--cpython <path to a CPython v3.14.6 source tree> is required")
	}
	provPath := filepath.Join(f.out, "PROVENANCE.toml")
	prov, err := stdlibvendor.LoadProvenance(provPath)
	if err != nil {
		return err
	}

	paths, err := stdlibvendor.Enumerate(f.cpython, prov.Scope)
	if err != nil {
		return err
	}
	entries, lines, err := stdlibvendor.BuildManifest(f.cpython, paths)
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(f.out, "MANIFEST.sha256")
	_, prev, err := stdlibvendor.ParseManifest(manifestPath)
	if err != nil {
		return err
	}
	added, removed, changed := stdlibvendor.Diff(prev, entries)

	if dryRun || f.dryRun {
		reportChurn(f.out, added, removed, changed, len(entries), lines, true)
		return nil
	}

	// Copy each in-scope file verbatim, byte for byte, into out/Lib.
	for _, rel := range paths {
		src := filepath.Join(f.cpython, filepath.FromSlash(rel))
		dst := filepath.Join(f.out, filepath.FromSlash(rel))
		content, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, content, 0o644); err != nil {
			return err
		}
	}

	if err := stdlibvendor.WriteManifest(manifestPath, prov.Tag, prov.Commit, entries); err != nil {
		return err
	}
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	provText := stdlibvendor.EmitProvenance(prov, len(entries), lines, generatedAt)
	if err := os.WriteFile(provPath, []byte(provText), 0o644); err != nil {
		return err
	}

	reportChurn(f.out, added, removed, changed, len(entries), lines, false)
	return nil
}

func reportChurn(out string, added, removed, changed []string, files, lines int, dry bool) {
	verb := "synced"
	if dry {
		verb = "would sync"
	}
	fmt.Printf("%s %d files (%d lines) under %s/Lib\n", verb, files, lines, out)
	fmt.Printf("churn vs previous manifest: +%d added, -%d removed, ~%d changed\n",
		len(added), len(removed), len(changed))
	for _, p := range added {
		fmt.Println("  added   ", p)
	}
	for _, p := range removed {
		fmt.Println("  removed ", p)
	}
	for _, p := range changed {
		fmt.Println("  changed ", p)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `unagi-vendor: vendor the in-scope CPython Lib subtree verbatim

  unagi-vendor sync   --cpython <path> [--out .] [--dry-run]
  unagi-vendor diff   --cpython <path> [--out .]
  unagi-vendor verify --cpython <path> [--out .] [--offline]`)
}
