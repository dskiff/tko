package build

import (
	"os"
)

func CreateTempFile(cfg RunConfig) (*os.File, error) {
	fp, err := os.CreateTemp(cfg.TempPath, "tko-temp-*.tar")
	if err != nil {
		return nil, err
	}
	cfg.ExitCleanupWatcher.Append(fp.Name())
	return fp, nil
}
