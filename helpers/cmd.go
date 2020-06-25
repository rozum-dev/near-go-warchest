package helpers

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func Run(cmd string) (string, error) {
	cmd = strings.Replace(cmd, `"`, "", -1)
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return "", errors.New(fmt.Sprintf("Failed to execute command: %s", cmd))
	}
	return string(out), nil
}
