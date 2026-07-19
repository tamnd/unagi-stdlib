package stdlibvendor

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// RunVerify enforces the zero-patch invariant on the vendored tree rooted at
// out. It checks two independent axes and fails on any divergence:
//
//   - Local self-consistency: every file under out/Lib is in the manifest with
//     a matching sha256, and every manifest path exists on disk. Neither an
//     edited file nor an added file nor a deleted file passes.
//   - Upstream identity: every manifest git blob hash equals the blob hash of
//     that path in the CPython source at cpython. A self-consistent manifest
//     computed over locally-edited files still fails here, because edited bytes
//     have a different blob hash than upstream's object.
//
// offline skips the upstream axis for environments without a CPython checkout.
// It writes a per-divergence report to w and returns a non-nil error if the
// tree is not a verbatim mirror.
func RunVerify(out, cpython string, offline bool, w io.Writer) error {
	manifestPath := filepath.Join(out, "MANIFEST.sha256")
	hdr, entries, err := ParseManifest(manifestPath)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("empty or missing manifest at %s", manifestPath)
	}

	var edited, untracked, missing, blobMismatch []string
	manifestPaths := make(map[string]Entry, len(entries))
	for _, e := range entries {
		manifestPaths[e.Path] = e
	}

	// Local self-consistency: recompute sha256 of every manifest file.
	for _, e := range entries {
		content, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(e.Path)))
		if os.IsNotExist(err) {
			missing = append(missing, e.Path)
			continue
		}
		if err != nil {
			return err
		}
		if SHA256Hex(content) != e.SHA256 {
			edited = append(edited, e.Path)
		}
	}

	// Set equality: any file under out/Lib that is not in the manifest.
	libRoot := filepath.Join(out, "Lib")
	err = filepath.WalkDir(libRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(out, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if _, ok := manifestPaths[rel]; !ok {
			untracked = append(untracked, rel)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Upstream identity: re-derive each blob hash from the CPython source.
	if !offline {
		if cpython == "" {
			return fmt.Errorf("upstream identity check needs --cpython <path>; pass --offline to skip it")
		}
		for _, e := range entries {
			content, err := os.ReadFile(filepath.Join(cpython, filepath.FromSlash(e.Path)))
			if os.IsNotExist(err) {
				blobMismatch = append(blobMismatch, e.Path+" (absent upstream)")
				continue
			}
			if err != nil {
				return err
			}
			if GitBlobHash(content) != e.GitBlob {
				blobMismatch = append(blobMismatch, e.Path)
			}
		}
	}

	sort.Strings(edited)
	sort.Strings(untracked)
	sort.Strings(missing)
	sort.Strings(blobMismatch)

	total := len(edited) + len(untracked) + len(missing) + len(blobMismatch)
	if total == 0 {
		fmt.Fprintf(w, "OK: %d files verbatim against %s (commit %s)\n", len(entries), hdr.Tag, shortCommit(hdr.Commit))
		if offline {
			fmt.Fprintln(w, "note: upstream identity axis skipped (--offline)")
		}
		return nil
	}

	fmt.Fprintf(w, "FAIL: verbatim drift detected against %s (commit %s)\n\n", hdr.Tag, shortCommit(hdr.Commit))
	for _, p := range edited {
		fmt.Fprintln(w, "  edited       ", p)
	}
	for _, p := range untracked {
		fmt.Fprintln(w, "  untracked    ", p, "(present on disk, absent from manifest)")
	}
	for _, p := range missing {
		fmt.Fprintln(w, "  missing      ", p, "(in manifest, absent on disk)")
	}
	for _, p := range blobMismatch {
		fmt.Fprintln(w, "  blob-mismatch", p, "(bytes differ from the upstream CPython object)")
	}
	fmt.Fprintf(w, "\n%d problems; run cmd/unagi-vendor to re-sync from upstream, do not hand-edit.\n", total)
	return fmt.Errorf("verbatim drift: %d problems", total)
}

func shortCommit(c string) string {
	if len(c) > 12 {
		return c[:12]
	}
	return c
}
