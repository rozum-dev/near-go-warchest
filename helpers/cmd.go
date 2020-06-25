package helpers

import (
	"errors"
	"fmt"
	"os/exec"
)

func Run(cmd string) (string, error) {
	out, err := exec.Command("bash", "-c", cmd[1:len(cmd)-1]).Output()
	if err != nil {
		return "", errors.New(fmt.Sprintf("Failed to execute command: %s", cmd))
	}
	return string(out), nil
}
