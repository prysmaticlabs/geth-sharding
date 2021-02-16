package fileutil

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/shared/params"
	log "github.com/sirupsen/logrus"
)

// ExpandPath given a string which may be a relative path.
// 1. replace tilde with users home dir
// 2. expands embedded environment variables
// 3. cleans the path, e.g. /a/b/../c -> /a/c
// Note, it has limitations, e.g. ~someuser/tmp will not be expanded
func ExpandPath(p string) (string, error) {
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		if home := HomeDir(); home != "" {
			p = home + p[1:]
		}
	}
	return filepath.Abs(path.Clean(os.ExpandEnv(p)))
}

// MkdirAll takes in a path, expands it if necessary, and looks through the
// permissions of every directory along the path, ensuring we are not attempting
// to overwrite any existing permissions. Finally, creates the directory accordingly
// with standardized, Prysm project permissions. This is the static-analysis enforced
// method for creating a directory programmatically in Prysm.
func MkdirAll(dirPath string) error {
	expanded, err := ExpandPath(dirPath)
	if err != nil {
		return err
	}
	exists, err := HasDir(expanded)
	if err != nil {
		return err
	}
	if exists {
		info, err := os.Stat(expanded)
		if err != nil {
			return err
		}
		if info.Mode().Perm() != params.BeaconIoConfig().ReadWriteExecutePermissions {
			return errors.New("dir already exists without proper 0700 permissions")
		}
	}
	return os.MkdirAll(expanded, params.BeaconIoConfig().ReadWriteExecutePermissions)
}

// WriteFile is the static-analysis enforced method for writing binary data to a file
// in Prysm, enforcing a single entrypoint with standardized permissions.
func WriteFile(file string, data []byte) error {
	expanded, err := ExpandPath(file)
	if err != nil {
		return err
	}
	if FileExists(expanded) {
		info, err := os.Stat(expanded)
		if err != nil {
			return err
		}
		if info.Mode() != params.BeaconIoConfig().ReadWritePermissions {
			return errors.New("file already exists without proper 0600 permissions")
		}
	}
	return ioutil.WriteFile(expanded, data, params.BeaconIoConfig().ReadWritePermissions)
}

// HomeDir for a user.
func HomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

// HasDir checks if a directory indeed exists at the specified path.
func HasDir(dirPath string) (bool, error) {
	fullPath, err := ExpandPath(dirPath)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if info == nil {
		return false, err
	}
	return info.IsDir(), err
}

// HasReadWritePermissions checks if file at a path has proper
// 0600 permissions set.
func HasReadWritePermissions(itemPath string) (bool, error) {
	info, err := os.Stat(itemPath)
	if err != nil {
		return false, err
	}
	return info.Mode() == params.BeaconIoConfig().ReadWritePermissions, nil
}

// FileExists returns true if a file is not a directory and exists
// at the specified path.
func FileExists(filename string) bool {
	filePath, err := ExpandPath(filename)
	if err != nil {
		return false
	}
	info, err := os.Stat(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.WithError(err).Info("Checking for file existence returned an error")
		}
		return false
	}
	return info != nil && !info.IsDir()
}

// ReadFileAsBytes expands a file name's absolute path and reads it as bytes from disk.
func ReadFileAsBytes(filename string) ([]byte, error) {
	filePath, err := ExpandPath(filename)
	if err != nil {
		return nil, errors.Wrap(err, "could not determine absolute path of password file")
	}
	return ioutil.ReadFile(filePath)
}

// CopyFile copy a file from source to destination path.
func CopyFile(src, dst string) error {
	input, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dst, input, params.BeaconIoConfig().ReadWritePermissions)
	if err != nil {
		err := errors.Wrapf(err, "error creating file %s", dst)
		return err
	}
	return nil
}

// CopyDir copies contents of one directory into another, recursively.
func CopyDir(src, dst string) error {
	dstExists, err := HasDir(dst)
	if err != nil {
		return err
	}
	if dstExists {
		return errors.New("destination directory already exists")
	}
	fds, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}
	if err := MkdirAll(dst); err != nil {
		return errors.Wrapf(err, "error creating directory: %s", dst)
	}
	for _, fd := range fds {
		srcPath := path.Join(src, fd.Name())
		dstPath := path.Join(dst, fd.Name())
		if fd.IsDir() {
			if err = CopyDir(srcPath, dstPath); err != nil {
				return errors.Wrapf(err, "error copying directory %s -> %s", srcPath, dstPath)
			}
		} else {
			if err = CopyFile(srcPath, dstPath); err != nil {
				return errors.Wrapf(err, "error copying file %s -> %s", srcPath, dstPath)
			}
		}
	}
	return nil
}

// DirsEqual checks whether two directories have the same content.
func DirsEqual(src, dst string) bool {
	hash1, err := HashDir(src)
	if err != nil {
		return false
	}

	hash2, err := HashDir(dst)
	if err != nil {
		return false
	}

	return hash1 == hash2
}

// HashDir calculates and returns hash of directory: each file's hash is calculated and saved along
// with the file name into the list, after which list is hashed to produce the final signature.
// Implementation is based on https://github.com/golang/mod/blob/release-branch.go1.15/sumdb/dirhash/hash.go
func HashDir(dir string) (string, error) {
	files, err := DirFiles(dir)
	if err != nil {
		return "", err
	}

	h := sha256.New()
	files = append([]string(nil), files...)
	sort.Strings(files)
	for _, file := range files {
		fd, err := os.Open(filepath.Join(dir, file))
		if err != nil {
			return "", err
		}
		hf := sha256.New()
		_, err = io.Copy(hf, fd)
		if err != nil {
			return "", err
		}
		if err := fd.Close(); err != nil {
			return "", err
		}
		if _, err := fmt.Fprintf(h, "%x  %s\n", hf.Sum(nil), file); err != nil {
			return "", err
		}
	}
	return "hashdir:" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// DirFiles returns list of files found within a given directory and its sub-directories.
// Directory prefix will not be included as a part of returned file string i.e. for a file located
// in "dir/foo/bar" only "foo/bar" part will be returned.
func DirFiles(dir string) ([]string, error) {
	var files []string
	dir = filepath.Clean(dir)
	err := filepath.Walk(dir, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relFile := file
		if dir != "." {
			relFile = file[len(dir)+1:]
		}
		files = append(files, filepath.ToSlash(relFile))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
