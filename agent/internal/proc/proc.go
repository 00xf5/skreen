package proc

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type Process struct {
	Name   string `json:"name"`
	PID    int    `json:"pid"`
	Memory int64  `json:"memory_kb"`
}

// List returns running processes using tasklist (Windows).
func List() ([]Process, error) {
	cmd := exec.Command("tasklist", "/FO", "CSV", "/NH")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var procs []Process
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		// CSV format: "name","pid","session","num","memory"
		parts := strings.Split(line, "\",\"")
		if len(parts) < 5 {
			continue
		}
		name := strings.Trim(parts[0], "\"")
		pid, _ := strconv.Atoi(strings.Trim(parts[1], "\""))
		memStr := strings.Trim(parts[4], "\"")
		memStr = strings.ReplaceAll(memStr, ",", "")
		memStr = strings.ReplaceAll(memStr, " K", "")
		memStr = strings.TrimSpace(memStr)
		mem, _ := strconv.ParseInt(memStr, 10, 64)
		procs = append(procs, Process{Name: name, PID: pid, Memory: mem})
	}
	return procs, nil
}

// Kill terminates a process by PID.
func Kill(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid))
	return cmd.Run()
}
