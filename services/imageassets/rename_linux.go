package imageassets

import (
	"os"

	"golang.org/x/sys/unix"
)

func renameNoReplace(dir *os.File, oldname, newname string) error {
	return unix.Renameat2(int(dir.Fd()), oldname, int(dir.Fd()), newname, unix.RENAME_NOREPLACE)
}
