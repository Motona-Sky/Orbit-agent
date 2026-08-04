package tools_test

import (
	"orbit/internal/tools"
	"testing"
)

// maxTreeDepth limits recursion depth to guard against symlink loops.

// func TestLstree(t *testing.T) {
// 	paths, err := lsTree(t.TempDir(), 2)
// 	if err != nil {
// 		t.Errorf("lsTree failed: %v", err)
// 	}
// 	t.Logf("lsTree: %v", paths)
// }

func TestLs(t *testing.T) {
	files, err := tools.Ls(t.TempDir(), true, 2)
	if err != nil {
		t.Errorf("Ls failed: %v", err)
	}
	t.Logf("Ls: %v", files)
}
