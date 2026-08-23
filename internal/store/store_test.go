package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/vdbergkevin/vibe-dock/internal/model"
)

func TestProjectConversationAndMessages(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	project, err := s.AddProject(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	conversation, err := s.CreateConversation(ctx, project.ID, "Build the shell")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetConversationMode(ctx, conversation.ID, "custom-review-agent"); err != nil {
		t.Fatalf("persist Vibe-advertised agent mode: %v", err)
	}
	message, err := s.AddMessage(ctx, model.Message{SessionID: conversation.ID, Role: "user", Kind: "text", Content: "Create the first screen"})
	if err != nil {
		t.Fatal(err)
	}
	if message.ID == "" {
		t.Fatal("expected message ID")
	}
	if _, err := s.AddMessage(ctx, model.Message{
		SessionID: conversation.ID,
		Role:      "assistant",
		Kind:      "text",
		Content:   "The first screen is ready.",
		Metadata:  map[string]any{"durationMs": int64(64_000)},
	}); err != nil {
		t.Fatal(err)
	}
	projects, err := s.Projects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || len(projects[0].Sessions) != 1 {
		t.Fatalf("unexpected projects: %#v", projects)
	}
	if projects[0].Sessions[0].Mode != "custom-review-agent" {
		t.Fatalf("unexpected persisted agent mode: %q", projects[0].Sessions[0].Mode)
	}
	if err := s.SetConversationTitle(ctx, conversation.ID, "Build the desktop shell"); err != nil {
		t.Fatal(err)
	}
	updatedConversation, err := s.Conversation(ctx, conversation.ID)
	if err != nil || updatedConversation.Title != "Build the desktop shell" {
		t.Fatalf("conversation title was not updated: %#v, %v", updatedConversation, err)
	}
	messages, err := s.Messages(ctx, conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0].Content != "Create the first screen" {
		t.Fatalf("unexpected messages: %#v", messages)
	}
	if duration, ok := messages[1].Metadata["durationMs"].(float64); !ok || duration != 64_000 {
		t.Fatalf("unexpected worked duration metadata: %#v", messages[1].Metadata)
	}
}

func TestDeleteConversationCascadesMessagesAndLibraryAttachments(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "delete-conversation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	project, err := s.AddProject(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	conversation, err := s.CreateConversation(ctx, project.ID, "Temporary conversation")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddMessage(ctx, model.Message{SessionID: conversation.ID, Role: "user", Kind: "text", Content: "Temporary message"}); err != nil {
		t.Fatal(err)
	}
	library, err := s.CreateLibrary(ctx, "Temporary context", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetConversationLibraries(ctx, conversation.ID, []string{library.ID}); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteConversation(ctx, conversation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Conversation(ctx, conversation.ID); err == nil {
		t.Fatal("expected deleted conversation to be missing")
	}
	messages, err := s.Messages(ctx, conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Fatalf("messages were not deleted: %#v", messages)
	}
	libraryIDs, err := s.ConversationLibraryIDs(ctx, conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(libraryIDs) != 0 {
		t.Fatalf("Library attachments were not deleted: %#v", libraryIDs)
	}
	if err := s.DeleteConversation(ctx, conversation.ID); err == nil {
		t.Fatal("expected deleting a missing conversation to fail")
	}
}

func TestChatWorkAndCodeProjectsRemainSeparated(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "projects.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	codeProject, err := s.AddProject(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	chatProject, err := s.AddChatProject(ctx, "Product ideas", filepath.Join(t.TempDir(), "managed-chat"))
	if err != nil {
		t.Fatal(err)
	}
	workProject, err := s.AddManagedProject(ctx, "Launch research", filepath.Join(t.TempDir(), "managed-work"), "work")
	if err != nil {
		t.Fatal(err)
	}
	if codeProject.Kind != "code" || chatProject.Kind != "chat" || workProject.Kind != "work" {
		t.Fatalf("unexpected project kinds: code=%q chat=%q work=%q", codeProject.Kind, chatProject.Kind, workProject.Kind)
	}
	if chatProject.Icon != "messages" || workProject.Icon != "briefcase" {
		t.Fatalf("unexpected default icons: chat=%q work=%q", chatProject.Icon, workProject.Icon)
	}
	projects, err := s.Projects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 3 {
		t.Fatalf("expected three projects, got %#v", projects)
	}
}

func TestManagedProjectAppearancePersistsAndCanBeUpdated(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "appearance.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	project, err := s.AddManagedProjectWithAppearance(ctx, "Trip planning", filepath.Join(t.TempDir(), "trip"), "chat", "plane", "#5C91D8")
	if err != nil {
		t.Fatal(err)
	}
	if project.Icon != "plane" || project.Color != "#5c91d8" {
		t.Fatalf("unexpected created appearance: %#v", project)
	}
	updated, err := s.UpdateProjectAppearance(ctx, project.ID, "globe", "#4fa78f")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Icon != "globe" || updated.Color != "#4fa78f" {
		t.Fatalf("unexpected updated appearance: %#v", updated)
	}
	reloaded, err := s.Project(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Icon != "globe" || reloaded.Color != "#4fa78f" {
		t.Fatalf("appearance was not persisted: %#v", reloaded)
	}
	if _, err := s.UpdateProjectAppearance(ctx, project.ID, "not-an-icon", "#4fa78f"); err == nil {
		t.Fatal("expected invalid icon to be rejected")
	}
}

func TestLibrariesPersistDocumentsAndConversationAttachments(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "libraries.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	project, err := s.AddManagedProject(ctx, "Research", filepath.Join(t.TempDir(), "managed-work"), "work")
	if err != nil {
		t.Fatal(err)
	}
	conversation, err := s.CreateConversation(ctx, project.ID, "Prepare brief")
	if err != nil {
		t.Fatal(err)
	}
	library, err := s.CreateLibrary(ctx, "Product knowledge", "Specs and release notes")
	if err != nil {
		t.Fatal(err)
	}
	document, err := s.AddLibraryDocument(ctx, model.LibraryDocument{LibraryID: library.ID, Name: "spec.md", Kind: "file", Source: "/tmp/spec.md", LocalPath: "/tmp/library/spec.md", Size: 42})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddLibraryDocument(ctx, model.LibraryDocument{LibraryID: library.ID, Name: "Release notes", Kind: "webpage", Source: "https://example.com/releases"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetConversationLibraries(ctx, conversation.ID, []string{library.ID, library.ID}); err != nil {
		t.Fatal(err)
	}
	loadedConversation, err := s.Conversation(ctx, conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loadedConversation.LibraryIDs) != 1 || loadedConversation.LibraryIDs[0] != library.ID {
		t.Fatalf("unexpected Library attachments: %#v", loadedConversation.LibraryIDs)
	}
	attached, err := s.ConversationLibraries(ctx, conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(attached) != 1 || len(attached[0].Documents) != 2 {
		t.Fatalf("unexpected attached Libraries: %#v", attached)
	}
	if err := s.DeleteLibraryDocument(ctx, document.ID); err != nil {
		t.Fatal(err)
	}
	loadedLibrary, err := s.Library(ctx, library.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loadedLibrary.Documents) != 1 || loadedLibrary.Documents[0].Kind != "webpage" {
		t.Fatalf("unexpected remaining documents: %#v", loadedLibrary.Documents)
	}
}

func TestLegacyProjectsMigrateToCode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.Exec(`CREATE TABLE projects (id TEXT PRIMARY KEY, name TEXT NOT NULL, path TEXT NOT NULL UNIQUE, color TEXT NOT NULL, pinned INTEGER NOT NULL DEFAULT 0, last_opened TEXT NOT NULL, created_at TEXT NOT NULL)`)
	if err == nil {
		_, err = db.Exec(`INSERT INTO projects(id,name,path,color,pinned,last_opened,created_at) VALUES(?,?,?,?,?,?,?)`, "legacy", "Legacy", filepath.Join(t.TempDir(), "legacy-project"), "#fff", 0, now, now)
	}
	if closeErr := db.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	project, err := s.Project(context.Background(), "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if project.Kind != "code" {
		t.Fatalf("legacy project migrated as %q", project.Kind)
	}
	if project.Icon != "" {
		t.Fatalf("legacy project icon = %q, want empty", project.Icon)
	}
}

func TestPluginUpsert(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	plugin, err := s.SavePlugin(ctx, model.Plugin{Name: "Filesystem", Command: "npx", Args: []string{"-y", "server"}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	plugin.Enabled = false
	if _, err := s.SavePlugin(ctx, plugin); err != nil {
		t.Fatal(err)
	}
	plugins, err := s.Plugins(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 || plugins[0].Enabled {
		t.Fatalf("unexpected plugins: %#v", plugins)
	}
}
