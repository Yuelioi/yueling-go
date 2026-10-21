package llm

import (
	"context"
	"fmt"
	"image"
	"sync"

	"github.com/Yuelioi/yueling-go/services/imaging"
)

const MaxAIEditFrames = 12

func gifEditLimits() imaging.Limits {
	limits := imaging.DefaultLimits
	limits.MaxFrames = MaxAIEditFrames
	limits.MaxTotalPixels = 12_000_000
	return limits
}

func (c *Client) editAnimation(ctx context.Context, prompt string, source *imaging.Animation, references [][]byte, animationIndex int, opts ImageOptions) ([]byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	bounds := source.Frames[0].Bounds()
	result := &imaging.Animation{Frames: make([]*image.NRGBA, len(source.Frames)), Delays: source.Delays, LoopCount: source.LoopCount}
	var mu sync.Mutex
	var firstErr error
	next := 0
	var workers sync.WaitGroup
	concurrency := min(3, len(source.Frames))
	// Codex serializes each account, so queued frames would spend their
	// request timeouts waiting for earlier frames rather than generating.
	if c.settings.Kind == "codex" {
		concurrency = 1
	}
	for range concurrency {
		workers.Go(func() {
			for {
				mu.Lock()
				if firstErr != nil || ctx.Err() != nil || next == len(source.Frames) {
					mu.Unlock()
					return
				}
				i := next
				next++
				mu.Unlock()
				data, err := imaging.Run(ctx, func() ([]byte, error) { return imaging.EncodePNG(source.Frames[i], gifEditLimits()) })
				if err == nil {
					framePrompt := fmt.Sprintf("%s\n附图中的第%d张是同一GIF的第%d/%d帧，其他图片为固定参考。只编辑此帧，保持原始动作、位置、构图和统一风格；不要添加拼图、边框或其他帧。", prompt, animationIndex+1, i+1, len(source.Frames))
					frameReferences := append([][]byte(nil), references...)
					frameReferences[animationIndex] = data
					data, err = c.imageSingle(ctx, framePrompt, frameReferences, opts)
				}
				if err == nil {
					result.Frames[i], err = imaging.Run(ctx, func() (*image.NRGBA, error) {
						frame, err := imaging.Decode(data, imaging.DefaultLimits)
						if err != nil || frame.Animated() {
							return nil, &Error{Kind: InvalidResponse, Detail: "invalid_image"}
						}
						return imaging.ResizeImage(frame.Frames[0], bounds.Dx(), bounds.Dy()), nil
					})
				}
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
						cancel()
					}
					mu.Unlock()
					return
				}
			}
		})
	}
	workers.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	if ctx.Err() != nil {
		return nil, classify(ctx.Err())
	}
	data, err := imaging.Run(ctx, func() ([]byte, error) { return imaging.Encode(result, gifEditLimits()) })
	if err != nil {
		if ctx.Err() != nil {
			return nil, classify(ctx.Err())
		}
		return nil, &Error{Kind: InvalidResponse, Detail: "gif_encode_failed"}
	}
	return data, nil
}
