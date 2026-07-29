package style

import "testing"

func TestThemeConfigByKeyReturnsConcreteStyleValues(t *testing.T) {
	styleConfig, ok := ThemeConfigByKey("dark")
	if !ok {
		t.Fatal("expected dark theme config to exist")
	}

	if styleConfig.Palette.Accent != "#12dce8" {
		t.Fatalf("expected concrete accent color, got %q", styleConfig.Palette.Accent)
	}
	if styleConfig.Layout.PanelHeight != 23 {
		t.Fatalf("expected concrete panel height 23, got %d", styleConfig.Layout.PanelHeight)
	}
}
