//go:build windows

package main

import (
	"errors"
	"os/exec"
)

func startPlatformPTY(_ *exec.Cmd, _, _ uint16) (terminalPTY, error) {
	return nil, errors.New("the integrated terminal is not available on Windows yet; use the external terminal action")
}
