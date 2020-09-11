package helpers

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

func Run(ctx context.Context, cmd string) (string, error) {
	c, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// fmt.Printf("Try to run command %s\n", cmd)
	out, err := exec.CommandContext(c, "bash", "-c", cmd).Output()


	if c.Err() == context.DeadlineExceeded {
		fmt.Printf("Command %s timed out\n", cmd)
		return "", context.DeadlineExceeded
	}

	if err != nil {
		return "", errors.New(fmt.Sprintf("Failed to execute command: %s", cmd))
	}
	return string(out), nil
}
