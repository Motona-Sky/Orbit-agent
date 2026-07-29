package tools_test

import (
	"looporbit/internal/tools"
	"testing"
)

// maxTreeDepth limits recursion depth to guard against symlink loops.

// func TestLstree(t *testing.T) {
// 	paths, err := lsTree("F:\\LoopOrbit-agent", 2)
// 	if err != nil {
// 		t.Errorf("lsTree failed: %v", err)
// 	}
// 	t.Logf("lsTree: %v", paths)
// }

func TestLs(t *testing.T) {
	files, err := tools.Ls("F:\\LoopOrbit-agent", true, 2)
	if err != nil {
		t.Errorf("Ls failed: %v", err)
	}
	t.Logf("Ls: %v", files)
}
