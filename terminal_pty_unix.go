//go:build !windows

package main

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type unixPTY struct{ *os.File }

func (p *unixPTY) Resize(columns, rows uint16) error {
	return pty.Setsize(p.File, &pty.Winsize{Cols: columns, Rows: rows})
}

func startPlatformPTY(command *exec.Cmd, columns, rows uint16) (terminalPTY, error) {
	file, err := pty.StartWithSize(command, &pty.Winsize{Cols: columns, Rows: rows})
	if err != nil {
		return nil, err
	}
	return &unixPTY{File: file}, nil
}
