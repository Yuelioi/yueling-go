package llm

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"strings"
	"sync"

	"github.com/Yuelioi/yueling-go/services/imaging"
)

const MaxAIEditFrames = 12

func (o ImageOptions) Validate() error {
	if len(o.AnimationFrames) == 0 {
		if o.FrameDurationMS != 0 {
			return &Error{Kind: Configuration, Detail: "animation_plan"}
		}
		return nil
	}
	if len(o.AnimationFrames) < 2 || len(o.AnimationFrames) > MaxAIEditFrames || o.FrameDurationMS != 0 && (o.FrameDurationMS < 20 || o.FrameDurationMS > 5000) {
		return &Error{Kind: Configuration, Detail: "animation_plan"}
	}
	for _, frame := range o.AnimationFrames {
		if strings.TrimSpace(frame) == "" || len(frame) > 2000 {
			return &Error{Kind: Configuration, Detail: "animation_plan"}
		}
	}
	return nil
}

func (c *Client) createAnimation(ctx context.Context, prompt string, references [][]byte, opts ImageOptions) ([]byte, error) {
	duration := opts.FrameDurationMS
	if duration == 0 {
		duration = 400
	}
	result := &imaging.Animation{LoopCount: 0}
	var bounds image.Rectangle
	for i, description := range opts.AnimationFrames {
		if err := ctx.Err(); err != nil {
			return nil, classify(err)
		}
		framePrompt := fmt.Sprintf("%s\n制作同一GIF的第%d/%d帧。此帧动作：%s。固定角色长相、画风、构图、镜头与背景，仅改变动作；只输出这一帧，不画分镜拼图。", prompt, i+1, len(opts.AnimationFrames), description)
		data, err := c.imageSingle(ctx, framePrompt, references, ImageOptions{TransparentBackground: opts.TransparentBackground})
		if err != nil {
			return nil, err
		}
		frame, err := imaging.Run(ctx, func() (*image.NRGBA, error) {
			decoded, err := imaging.Decode(data, imaging.DefaultLimits)
			if err != nil || decoded.Animated() {
				return nil, &Error{Kind: InvalidResponse, Detail: "invalid_image"}
			}
			if bounds.Empty() {
				b := decoded.Frames[0].Bounds()
				width, height := b.Dx(), b.Dy()
				if longest := max(width, height); longest > 512 {
					width, height = max(1, width*512/longest), max(1, height*512/longest)
				}
				bounds = image.Rect(0, 0, width, height)
			}
			return imaging.ResizeImage(decoded.Frames[0], bounds.Dx(), bounds.Dy()), nil
		})
		if err != nil {
			return nil, err
		}
		result.Frames = append(result.Frames, frame)
		result.Delays = append(result.Delays, max(2, (duration+5)/10))
		// A text-only plan uses its first completed frame as the identity reference.
		if i == 0 && len(references) == 0 {
			references = [][]byte{bytes.Clone(data)}
		}
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
