package services

import "path/filepath"

// DataDir is the root data directory, set from config at startup.
var DataDir = "data"

// DataPath joins DataDir with the given path components.
func DataPath(parts ...string) string {
	return filepath.Join(append([]string{DataDir}, parts...)...)
}
