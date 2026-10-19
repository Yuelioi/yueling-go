//go:build !linux && !darwin

package imageassets

import (
	"errors"
	"os"
)

func renameNoReplace(_ *os.File, _, _ string) error {
	return errors.New("当前平台不支持安全的原子重命名，未修改原图")
}
