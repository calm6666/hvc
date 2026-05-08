package hoststats

import (
	"bytes"
	"os/exec"
	"strings"
)

func commandOutput(name string, args ...string) []byte {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil
	}
	cmd := exec.Command(path, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	return bytes.TrimSpace(output)
}

func splitNonEmptyLines(output []byte) []string {
	if len(output) == 0 {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	items := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		items = append(items, line)
	}
	return items
}
