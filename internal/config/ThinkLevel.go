package config

const DefaultThinkLevel = "high"

var thinkLevelOptions = []string{"low", "medium", "high", "xhigh", "max"}

// ThinkLevelOptions returns the supported reasoning effort levels.
func ThinkLevelOptions() []string {
	return append([]string(nil), thinkLevelOptions...)
}

// NormalizeThinkLevel falls back to the default for missing or invalid values.
func NormalizeThinkLevel(thinkLevel string) string {
	for _, option := range thinkLevelOptions {
		if thinkLevel == option {
			return thinkLevel
		}
	}
	return DefaultThinkLevel
}
