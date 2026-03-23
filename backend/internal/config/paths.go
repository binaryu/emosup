package config

import "path/filepath"

type DataPaths struct {
	Root      string
	Config    string
	ScansDir  string
	TasksDir  string
	LogsDir   string
	Downloads string
}

func NewDataPaths(root string) DataPaths {
	return DataPaths{
		Root:      root,
		Config:    filepath.Join(root, "config.json"),
		ScansDir:  filepath.Join(root, "scans"),
		TasksDir:  filepath.Join(root, "tasks"),
		LogsDir:   filepath.Join(root, "logs"),
		Downloads: filepath.Join(root, "downloads"),
	}
}
