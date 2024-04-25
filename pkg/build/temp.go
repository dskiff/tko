package build

import (
	"os"
)

func CreateTempFile(ctx BuildContext) (*os.File, error) {
	fp, err := os.CreateTemp(ctx.TempPath, "tko-temp-*.tar")
	if err != nil {
		return nil, err
	}
	ctx.ExitCleanupWatcher.Append(fp.Name())
	return fp, nil
}
