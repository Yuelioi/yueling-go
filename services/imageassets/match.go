package imageassets

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/imaging"
)

type ImageMatch struct {
	Category string `json:"category"`
	Asset
}

type MatchPage struct {
	Assets []ImageMatch `json:"assets"`
	Total  int          `json:"total"`
	Number int          `json:"page"`
}

func (l *Library) MatchImage(ctx context.Context, data []byte, category, query string, page int, opts Options) (MatchPage, error) {
	result := MatchPage{Assets: []ImageMatch{}, Number: page}
	if page < 1 || page > 1000000 {
		return result, errors.New("页码必须在1至1000000之间")
	}
	hashes := map[[32]byte]bool{sha256.Sum256(data): true}
	_, err := imaging.Run(ctx, func() (string, error) {
		if _, err := validate(data); err != nil {
			return "", err
		}
		if opts.Convert {
			hashes[sha256.Sum256(services.ShrinkToJPEG(data, opts.MinBytes, opts.Quality))] = true
		}
		return "", nil
	})
	if err != nil {
		return result, err
	}
	categories := l.Categories()
	if category != "" {
		c, err := l.resolve(category)
		if err != nil {
			return result, err
		}
		categories = []Category{c}
	}
	var matches []ImageMatch
	seen := map[string]bool{}
	for _, category := range categories {
		if category.External || seen[category.Name] {
			continue
		}
		seen[category.Name] = true
		c, err := l.resolve(category.Name)
		if err != nil {
			return result, err
		}
		items, err := l.matchImage(ctx, c.Name, query, hashes)
		if err != nil {
			return result, err
		}
		matches = append(matches, items...)
	}
	result.Total = len(matches)
	start := (page - 1) * PageSize
	if start < len(matches) {
		result.Assets = matches[start:min(start+PageSize, len(matches))]
	}
	return result, nil
}

func (l *Library) matchImage(ctx context.Context, category, query string, hashes map[[32]byte]bool) ([]ImageMatch, error) {
	root, dir, err := l.open(category)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	items, err := assets(ctx, root, dir, category, query)
	if err != nil {
		return nil, err
	}
	var matches []ImageMatch
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path := filepath.Join(dir, item.Name)
		info, err := root.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > MaxBytes || assetID(category, info) != item.ID {
			continue
		}
		f, err := root.Open(path)
		if err != nil {
			return nil, err
		}
		current, statErr := f.Stat()
		if statErr != nil || !current.Mode().IsRegular() || assetID(category, current) != item.ID {
			f.Close()
			continue
		}
		data, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
		f.Close()
		if err != nil {
			return nil, err
		}
		if len(data) > MaxBytes {
			continue
		}
		matched := hashes[sha256.Sum256(data)]
		// Sending large images may re-encode them before QQ receives the bytes.
		if !matched && len(data) > 1<<20 {
			_, err = imaging.Run(ctx, func() (string, error) {
				matched = hashes[sha256.Sum256(services.ShrinkToJPEG(data, 1<<20, 85))]
				return "", nil
			})
			if err != nil {
				return nil, err
			}
		}
		if matched {
			matches = append(matches, ImageMatch{Category: category, Asset: item})
		}
	}
	return matches, nil
}
