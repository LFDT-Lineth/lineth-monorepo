package files

import (
	"compress/gzip"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	DIR string = "/tmp/go-file-test"
)

func TestCheckFilePath(t *testing.T) {
	// Create a temporary directory and file for testing.
	tmpDir, err := os.MkdirTemp("", "testdir")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("cleanup failed: %v", err)
		}
	}()

	tmpFile, err := os.CreateTemp(tmpDir, "testfile")
	if err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	t.Run("success existing file", func(t *testing.T) {
		if gotErr := CheckFilePath(tmpFile.Name()); gotErr != nil {
			t.Errorf("unexpected error: %q", gotErr)
		}
	})

	t.Run("failure", func(t *testing.T) {
		tests := []struct {
			desc     string
			filePath string
			wantErr  error
		}{
			{
				desc:     "non-existing file",
				filePath: "/path/to/nonexistent/file",
				wantErr:  os.ErrNotExist,
			},
			{
				desc:     "directory instead of file",
				filePath: tmpDir,
				wantErr:  fmt.Errorf("%q is not a file", tmpDir),
			},
		}

		for i, tt := range tests {
			t.Run(tt.desc, func(t *testing.T) {
				gotErr := CheckFilePath(tt.filePath)
				if !errors.Is(gotErr, tt.wantErr) && gotErr.Error() != tt.wantErr.Error() {
					t.Errorf("test case %d:\nexpected error: %q\ngot: %q", i, tt.wantErr, gotErr)
				}
			})
		}
	})
}

func TestCheckDirPath(t *testing.T) {
	// Create a temporary directory and file for testing.
	tmpDir, err := os.MkdirTemp("", "testdir")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("cleanup failed: %v", err)
		}
	}()

	tmpFile, err := os.CreateTemp(tmpDir, "testfile")
	if err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	t.Run("success existing dir", func(t *testing.T) {
		if gotErr := CheckDirPath(tmpDir); gotErr != nil {
			t.Errorf("unexpected error: %q", gotErr)
		}
	})
	t.Run("failure", func(t *testing.T) {
		tests := []struct {
			desc    string
			dirPath string
			wantErr error
		}{
			{
				desc:    "non-existing dir",
				dirPath: "/path/to/nonexistentdir",
				wantErr: os.ErrNotExist,
			},
			{
				desc:    "file instead of directory",
				dirPath: tmpFile.Name(),
				wantErr: fmt.Errorf("%q is not a directory", tmpFile.Name()),
			},
		}

		for i, tt := range tests {
			t.Run(tt.desc, func(t *testing.T) {
				gotErr := CheckDirPath(tt.dirPath)
				if !errors.Is(gotErr, tt.wantErr) && gotErr.Error() != tt.wantErr.Error() {
					t.Errorf("test case %d:\nexpected error: %q\ngot: %q", i, tt.wantErr, gotErr)
				}
			})
		}
	})
}

func TestFilesCompressed(t *testing.T) {

	// Write random bytes
	fileSize := 1 << 12
	b := make([]byte, fileSize)
	_, err := rand.Reader.Read(b)
	require.NoError(t, err)

	path := path.Join(DIR, "rand")

	w := mustWriteCompressed(path)
	_, err = w.Write(b)
	require.NoError(t, err)

	require.NoError(t, w.Close())

	r := MustReadCompressed(path)
	newB := make([]byte, fileSize)
	_, err = r.Read(newB)
	require.NoError(t, err)

	assert.Equal(t, b, newB)

}

func TestFilesRW(t *testing.T) {

	// Write random bytes
	fileSize := 1 << 12
	b := make([]byte, fileSize)
	_, err := rand.Reader.Read(b)
	require.NoError(t, err)

	path := path.Join(DIR, "rand")

	w := MustOverwrite(path)
	_, err = w.Write(b)
	require.NoError(t, err)

	require.NoError(t, w.Close())

	r := MustRead(path)
	newB := make([]byte, fileSize)
	_, err = r.Read(newB)
	require.NoError(t, err)

	assert.Equal(t, b, newB)
}

// Write into a .gz file or panic
func mustWriteCompressed(p string) *ZipFile {
	f := MustOverwrite(p)
	return &ZipFile{
		f:      f,
		Writer: gzip.NewWriter(f),
	}
}
