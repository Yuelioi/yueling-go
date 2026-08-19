package imaging

import (
	"errors"
	"image"
)

// FrameAt samples a complete frame at the given time in hundredths of a second.
// Animated inputs loop; static inputs always return their first frame.
func (a *Animation) FrameAt(ticks int) *image.NRGBA {
	if a == nil || len(a.Frames) == 0 {
		return nil
	}
	if len(a.Frames) == 1 {
		return a.Frames[0]
	}
	total := 0
	for i := range a.Frames {
		total += a.delayAt(i)
	}
	if total <= 0 {
		return a.Frames[0]
	}
	ticks %= total
	if ticks < 0 {
		ticks += total
	}
	for i, frame := range a.Frames {
		delay := a.delayAt(i)
		if ticks < delay {
			return frame
		}
		ticks -= delay
	}
	return a.Frames[len(a.Frames)-1]
}

func (a *Animation) delayAt(index int) int {
	if index < len(a.Delays) && a.Delays[index] > 0 {
		return max(a.Delays[index], minimumFrameDelay)
	}
	return defaultFrameDelay
}

// RenderTimeline renders a fixed template animation while sampling every input
// animation on the same clock. It is the common shape for frame-and-coordinate
// avatar memes such as petpet-like and two-avatar templates.
func RenderTimeline(
	frameCount int,
	delay int,
	inputs []*Animation,
	render func(frameIndex int, sampled []*image.NRGBA) (image.Image, error),
) (*Animation, error) {
	if frameCount <= 0 || render == nil {
		return nil, errors.New("模板时间轴无效")
	}
	delay = max(delay, minimumFrameDelay)
	frames := make([]*image.NRGBA, frameCount)
	delays := make([]int, frameCount)
	var bounds image.Rectangle
	for i := range frameCount {
		sampled := make([]*image.NRGBA, len(inputs))
		for j, input := range inputs {
			sampled[j] = input.FrameAt(i * delay)
			if sampled[j] == nil {
				return nil, errors.New("模板输入图片为空")
			}
		}
		frame, err := render(i, sampled)
		if err != nil {
			return nil, err
		}
		if frame == nil {
			return nil, errors.New("模板返回了空帧")
		}
		frames[i] = ToNRGBA(frame)
		if i == 0 {
			bounds = frames[i].Bounds()
		} else if frames[i].Bounds() != bounds {
			return nil, errors.New("模板输出帧尺寸不一致")
		}
		delays[i] = delay
	}
	return &Animation{Frames: frames, Delays: delays, LoopCount: 0}, nil
}
