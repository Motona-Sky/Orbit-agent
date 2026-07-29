package cli

import "looporbit/internal/config"

func orbitalViewport(width, height int, styleConfig config.StyleConfig) (int, int) {
	layout := styleConfig.Layout
	if width <= 0 {
		width = layout.FallbackWidth
	}
	if width < layout.MinWidth {
		width = layout.MinWidth
	}

	if height <= 0 {
		height = layout.FallbackHeight
	}
	if height < layout.MinHeight {
		height = layout.MinHeight
	}

	return width, height
}
