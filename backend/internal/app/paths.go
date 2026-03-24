package app

import (
	"os"
	"path/filepath"
	"strings"
)

func resolveDataRoot() string {
	if override := strings.TrimSpace(os.Getenv("EMOSUP_DATA_DIR")); override != "" {
		return absOrClean(override)
	}

	cwd, _ := os.Getwd()
	exeDir := executableDir()

	for _, candidate := range []string{
		filepath.Join(cwd, "backend", "data"),
		filepath.Join(cwd, "data"),
		filepath.Join(exeDir, "data"),
		filepath.Join(exeDir, "backend", "data"),
	} {
		if dirExists(candidate) {
			return absOrClean(candidate)
		}
	}

	switch {
	case looksLikeRepoRoot(cwd):
		return absOrClean(filepath.Join(cwd, "backend", "data"))
	case looksLikeBackendRoot(cwd):
		return absOrClean(filepath.Join(cwd, "data"))
	case looksLikeRepoRoot(exeDir):
		return absOrClean(filepath.Join(exeDir, "backend", "data"))
	case looksLikeBackendRoot(exeDir):
		return absOrClean(filepath.Join(exeDir, "data"))
	case exeDir != "":
		return absOrClean(filepath.Join(exeDir, "data"))
	default:
		return absOrClean(filepath.Join(".", "data"))
	}
}

func findFrontendDistDir() string {
	cwd, _ := os.Getwd()
	exeDir := executableDir()

	candidates := []string{
		strings.TrimSpace(os.Getenv("EMOSUP_FRONTEND_DIST")),
		filepath.Join(cwd, "frontend", "dist"),
		filepath.Join(cwd, "frontend"),
		filepath.Join(cwd, "..", "frontend", "dist"),
		filepath.Join(cwd, "..", "frontend"),
		filepath.Join(exeDir, "frontend", "dist"),
		filepath.Join(exeDir, "frontend"),
		filepath.Join(exeDir, "..", "frontend", "dist"),
		filepath.Join(exeDir, "..", "frontend"),
		filepath.Join("..", "frontend", "dist"),
		filepath.Join("frontend", "dist"),
		filepath.Join("..", "frontend"),
		filepath.Join("frontend"),
	}

	for _, candidate := range candidates {
		if isFrontendBuild(candidate) {
			return absOrClean(candidate)
		}
	}

	return ""
}

func executableDir() string {
	executablePath, err := os.Executable()
	if err != nil {
		return ""
	}

	return filepath.Dir(executablePath)
}

func looksLikeRepoRoot(dir string) bool {
	return fileExists(filepath.Join(dir, "backend", "go.mod")) && fileExists(filepath.Join(dir, "frontend", "package.json"))
}

func looksLikeBackendRoot(dir string) bool {
	return fileExists(filepath.Join(dir, "go.mod")) && dirExists(filepath.Join(dir, "cmd"))
}

func isFrontendBuild(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}

	indexInfo, indexErr := os.Stat(filepath.Join(dir, "index.html"))
	assetsInfo, assetsErr := os.Stat(filepath.Join(dir, "assets"))
	return indexErr == nil && assetsErr == nil && !indexInfo.IsDir() && assetsInfo.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func absOrClean(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}

	absPath, err := filepath.Abs(path)
	if err == nil {
		return absPath
	}

	return filepath.Clean(path)
}
