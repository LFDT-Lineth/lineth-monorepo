package files

import (
	"compress/gzip"
	"fmt"
	"os"
	"path"

	"github.com/sirupsen/logrus"
)

// CheckFilePath checks whether the provided filePath points to an existing file.
func CheckFilePath(filePath string) error {
	// Use os.Stat to get information about the file
	// #nosec G703 -- path traversal are not a concern as this is not meant to be run in a browser.
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		// this error is either a os.ErrNotExist or something else if the Stat function failed.
		return err
	}

	// Check if it's a regular file (not a directory)
	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("%q is not a file", filePath)
	}

	return nil
}

// CheckDirPath checks whether the provided dirPath points to an existing directory.
func CheckDirPath(dirPath string) error {
	// Use os.Stat to get information about the directory
	fileInfo, err := os.Stat(dirPath)
	if err != nil {
		// this error is either a os.ErrNotExist or something else if the Stat function failed.
		return err
	}

	// Check if it's a directory
	if !fileInfo.IsDir() {
		return fmt.Errorf("%q is not a directory", dirPath)
	}

	return nil
}

// TODO @gbotrel most of this "MustXXX" functions must go away and be replaced by proper error handling

// opens an existing file with read access
func MustRead(path string) *os.File {
	f, err := os.Open(path)
	if err != nil {
		logrus.Panicf("Error opening %v - %v", path, err)
	}

	return f
}

// create a file and the parent's folder if necessary. Overwrite the file
// if it already exists.
func MustOverwrite(p string) *os.File {

	/*
		Ensures the parent directory exists
	*/
	dir := path.Dir(p)
	err := os.MkdirAll(dir, 0770)
	if err != nil {
		logrus.Panicf("Could not create directory - %v, error: %v, filepath: %v", dir, err, p)
	}

	/*
		Create or overwrite a file
	*/
	f, err := os.Create(p)
	if err != nil {
		logrus.Panicf("Could not create directory - %v, error: %v, filepath: %v", dir, err, p)
	}

	return f
}

// Wrapper for os.File allowing to close cleanly
type ZipFile struct {
	f *os.File
	*gzip.Writer
	*gzip.Reader
}

// Read a .gz archive or panic
func MustReadCompressed(path string) *ZipFile {
	f := MustRead(path)
	unzipped, err := gzip.NewReader(f)
	if err != nil {
		logrus.Panicf("Error decompressing %v - %v", path, err)
	}

	return &ZipFile{
		f:      f,
		Reader: unzipped,
	}
}

// Close unzipped file
func (z *ZipFile) Close() error {
	// Close the writer if any
	if z.Writer != nil {
		// For some reason, forgetting to call `Flush` here causes
		// `EOF`` errors.
		if err := z.Writer.Flush(); err != nil {
			return err
		}

		if err := z.Writer.Close(); err != nil {
			return err
		}
	}

	// Close the reader if any
	if z.Reader != nil {
		if err := z.Reader.Close(); err != nil {
			return err
		}
	}

	// Close the file
	if err := z.f.Close(); err != nil {
		return err
	}

	return nil
}
