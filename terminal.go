package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type TerminalSession struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	CWD       string `json:"cwd"`
	Shell     string `json:"shell"`
}

type terminalDataEvent struct {
	SessionID string `json:"sessionId"`
	Data      string `json:"data"`
}

type terminalExitEvent struct {
	SessionID string `json:"sessionId"`
	ExitCode  int    `json:"exitCode"`
	Error     string `json:"error,omitempty"`
}

type terminalPTY interface {
	io.ReadWriteCloser
	Resize(columns, rows uint16) error
}

type terminalProcess struct {
	session TerminalSession
	command *exec.Cmd
	pty     terminalPTY
	runOnce sync.Once
	done    chan struct{}
	writeMu sync.Mutex
	stateMu sync.Mutex
	stopped bool
}

var terminalSequence atomic.Uint64

func (s *AppService) StartProjectTerminal(projectID string, columns, rows int) (TerminalSession, error) {
	path, err := s.codeProjectPath(projectID)
	if err != nil {
		return TerminalSession{}, err
	}
	shell, err := preferredShell()
	if err != nil {
		return TerminalSession{}, err
	}
	columns, rows = terminalSize(columns, rows)
	command := exec.Command(shell, loginShellArgs(shell)...)
	command.Dir = path
	command.Env = terminalEnvironment()
	handle, err := startPlatformPTY(command, uint16(columns), uint16(rows))
	if err != nil {
		return TerminalSession{}, fmt.Errorf("start integrated terminal: %w", err)
	}
	session := TerminalSession{
		ID:        fmt.Sprintf("term_%d_%d", time.Now().UnixMilli(), terminalSequence.Add(1)),
		ProjectID: projectID,
		CWD:       path,
		Shell:     filepath.Base(shell),
	}
	process := &terminalProcess{session: session, command: command, pty: handle, done: make(chan struct{})}
	s.terminalsMu.Lock()
	s.terminals[session.ID] = process
	s.terminalsMu.Unlock()
	return session, nil
}

// AttachProjectTerminal starts streaming after the frontend has received the
// session ID, so the shell's first prompt cannot race the Wails method result.
func (s *AppService) AttachProjectTerminal(sessionID string) error {
	process, err := s.terminal(sessionID)
	if err != nil {
		return err
	}
	s.attachTerminal(process)
	return nil
}

func (s *AppService) WriteProjectTerminal(sessionID, data string) error {
	process, err := s.terminal(sessionID)
	if err != nil {
		return err
	}
	process.writeMu.Lock()
	defer process.writeMu.Unlock()
	_, err = io.WriteString(process.pty, data)
	if err != nil {
		return fmt.Errorf("write terminal: %w", err)
	}
	return nil
}

func (s *AppService) ResizeProjectTerminal(sessionID string, columns, rows int) error {
	process, err := s.terminal(sessionID)
	if err != nil {
		return err
	}
	columns, rows = terminalSize(columns, rows)
	if err := process.pty.Resize(uint16(columns), uint16(rows)); err != nil {
		return fmt.Errorf("resize terminal: %w", err)
	}
	return nil
}

func (s *AppService) StopProjectTerminal(sessionID string) error {
	process, err := s.terminal(sessionID)
	if err != nil {
		return nil
	}
	s.stopTerminal(process)
	return nil
}

func (s *AppService) terminal(sessionID string) (*terminalProcess, error) {
	s.terminalsMu.Lock()
	defer s.terminalsMu.Unlock()
	process := s.terminals[sessionID]
	if process == nil {
		return nil, errors.New("terminal session is no longer running")
	}
	return process, nil
}

func (s *AppService) attachTerminal(process *terminalProcess) {
	process.runOnce.Do(func() { go s.streamTerminal(process) })
}

func (s *AppService) streamTerminal(process *terminalProcess) {
	defer close(process.done)
	buffer := make([]byte, 32*1024)
	for {
		count, readErr := process.pty.Read(buffer)
		if count > 0 && s.app != nil {
			s.app.Event.Emit("terminal:data", terminalDataEvent{SessionID: process.session.ID, Data: string(buffer[:count])})
		}
		if readErr != nil {
			break
		}
	}
	waitErr := process.command.Wait()
	_ = process.pty.Close()
	s.terminalsMu.Lock()
	if s.terminals[process.session.ID] == process {
		delete(s.terminals, process.session.ID)
	}
	s.terminalsMu.Unlock()

	process.stateMu.Lock()
	stopped := process.stopped
	process.stateMu.Unlock()
	exit := terminalExitEvent{SessionID: process.session.ID}
	if process.command.ProcessState != nil {
		exit.ExitCode = process.command.ProcessState.ExitCode()
	}
	if waitErr != nil && !stopped {
		exit.Error = waitErr.Error()
	}
	if s.app != nil {
		s.app.Event.Emit("terminal:exit", exit)
	}
}

func (s *AppService) stopTerminal(process *terminalProcess) {
	process.stateMu.Lock()
	if process.stopped {
		process.stateMu.Unlock()
		return
	}
	process.stopped = true
	process.stateMu.Unlock()
	s.attachTerminal(process)
	if process.command.Process != nil {
		_ = process.command.Process.Signal(os.Interrupt)
	}
	_ = process.pty.Close()
	go func() {
		select {
		case <-process.done:
		case <-time.After(750 * time.Millisecond):
			if process.command.Process != nil {
				_ = process.command.Process.Kill()
			}
		}
	}()
}

func (s *AppService) closeTerminals() {
	s.terminalsMu.Lock()
	processes := make([]*terminalProcess, 0, len(s.terminals))
	for _, process := range s.terminals {
		processes = append(processes, process)
	}
	s.terminalsMu.Unlock()
	for _, process := range processes {
		s.stopTerminal(process)
	}
	for _, process := range processes {
		select {
		case <-process.done:
		case <-time.After(time.Second):
		}
	}
}

func terminalSize(columns, rows int) (int, int) {
	if columns < 20 || columns > 500 {
		columns = 100
	}
	if rows < 5 || rows > 200 {
		rows = 24
	}
	return columns, rows
}

func preferredShell() (string, error) {
	candidates := []string{os.Getenv("SHELL")}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates, "/bin/zsh", "/bin/bash", "/bin/sh")
	} else {
		candidates = append(candidates, "/bin/bash", "/bin/sh")
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", errors.New("no supported shell was found")
}

func loginShellArgs(shell string) []string {
	switch filepath.Base(shell) {
	case "bash", "dash", "fish", "ksh", "sh", "zsh":
		return []string{"-l"}
	default:
		return nil
	}
}

func terminalEnvironment() []string {
	environment := make([]string, 0, len(os.Environ())+3)
	for _, item := range os.Environ() {
		key := strings.SplitN(item, "=", 2)[0]
		if key != "TERM" && key != "COLORTERM" && key != "TERM_PROGRAM" {
			environment = append(environment, item)
		}
	}
	return append(environment, "TERM=xterm-256color", "COLORTERM=truecolor", "TERM_PROGRAM=VibeDesktop")
}
