package security

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func EnsureSecureFilePermissions(path string, expectedMode os.FileMode) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat file: %w", err)
	}

	currentMode := info.Mode().Perm()
	if currentMode != expectedMode {
		log.Printf("WARNING: %s has insecure permissions %o, fixing to %o", path, currentMode, expectedMode)
		if err := os.Chmod(path, expectedMode); err != nil {
			return fmt.Errorf("chmod file: %w", err)
		}
		log.Printf("Successfully updated permissions for %s", path)
	}

	return nil
}

func EnsureSecureDirectoryPermissions(dirPath string, expectedMode os.FileMode) error {
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dirPath)
	}

	currentMode := info.Mode().Perm()
	if currentMode != expectedMode {
		log.Printf("WARNING: Directory %s has insecure permissions %o, fixing to %o", dirPath, currentMode, expectedMode)
		if err := os.Chmod(dirPath, expectedMode); err != nil {
			return fmt.Errorf("chmod directory: %w", err)
		}
		log.Printf("Successfully updated permissions for directory %s", dirPath)
	}

	return nil
}

func CheckSecretFilesPermissions(basePath string) error {
	files := []struct {
		path         string
		expectedMode os.FileMode
	}{
		{filepath.Join(basePath, "tokens.db"), 0o600},
		{filepath.Join(basePath, "tokens.db-wal"), 0o600},
		{filepath.Join(basePath, "tokens.db-shm"), 0o600},
	}

	for _, f := range files {
		if err := EnsureSecureFilePermissions(f.path, f.expectedMode); err != nil {
			return fmt.Errorf("check %s: %w", f.path, err)
		}
	}

	if err := EnsureSecureDirectoryPermissions(basePath, 0o700); err != nil {
		return fmt.Errorf("check directory %s: %w", basePath, err)
	}

	return nil
}
