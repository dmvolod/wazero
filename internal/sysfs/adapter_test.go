package sysfs

import (
	"errors"
	"io/fs"
	"os"
	pathutil "path"
	"syscall"
	"testing"

	"github.com/tetratelabs/wazero/internal/fstest"
	"github.com/tetratelabs/wazero/internal/testing/require"
)

func TestAdapt_String(t *testing.T) {
	testFS := Adapt(os.DirFS("."))
	require.Equal(t, ".", testFS.String())
}

func TestAdapt_MkDir(t *testing.T) {
	testFS := Adapt(os.DirFS(t.TempDir()))

	err := testFS.Mkdir("mkdir", fs.ModeDir)
	require.Equal(t, syscall.ENOSYS, err)
}

func TestAdapt_Chmod(t *testing.T) {
	testFS := Adapt(os.DirFS(t.TempDir()))

	err := testFS.Chmod("chmod", fs.ModeDir)
	require.Equal(t, syscall.ENOSYS, err)
}

func TestAdapt_Rename(t *testing.T) {
	tmpDir := t.TempDir()
	testFS := Adapt(os.DirFS(tmpDir))

	file1 := "file1"
	file1Path := pathutil.Join(tmpDir, file1)
	file1Contents := []byte{1}
	err := os.WriteFile(file1Path, file1Contents, 0o600)
	require.NoError(t, err)

	file2 := "file2"
	file2Path := pathutil.Join(tmpDir, file2)
	file2Contents := []byte{2}
	err = os.WriteFile(file2Path, file2Contents, 0o600)
	require.NoError(t, err)

	err = testFS.Rename(file1, file2)
	require.Equal(t, syscall.ENOSYS, err)
}

func TestAdapt_Rmdir(t *testing.T) {
	tmpDir := t.TempDir()
	testFS := Adapt(os.DirFS(tmpDir))

	path := "rmdir"
	realPath := pathutil.Join(tmpDir, path)
	require.NoError(t, os.Mkdir(realPath, 0o700))

	err := testFS.Rmdir(path)
	require.Equal(t, syscall.ENOSYS, err)
}

func TestAdapt_Unlink(t *testing.T) {
	tmpDir := t.TempDir()
	testFS := Adapt(os.DirFS(tmpDir))

	path := "unlink"
	realPath := pathutil.Join(tmpDir, path)
	require.NoError(t, os.WriteFile(realPath, []byte{}, 0o600))

	err := testFS.Unlink(path)
	require.Equal(t, syscall.ENOSYS, err)
}

func TestAdapt_Utimes(t *testing.T) {
	tmpDir := t.TempDir()
	testFS := Adapt(os.DirFS(tmpDir))

	path := "utimes"
	realPath := pathutil.Join(tmpDir, path)
	require.NoError(t, os.WriteFile(realPath, []byte{}, 0o600))

	err := testFS.Utimes(path, 1, 1)
	require.Equal(t, syscall.ENOSYS, err)
}

func TestAdapt_Open_Read(t *testing.T) {
	// Create a subdirectory, so we can test reads outside the FS root.
	tmpDir := t.TempDir()
	tmpDir = pathutil.Join(tmpDir, t.Name())
	require.NoError(t, os.Mkdir(tmpDir, 0o700))
	testFS := Adapt(os.DirFS(tmpDir))

	testOpen_Read(t, tmpDir, testFS)

	t.Run("path outside root invalid", func(t *testing.T) {
		_, err := testFS.OpenFile("../foo", os.O_RDONLY, 0)

		// fs.FS doesn't allow relative path lookups
		require.Equal(t, syscall.EINVAL, err)
	})
}

// hackFS cheats the fs.FS contract by opening for write (os.O_RDWR).
//
// Until we have an alternate public interface for filesystems, some users will
// rely on this. Via testing, we ensure we don't accidentally break them.
type hackFS string

func (dir hackFS) Open(name string) (fs.File, error) {
	path := ensureTrailingPathSeparator(string(dir)) + name

	if f, err := os.OpenFile(path, os.O_RDWR, 0); err == nil {
		return f, nil
	} else if errors.Is(err, syscall.EISDIR) {
		return os.OpenFile(path, os.O_RDONLY, 0)
	} else if errors.Is(err, syscall.ENOENT) {
		return os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0o444)
	} else {
		return nil, err
	}
}

// TestAdapt_HackedWrites ensures we allow writes even if they violate the
// fs.FS contract.
func TestAdapt_HackedWrites(t *testing.T) {
	tmpDir := t.TempDir()
	testFS := Adapt(hackFS(tmpDir))

	testOpen_O_RDWR(t, tmpDir, testFS)
}

func TestAdapt_TestFS(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	require.NoError(t, fstest.WriteTestFiles(tmpDir))
	dirFS := os.DirFS(tmpDir)

	// TODO: We can't currently test embed.FS here because the source of
	// fstest.FS are not real files.
	tests := []struct {
		name string
		fs   fs.FS
	}{
		{name: "os.DirFS", fs: dirFS},
		{name: "fstest.MapFS", fs: fstest.FS},
	}

	for _, tc := range tests {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			// Adapt a normal fs.FS to sysfs.FS
			testFS := Adapt(tc.fs)

			// Adapt it back to fs.FS and run the tests
			require.NoError(t, fstest.TestFS(testFS.(fs.FS)))
		})
	}
}
