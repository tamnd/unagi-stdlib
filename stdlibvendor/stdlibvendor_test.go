package stdlibvendor

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGitBlobHash pins that GitBlobHash reproduces git's own object hash, so a
// manifest row can be checked against the upstream object graph. The expected
// value is `printf 'hello\n' | git hash-object --stdin`.
func TestGitBlobHash(t *testing.T) {
	got := GitBlobHash([]byte("hello\n"))
	want := "ce013625030ba8dba906f756967f9e9ca394464a"
	if got != want {
		t.Fatalf("GitBlobHash = %s, want %s", got, want)
	}
	// The empty blob is a well-known fixed value.
	if got := GitBlobHash(nil); got != "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391" {
		t.Fatalf("empty GitBlobHash = %s", got)
	}
}

func TestSHA256Hex(t *testing.T) {
	// sha256 of the empty input is a known constant.
	if got := SHA256Hex(nil); got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("empty SHA256Hex = %s", got)
	}
}

func TestLineCount(t *testing.T) {
	cases := map[string]int{"": 0, "a": 0, "a\n": 1, "a\nb\n": 2, "\n\n": 2}
	for in, want := range cases {
		if got := LineCount([]byte(in)); got != want {
			t.Errorf("LineCount(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestScopeEnumerate(t *testing.T) {
	root := t.TempDir()
	files := []string{
		"Lib/os.py",
		"Lib/json/__init__.py",
		"Lib/test/test_os.py",
		"Lib/turtle.py",
		"Lib/encodings/utf_8.py",
		"Lib/README.txt",
		"Lib/__pycache__/os.cpython-314.pyc",
	}
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	scope := Scope{
		ExcludeDirs:   []string{"Lib/test"},
		ExcludeFiles:  []string{"Lib/turtle.py"},
		IncludeSuffix: []string{".py"},
	}
	got, err := Enumerate(root, scope)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Lib/encodings/utf_8.py", "Lib/json/__init__.py", "Lib/os.py"}
	if len(got) != len(want) {
		t.Fatalf("Enumerate = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Enumerate[%d] = %s, want %s (full %v)", i, got[i], want[i], got)
		}
	}
}

func TestScopeReinclude(t *testing.T) {
	root := t.TempDir()
	files := []string{
		"Lib/os.py",
		"Lib/test/__init__.py",
		"Lib/test/test_os.py",
		"Lib/test/support/__init__.py",
		"Lib/test/support/os_helper.py",
		"Lib/test/support/data.txt",
	}
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	scope := Scope{
		ExcludeDirs:   []string{"Lib/test"},
		IncludeSuffix: []string{".py"},
		Reinclude:     []string{"Lib/test/__init__.py", "Lib/test/support"},
	}
	got, err := Enumerate(root, scope)
	if err != nil {
		t.Fatal(err)
	}
	// The named file and subtree come back; test_os.py stays dropped by the
	// Lib/test exclude, and data.txt by the .py suffix gate.
	want := []string{
		"Lib/os.py",
		"Lib/test/__init__.py",
		"Lib/test/support/__init__.py",
		"Lib/test/support/os_helper.py",
	}
	if len(got) != len(want) {
		t.Fatalf("Enumerate = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Enumerate[%d] = %s, want %s (full %v)", i, got[i], want[i], got)
		}
	}
}

func TestDiff(t *testing.T) {
	prev := []Entry{
		{Path: "Lib/a.py", SHA256: "1"},
		{Path: "Lib/b.py", SHA256: "2"},
		{Path: "Lib/c.py", SHA256: "3"},
	}
	cur := []Entry{
		{Path: "Lib/a.py", SHA256: "1"}, // unchanged
		{Path: "Lib/b.py", SHA256: "9"}, // changed
		{Path: "Lib/d.py", SHA256: "4"}, // added
	}
	added, removed, changed := Diff(prev, cur)
	if len(added) != 1 || added[0] != "Lib/d.py" {
		t.Errorf("added = %v", added)
	}
	if len(removed) != 1 || removed[0] != "Lib/c.py" {
		t.Errorf("removed = %v", removed)
	}
	if len(changed) != 1 || changed[0] != "Lib/b.py" {
		t.Errorf("changed = %v", changed)
	}
}

func TestManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "MANIFEST.sha256")
	entries := []Entry{
		{Path: "Lib/b.py", SHA256: "bb", GitBlob: "b2"},
		{Path: "Lib/a.py", SHA256: "aa", GitBlob: "a1"},
	}
	if err := WriteManifest(path, "v3.14.6", "deadbeef", entries); err != nil {
		t.Fatal(err)
	}
	hdr, got, err := ParseManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if hdr.Tag != "v3.14.6" || hdr.Commit != "deadbeef" || hdr.Files != 2 {
		t.Fatalf("header = %+v", hdr)
	}
	// Written sorted by path, so a.py precedes b.py.
	if len(got) != 2 || got[0].Path != "Lib/a.py" || got[1].Path != "Lib/b.py" {
		t.Fatalf("entries = %+v", got)
	}
	if got[0].SHA256 != "aa" || got[0].GitBlob != "a1" {
		t.Fatalf("entry[0] = %+v", got[0])
	}
}

