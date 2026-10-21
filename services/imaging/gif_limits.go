package imaging

import (
	"encoding/binary"
	"errors"
)

// Count frames before DecodeAll allocates their pixel buffers.
func checkGIFFrames(data []byte, limits Limits) error {
	invalid := errors.New("GIF数据不完整")
	if len(data) < 13 {
		return invalid
	}
	width, height := int(binary.LittleEndian.Uint16(data[6:8])), int(binary.LittleEndian.Uint16(data[8:10]))
	pos := 13
	if data[10]&0x80 != 0 {
		pos += 3 * (1 << ((data[10] & 7) + 1))
	}
	skipBlocks := func() bool {
		for pos < len(data) {
			size := int(data[pos])
			pos++
			if size == 0 {
				return true
			}
			pos += size
		}
		return false
	}
	frames := 0
	for pos < len(data) {
		block := data[pos]
		pos++
		switch block {
		case 0x3b:
			if frames == 0 {
				return invalid
			}
			return nil
		case 0x21:
			if pos >= len(data) {
				return invalid
			}
			pos++
			if !skipBlocks() {
				return invalid
			}
		case 0x2c:
			if pos+9 > len(data) {
				return invalid
			}
			frames++
			if err := validateGeometry(width, height, frames, limits); err != nil {
				return err
			}
			packed := data[pos+8]
			pos += 9
			if packed&0x80 != 0 {
				pos += 3 * (1 << ((packed & 7) + 1))
			}
			if pos >= len(data) {
				return invalid
			}
			pos++
			if !skipBlocks() {
				return invalid
			}
		default:
			return invalid
		}
	}
	return invalid
}
