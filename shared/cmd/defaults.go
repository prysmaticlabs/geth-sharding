// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"path/filepath"
	"runtime"

	"github.com/prysmaticlabs/prysm/shared/fileutil"
)

// DefaultDataDir is the default data directory to use for the databases and other
// persistence requirements.
func DefaultDataDir() string {
	// Try to place the data folder in the user's home dir
	home := fileutil.HomeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "Eth2")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Local", "Eth2")
		} else {
			return filepath.Join(home, ".eth2")
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

// FixDefaultDataDir checks if previous data directory is found and can be migrated to a new path.
// This is used to resolve issue with weak default path (for Windows users) in existing installations.
// For full details see: https://github.com/prysmaticlabs/prysm/issues/5660.
func FixDefaultDataDir(prevDataDir, curDataDir string) error {
	if runtime.GOOS != "windows" {
		return nil
	}

	// See if shared directory is found (if it is -- we need to move it to non-shared destination).
	prevDataDirExists, err := fileutil.HasDir(prevDataDir)
	if err != nil {
		return err
	}
	if !prevDataDirExists {
		// If no previous "%APPDATA%\Eth2" found, nothing to patch and move to new default location.
		return nil
	}

	if curDataDir == "" {
		curDataDir = DefaultDataDir()
	}
	selectedDirExists, err := fileutil.HasDir(curDataDir)
	if err != nil {
		return err
	}
	if selectedDirExists {
		// No need not move anything, destination directory already exists.
		log.Warnf("Outdated data directory is found: %q! Current data folder %q is not empty, "+
			"so can not copy files automatically. Either remove outdated data directory, or "+
			"consider specifying non-existent new data directory (files will be copied automatically).",
			prevDataDir, curDataDir)
		return nil
	}

	if curDataDir == prevDataDir {
		return nil
	}

	log.Warnf("Outdated data directory is found: %q. Copying its contents to the new data folder: %q",
		prevDataDir, curDataDir)

	if err := fileutil.CopyDir(prevDataDir, curDataDir); err != nil {
		return err
	}

	log.Infof("All files from the outdated data directory %q has been moved to %q. Consider removing %q now.",
		prevDataDir, curDataDir, prevDataDir)
	return nil
}
