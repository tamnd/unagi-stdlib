package unagistdlib_test

import (
	"io/fs"
	"strings"
	"testing"

	unagistdlib "github.com/tamnd/unagi-stdlib"
)

// TestFSHasCoreModules pins that the embed surface resolves the path shapes the
// compiler relies on: a plain module, a regular package init, a nested
// submodule, and a codec file, each at its CPython Lib path.
func TestFSHasCoreModules(t *testing.T) {
	want := []string{
		"Lib/os.py",
		"Lib/json/__init__.py",
		"Lib/xml/etree/ElementTree.py",
		"Lib/encodings/utf_8.py",
		"Lib/_collections_abc.py",
	}
	f := unagistdlib.FS()
	for _, p := range want {
		if _, err := f.Open(p); err != nil {
			t.Errorf("embedded FS missing %s: %v", p, err)
		}
	}
}

// TestFSExcludesDroppedTrees pins that the dropped trees never made it into the
// embed surface, so a build cannot accidentally reach GUI or test code.
func TestFSExcludesDroppedTrees(t *testing.T) {
	deny := []string{
		"Lib/test/test_os.py",
		"Lib/tkinter/__init__.py",
		"Lib/idlelib/idle.py",
		"Lib/turtle.py",
		"Lib/_scproxy.py",
	}
	f := unagistdlib.FS()
	for _, p := range deny {
		if _, err := f.Open(p); err == nil {
			t.Errorf("embedded FS unexpectedly contains dropped path %s", p)
		}
	}
}

// TestEmbedIsAllPy is a coarse guard on what rode into the embed. Everything
// outside Lib/test is .py, matching the include_only_suffix scope rule. Under
// Lib/test a few non-.py data files come in on purpose: the reinclude list
// carries the conformance suites' fixtures (the cjkencodings codec strings),
// and those are the only allowed exception.
func TestEmbedIsAllPy(t *testing.T) {
	f := unagistdlib.FS()
	err := fs.WalkDir(f, "Lib", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".py") {
			return nil
		}
		if strings.HasPrefix(p, "Lib/test/") {
			return nil
		}
		t.Errorf("non-.py file embedded outside Lib/test: %s", p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
