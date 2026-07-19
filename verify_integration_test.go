package unagistdlib_test

import (
	"io"
	"testing"

	"github.com/tamnd/unagi-stdlib/stdlibvendor"
)

// TestManifestVerifiesOffline re-runs the drift check against the committed
// tree on every CI run, so an edit to a vendored file that slips past review
// fails the test job as well as the dedicated drift workflow. It runs offline,
// so it needs no CPython checkout: it proves the tree is self-consistent with
// the committed manifest. The drift workflow adds the upstream-identity axis.
func TestManifestVerifiesOffline(t *testing.T) {
	if err := stdlibvendor.RunVerify(".", "", true, io.Discard); err != nil {
		t.Fatalf("committed tree failed the offline drift check: %v", err)
	}
}
