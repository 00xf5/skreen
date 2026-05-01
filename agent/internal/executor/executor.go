package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Result holds command execution results
type Result struct {
	Output   string
	Error    string
	ExitCode int
	Duration time.Duration
}

// Executor handles secure command execution
type Executor struct {
	timeout      time.Duration
	maxOutput    int // Maximum output size in bytes
	blockedCmds  []string
}

// New creates a new command executor
func New(timeout time.Duration) *Executor {
	return &Executor{
		timeout:   timeout,
		maxOutput: 1024 * 1024, // 1MB max output
		blockedCmds: []string{
			"rm -rf /",
			"mkfs",
			"dd if=/dev/zero",
			":(){ :|:& };:", // Fork bomb
		},
	}
}

// Execute runs a command safely and returns the result
func (e *Executor) Execute(command string) *Result {
	start := time.Now()

	// Validate command
	if err := e.validate(command); err != nil {
		return &Result{
			Error:    err.Error(),
			ExitCode: -1,
			Duration: time.Since(start),
		}
	}

	// Prepare command based on OS
	var cmd *exec.Cmd
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	switch runtime.GOOS {
	case "windows":
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	default:
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command)
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err := cmd.Run()
	duration := time.Since(start)

	result := &Result{
		Duration: duration,
	}

	// Capture exit code
	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	} else if err != nil && ctx.Err() == context.DeadlineExceeded {
		result.ExitCode = -1
		result.Error = fmt.Sprintf("command timed out after %v", e.timeout)
	} else if err != nil {
		result.ExitCode = -1
		result.Error = err.Error()
	}

	// Capture output (truncated if too large)
	out := stdout.String()
	if len(out) > e.maxOutput {
		out = out[:e.maxOutput] + "\n[truncated]"
	}
	result.Output = out

	// Capture stderr
	errStr := stderr.String()
	if errStr != "" && result.Error == "" {
		result.Error = errStr
	}

	return result
}

// validate checks if a command is safe to execute
func (e *Executor) validate(command string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("empty command")
	}

	// Check against blocked commands
	lowerCmd := strings.ToLower(command)
	for _, blocked := range e.blockedCmds {
		if strings.Contains(lowerCmd, strings.ToLower(blocked)) {
			return fmt.Errorf("command contains blocked pattern: %s", blocked)
		}
	}

	return nil
}

// SetTimeout changes the execution timeout
func (e *Executor) SetTimeout(timeout time.Duration) {
	e.timeout = timeout
}

// IsSafeCommand performs a quick safety check without executing
func (e *Executor) IsSafeCommand(command string) error {
	return e.validate(command)
}
