// Package services provides shared helpers for plugins.
package services

import (
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

// GetRandomImage returns the path to a random file in data/images/<folder>/.
// If keyword is non-empty, only files whose names contain it are considered.
func GetRandomImage(folder, keyword string) (string, error) {
	dir := filepath.Join("data", "images", folder)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if keyword == "" || strings.Contains(name, keyword) {
			files = append(files, filepath.Join(dir, name))
		}
	}
	if len(files) == 0 {
		return "", errors.New("no images found")
	}
	return files[rand.Intn(len(files))], nil
}

// ListImageNames returns the stem names (no extension) of files matching keyword.
func ListImageNames(folder, keyword string) ([]string, error) {
	dir := filepath.Join("data", "images", folder)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if keyword == "" || strings.Contains(name, keyword) {
			ext := filepath.Ext(name)
			names = append(names, strings.TrimSuffix(name, ext))
		}
	}
	return names, nil
}
