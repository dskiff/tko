package build

import (
	"archive/tar"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

var unixEpoch = time.Unix(0, 0)

func createLayerFromFolder(ctx BuildContext, layer BuildSpecInjectLayer, opts ...tarball.LayerOption) (v1.Layer, error) {
	tarPath, err := createTarFromFolder(ctx, layer.SourcePath, layer.DestinationPath)
	if err != nil {
		return nil, err
	}

	return tarball.LayerFromFile(tarPath, opts...)
}

func createTarFromFolder(ctx BuildContext, srcPath, dstPath string) (string, error) {
	tarFile, err := createTempFile(ctx)
	if err != nil {
		return "", err
	}
	defer tarFile.Close()

	writer := tar.NewWriter(tarFile)
	defer writer.Close()

	err = filepath.Walk(srcPath, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
			return fmt.Errorf("links are not supported: %s", fi.Name())
		}

		relPath, err := filepath.Rel(srcPath, file)
		if err != nil {
			return err
		}
		header.Name = filepath.Join(dstPath, relPath)

		header.AccessTime = unixEpoch
		header.ChangeTime = unixEpoch
		header.ModTime = unixEpoch
		header.Uid = 0
		header.Gid = 0
		header.Gname = "root"
		header.Uname = "root"
		header.PAXRecords = nil
		header.Xattrs = nil

		log.Println("adding file:", header.Name)

		// Write file header
		if err := writer.WriteHeader(header); err != nil {
			return err
		}

		// If not a directory, write file content
		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			defer data.Close()

			// Copy file data to tar writer
			if _, err := io.Copy(writer, data); err != nil {
				return err
			}
		}
		return nil
	})

	return tarFile.Name(), err
}
