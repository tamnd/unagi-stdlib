// Package stdlibvendor holds the shared machinery behind the two vendoring
// commands: enumerating the in-scope CPython Lib subtree, copying it verbatim,
// hashing every file two ways, and reading and writing the manifest and the
// provenance pin.
//
// It carries no third-party dependencies on purpose. This module is a go.mod
// dependency of tamnd/unagi, so anything imported here would leak into that
// build's go.sum, and the whole point of the repo is to embed verbatim CPython
// behind a single embed.FS with nothing else riding along.
package stdlibvendor

import (
	"bufio"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Scope is the machine-readable form of the vendor-versus-drop rules. It is the
// [scope] block of PROVENANCE.toml: which directories and files under Lib are
// dropped, which suffixes are copied, and which paths under a dropped directory
// are pulled back in.
type Scope struct {
	ExcludeDirs   []string
	ExcludeFiles  []string
	IncludeSuffix []string
	Reinclude     []string
}

// excluded reports whether a Lib-relative slash path is dropped by the scope. A
// reinclude entry wins over an exclude, so a named subtree or file under an
// otherwise-dropped directory is vendored: that is how the test/support helpers
// come in while the rest of Lib/test stays out.
func (s Scope) excluded(rel string) bool {
	if s.reincluded(rel) {
		return false
	}
	if slices.Contains(s.ExcludeFiles, rel) {
		return true
	}
	for _, d := range s.ExcludeDirs {
		if rel == d || strings.HasPrefix(rel, d+"/") {
			return true
		}
	}
	return false
}

// reincluded reports whether rel is a reinclude entry or sits under one. An
// entry is matched as an exact path or as a directory prefix, so both a single
// file (Lib/test/__init__.py) and a subtree (Lib/test/support) pull back in.
func (s Scope) reincluded(rel string) bool {
	for _, r := range s.Reinclude {
		if rel == r || strings.HasPrefix(rel, r+"/") {
			return true
		}
	}
	return false
}

// included reports whether a path's suffix is in the copy set.
func (s Scope) included(rel string) bool {
	for _, suf := range s.IncludeSuffix {
		if strings.HasSuffix(rel, suf) {
			return true
		}
	}
	return false
}

// Provenance is the parsed PROVENANCE.toml: the upstream pin plus the scope.
type Provenance struct {
	Tag     string
	Commit  string
	Subtree string
	Scope   Scope
}

// Entry is one manifest row: a Lib-relative path bound to the sha256 of its
// exact bytes and to the git blob hash of the upstream object it came from.
type Entry struct {
	Path    string
	SHA256  string
	GitBlob string
}

// SHA256Hex returns the lowercase hex sha256 of content.
func SHA256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// GitBlobHash returns the git blob sha1 of content: the hash git itself would
// store the file under, so a manifest row can be checked against the upstream
// object graph, not just against a local recompute.
func GitBlobHash(content []byte) string {
	h := sha1.New()
	_, _ = fmt.Fprintf(h, "blob %d\x00", len(content))
	_, _ = h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// Enumerate walks srcRoot/Lib, applies the scope, and returns the sorted list
// of in-scope paths relative to srcRoot (each like "Lib/functools.py"). The
// walk is deterministic: same tree and same scope yield the same list.
func Enumerate(srcRoot string, scope Scope) ([]string, error) {
	libRoot := filepath.Join(srcRoot, "Lib")
	var out []string
	err := filepath.WalkDir(libRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcRoot, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if scope.excluded(rel) || !scope.included(rel) {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// LineCount counts the lines in content the way `wc -l` does: the number of
// newline bytes. It exists so PROVENANCE.toml can record a stable total.
func LineCount(content []byte) int {
	n := 0
	for _, b := range content {
		if b == '\n' {
			n++
		}
	}
	return n
}

// BuildManifest reads each in-scope file from srcRoot, hashes it two ways, and
// returns the manifest entries plus the total line count across all files.
func BuildManifest(srcRoot string, paths []string) ([]Entry, int, error) {
	entries := make([]Entry, 0, len(paths))
	totalLines := 0
	for _, rel := range paths {
		content, err := os.ReadFile(filepath.Join(srcRoot, filepath.FromSlash(rel)))
		if err != nil {
			return nil, 0, err
		}
		entries = append(entries, Entry{
			Path:    rel,
			SHA256:  SHA256Hex(content),
			GitBlob: GitBlobHash(content),
		})
		totalLines += LineCount(content)
	}
	return entries, totalLines, nil
}

// WriteManifest writes the manifest in the fixed columnar format, sorted by
// path, with a header recording the upstream tag and commit and the file count.
func WriteManifest(pathOut, tag, commit string, entries []Entry) error {
	sorted := append([]Entry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	var b strings.Builder
	b.WriteString("# unagi-stdlib manifest, generated by cmd/unagi-vendor\n")
	fmt.Fprintf(&b, "# upstream: cpython tag %s commit %s\n", tag, commit)
	fmt.Fprintf(&b, "# files: %d\n", len(sorted))
	b.WriteString("# columns: sha256  git-blob-sha1  path\n")
	for _, e := range sorted {
		fmt.Fprintf(&b, "%s  %s  %s\n", e.SHA256, e.GitBlob, e.Path)
	}
	return os.WriteFile(pathOut, []byte(b.String()), 0o644)
}

// ManifestHeader is the parsed header of a manifest file.
type ManifestHeader struct {
	Tag    string
	Commit string
	Files  int
}

// ParseManifest reads a manifest file into its header and entries. A missing
// file is not an error: it returns a zero header and no entries, so the sync
// tool can diff a first run against an empty previous manifest.
func ParseManifest(pathIn string) (ManifestHeader, []Entry, error) {
	var hdr ManifestHeader
	f, err := os.Open(pathIn)
	if os.IsNotExist(err) {
		return hdr, nil, nil
	}
	if err != nil {
		return hdr, nil, err
	}
	defer func() { _ = f.Close() }()

	var entries []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") {
			switch {
			case strings.Contains(line, "upstream: cpython tag"):
				fields := strings.Fields(line)
				for i, f := range fields {
					if f == "tag" && i+1 < len(fields) {
						hdr.Tag = fields[i+1]
					}
					if f == "commit" && i+1 < len(fields) {
						hdr.Commit = fields[i+1]
					}
				}
			case strings.HasPrefix(line, "# files:"):
				_, _ = fmt.Sscanf(line, "# files: %d", &hdr.Files)
			}
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return hdr, nil, fmt.Errorf("malformed manifest line: %q", line)
		}
		entries = append(entries, Entry{SHA256: fields[0], GitBlob: fields[1], Path: fields[2]})
	}
	if err := sc.Err(); err != nil {
		return hdr, nil, err
	}
	return hdr, entries, nil
}

// Diff reports the churn between a previous and a current manifest: which paths
// were added, which were removed, and which changed content (same path, new
// sha256). Every slice is sorted for a stable report.
func Diff(prev, cur []Entry) (added, removed, changed []string) {
	prevByPath := make(map[string]Entry, len(prev))
	for _, e := range prev {
		prevByPath[e.Path] = e
	}
	curByPath := make(map[string]Entry, len(cur))
	for _, e := range cur {
		curByPath[e.Path] = e
		p, ok := prevByPath[e.Path]
		switch {
		case !ok:
			added = append(added, e.Path)
		case p.SHA256 != e.SHA256:
			changed = append(changed, e.Path)
		}
	}
	for _, e := range prev {
		if _, ok := curByPath[e.Path]; !ok {
			removed = append(removed, e.Path)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(changed)
	return added, removed, changed
}

// Base is a small helper for callers that want the last path element.
func Base(p string) string { return path.Base(p) }