func TestLoadProvenance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "PROVENANCE.toml")
	content := `[upstream]
tag = "v3.14.6"
commit = "abc123"
subtree = "Lib"

[scope]
exclude_dirs = [
  "Lib/test",
  "Lib/tkinter",
]
exclude_files = ["Lib/turtle.py"]
include_only_suffix = [".py"]
reinclude = [
  "Lib/test/__init__.py",
  "Lib/test/support",
]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadProvenance(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Tag != "v3.14.6" || p.Commit != "abc123" || p.Subtree != "Lib" {
		t.Fatalf("upstream = %+v", p)
	}
	if len(p.Scope.ExcludeDirs) != 2 || p.Scope.ExcludeDirs[1] != "Lib/tkinter" {
		t.Fatalf("exclude_dirs = %v", p.Scope.ExcludeDirs)
	}
	if len(p.Scope.ExcludeFiles) != 1 || p.Scope.ExcludeFiles[0] != "Lib/turtle.py" {
		t.Fatalf("exclude_files = %v", p.Scope.ExcludeFiles)
	}
	if len(p.Scope.IncludeSuffix) != 1 || p.Scope.IncludeSuffix[0] != ".py" {
		t.Fatalf("include_only_suffix = %v", p.Scope.IncludeSuffix)
	}
	if len(p.Scope.Reinclude) != 2 || p.Scope.Reinclude[1] != "Lib/test/support" {
		t.Fatalf("reinclude = %v", p.Scope.Reinclude)
	}
}

// TestEmitProvenanceRoundTrips pins the determinism property: EmitProvenance's
// output parses back to the same pin and scope, so re-running sync at a fixed
// tag reproduces PROVENANCE.toml except for generated_at.
func TestEmitProvenanceRoundTrips(t *testing.T) {
	p := &Provenance{
		Tag:     "v3.14.6",
		Commit:  "abc123",
		Subtree: "Lib",
		Scope: Scope{
			ExcludeDirs:   []string{"Lib/test"},
			ExcludeFiles:  []string{"Lib/turtle.py"},
			IncludeSuffix: []string{".py"},
			Reinclude:     []string{"Lib/test/__init__.py", "Lib/test/support"},
		},
	}
	text := EmitProvenance(p, 557, 281620, "2026-07-19T00:00:00Z")
	dir := t.TempDir()
	path := filepath.Join(dir, "PROVENANCE.toml")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadProvenance(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != p.Tag || got.Commit != p.Commit || got.Subtree != p.Subtree {
		t.Fatalf("round-trip upstream = %+v", got)
	}
	if len(got.Scope.ExcludeDirs) != 1 || got.Scope.ExcludeDirs[0] != "Lib/test" {
		t.Fatalf("round-trip scope = %+v", got.Scope)
	}
	if len(got.Scope.Reinclude) != 2 || got.Scope.Reinclude[0] != "Lib/test/__init__.py" {
		t.Fatalf("round-trip reinclude = %+v", got.Scope.Reinclude)
	}
}
