package modeldownload

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"llamarig/platform/filedoc"
)

func prepareTarget(targetPath string, partPath string) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create model directory: %w", err)
	}
	if err := rejectSymlinkAncestors(dir); err != nil {
		return err
	}
	if err := rejectSymlink(targetPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := rejectSymlink(partPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func rejectSymlinkAncestors(path string) error {
	return wrapSymlinkError(filedoc.RejectSymlinkAncestors(path))
}

func rejectSymlink(path string) error {
	return wrapSymlinkError(filedoc.RejectSymlink(path))
}

func wrapSymlinkError(err error) error {
	if errors.Is(err, filedoc.ErrSymlink) {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	return err
}
