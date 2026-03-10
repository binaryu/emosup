package service

import "os"

func SafeRemove(path string) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	return os.Remove(path)
}
