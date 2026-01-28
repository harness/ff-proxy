package files

import "os"

// Exists validates that a file exists at the given path.
func Exists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return err
	}
	return nil
}

// Readable validates that a file is readable at the given path.
func Readable(path string) error {
	if _, err := os.ReadFile(path); err != nil {
		return err
	}
	return nil
}
