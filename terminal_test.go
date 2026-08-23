//go:build !windows

package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/vdbergkevin/vibe-dock/internal/store"
)

func TestIntegratedTerminalLifecycle(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	dataStore, err := store.Open(filepath.Join(t.TempDir(), "integrated-terminal.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewAppService(dataStore, "")
	defer service.Close()
	projectRoot := t.TempDir()
	project, err := dataStore.AddProject(t.Context(), projectRoot)
	if err != nil {
		t.Fatal(err)
	}

	session, err := service.StartProjectTerminal(project.ID, 90, 28)
	if err != nil {
		t.Fatal(err)
	}
	process, err := service.terminal(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if session.CWD != projectRoot || session.Shell != "sh" {
		t.Fatalf("unexpected terminal session: %#v", session)
	}
	if err := service.ResizeProjectTerminal(session.ID, 100, 32); err != nil {
		t.Fatal(err)
	}
	if err := service.AttachProjectTerminal(session.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.WriteProjectTerminal(session.ID, "printf 'terminal-ready\\n'\nexit\n"); err != nil {
		t.Fatal(err)
	}

	select {
	case <-process.done:
	case <-time.After(3 * time.Second):
		t.Fatal("terminal did not exit")
	}
	if _, err := service.terminal(session.ID); err == nil {
		t.Fatal("exited terminal remained registered")
	}
}
