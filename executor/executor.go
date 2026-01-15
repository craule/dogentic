package executor

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// RunCommand executes a shell command and returns the output.
// It enforces a timeout to prevent hanging processes.
func RunCommand(cmdStr string) (string, error) {
	// 30 Seconds Timeout Default
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-Command", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", cmdStr)
	}

	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command timed out after 30s")
	}

	result := string(output)
	if err != nil {
		return fmt.Sprintf("%s\nError: %v", result, err), nil
	}

	return result, nil
}
