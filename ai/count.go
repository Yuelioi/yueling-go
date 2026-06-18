package ai

// ResolveCount picks a message-count for a context tool. When the model omits
// or gives an invalid count (provided < 1) the configured default is used.
// The result is always clamped to [min, max], so a misconfigured default can
// never push a request out of the safe range.
func ResolveCount(provided, def, min, max int) int {
	count := provided
	if count < 1 {
		count = def
	}
	if count < min {
		count = min
	}
	if count > max {
		count = max
	}
	return count
}
