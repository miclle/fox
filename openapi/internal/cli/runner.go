package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	ExitUsage       = 1
	ExitBuildFailed = 2
	ExitRunFailed   = 3
	ExitDrift       = 4
	ExitWriteFailed = 5
)

type DriverError struct {
	Phase    string
	ExitCode int
	Stderr   string
	Cause    error
}

func (e *DriverError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("driver %s failed: %v\n%s", e.Phase, e.Cause, e.Stderr)
	}
	return fmt.Sprintf("driver %s failed: %v", e.Phase, e.Cause)
}

func RunDriver(driverDir string) ([]byte, string, error) {
	binPath := filepath.Join(driverDir, "driver.bin")
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = driverDir
	var buildErr bytes.Buffer
	buildCmd.Stderr = &buildErr
	if err := buildCmd.Run(); err != nil {
		return nil, "", &DriverError{Phase: "build", ExitCode: ExitBuildFailed, Stderr: buildErr.String(), Cause: err}
	}
	defer os.Remove(binPath)
	runCmd := exec.Command(binPath)
	runCmd.Dir = driverDir
	var stdout, stderr bytes.Buffer
	runCmd.Stdout = &stdout
	runCmd.Stderr = &stderr
	if err := runCmd.Run(); err != nil {
		return nil, stderr.String(), &DriverError{Phase: "run", ExitCode: ExitRunFailed, Stderr: stderr.String(), Cause: err}
	}
	return stdout.Bytes(), stderr.String(), nil
}

func warningLines(stderr string) []string {
	lines := strings.Split(stderr, "\n")
	warnings := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "WARN:") {
			warnings = append(warnings, line)
		}
	}
	return warnings
}
