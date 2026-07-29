package style

import "looporbit/internal/config"

func DefaultStyleConfig() config.StyleConfig {
	styleConfig, _ := ThemeConfigByKey("dark")
	return styleConfig
}

func resolvedStyleConfig(styleConfig config.StyleConfig) config.StyleConfig {
	if styleConfig == (config.StyleConfig{}) {
		return DefaultStyleConfig()
	}
	return styleConfig
}

func ThemeConfigByKey(key string) (config.StyleConfig, bool) {
	switch key {
	case "dark":
		return config.StyleConfig{
			Palette: config.StylePalette{
				Background: "#02060d",
				Foreground: "#f2f5f6",
				Muted:      "#9ca3aa",
				Accent:     "#12dce8",
				PanelFill:  "#020b12",
				OptionFill: "#06262c",
				Divider:    "#0e6a74",
			},
			Layout: defaultLayout(),
		}, true
	case "light":
		return config.StyleConfig{
			Palette: config.StylePalette{
				Background: "#f4f7f8",
				Foreground: "#182327",
				Muted:      "#607079",
				Accent:     "#008c99",
				PanelFill:  "#edf4f6",
				OptionFill: "#dff3f6",
				Divider:    "#72b9c1",
			},
			Layout: defaultLayout(),
		}, true
	case "high-contrast":
		return config.StyleConfig{
			Palette: config.StylePalette{
				Background: "#000000",
				Foreground: "#ffffff",
				Muted:      "#c7d0d4",
				Accent:     "#00f0ff",
				PanelFill:  "#050505",
				OptionFill: "#002e34",
				Divider:    "#00f0ff",
			},
			Layout: defaultLayout(),
		}, true
	default:
		return config.StyleConfig{}, false
	}
}

func defaultLayout() config.StyleLayout {
	return config.StyleLayout{
		FallbackWidth:  120,
		FallbackHeight: 34,
		MinWidth:       48,
		MinHeight:      20,
		MaxPanelWidth:  96,
		PanelHeight:    23,
		MinPanelHeight: 12,
	}
}
