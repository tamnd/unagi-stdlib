package unagistdlib_test

import (
	"io/fs"
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

// TestEmbedIsAllPy is a coarse guard that nothing but .py rode into the embed,
// matching the include_only_suffix scope rule.
func TestEmbedIsAllPy(t *testing.T) {
	f := unagistdlib.FS()
	err := fs.WalkDir(f, "Lib", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if len(p) < 3 || p[len(p)-3:] != ".py" {
			t.Errorf("non-.py file embedded: %s", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
