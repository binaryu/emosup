package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type LogResult struct {
	Kind      string   `json:"kind"`
	Path      string   `json:"path"`
	Lines     []string `json:"lines"`
	Truncated bool     `json:"truncated"`
}

type LogService struct{}

func NewLogService() *LogService {
	return &LogService{}
}

// Tail returns the last `lines` lines of the requested log source.
//
// Kinds:
//   - "service":  systemd journal for the unit running this process
//   - "upgrade":  the in-panel upgrade/restart log (/tmp/emosup-upgrade.log)
func (s *LogService) Tail(ctx context.Context, kind string, lines int) (LogResult, error) {
	if lines <= 0 || lines > 5000 {
		lines = 200
	}

	switch kind {
	case "service":
		return s.tailServiceLog(ctx, lines)
	case "upgrade":
		path := upgradeLogPath()
		content, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return LogResult{}, fmt.Errorf("暂无升级日志（%s 不存在，尚未执行过面板升级）", path)
			}
			return LogResult{}, err
		}
		all := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
		return LogResult{Kind: kind, Path: path, Lines: tailLines(all, lines), Truncated: len(all) > lines}, nil
	default:
		return LogResult{}, fmt.Errorf("未知日志类型: %s", kind)
	}
}

func (s *LogService) tailServiceLog(ctx context.Context, lines int) (LogResult, error) {
	unit := systemdUnit()
	if unit == "" {
		return LogResult{}, fmt.Errorf("未检测到 systemd 服务单元（手动运行请查看启动终端的输出，或容器内执行 docker logs）")
	}
	if _, err := exec.LookPath("journalctl"); err != nil {
		return LogResult{}, fmt.Errorf("系统缺少 journalctl 命令")
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "journalctl", "-u", unit, "-n", strconv.Itoa(lines), "--no-pager", "-o", "short-iso")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// journalctl exits non-zero when the unit has no entries at all.
		return LogResult{}, fmt.Errorf("读取 journal 失败（unit=%s）: %v", unit, err)
	}
	all := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	return LogResult{Kind: "service", Path: unit, Lines: tailLines(all, lines), Truncated: len(all) > lines}, nil
}

func tailLines(lines []string, n int) []string {
	if len(lines) <= n {
		return lines
	}
	return lines[len(lines)-n:]
}
