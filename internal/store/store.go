package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/vdbergkevin/vibe-dock/internal/model"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure database: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS projects (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  path TEXT NOT NULL UNIQUE,
  kind TEXT NOT NULL DEFAULT 'code',
  color TEXT NOT NULL,
  icon TEXT NOT NULL DEFAULT '',
  pinned INTEGER NOT NULL DEFAULT 0,
  last_opened TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS conversations (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  mode TEXT NOT NULL DEFAULT 'default',
  agent_session_id TEXT NOT NULL DEFAULT '',
  preview TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS conversations_project_updated ON conversations(project_id, updated_at DESC);
CREATE TABLE IF NOT EXISTS messages (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  role TEXT NOT NULL,
  kind TEXT NOT NULL,
  content TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'complete',
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS messages_session_created ON messages(session_id, created_at);
CREATE TABLE IF NOT EXISTS libraries (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  color TEXT NOT NULL,
  remote_id TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS library_documents (
  id TEXT PRIMARY KEY,
  library_id TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  source TEXT NOT NULL,
  local_path TEXT NOT NULL DEFAULT '',
  size INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'ready',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS library_documents_library ON library_documents(library_id, created_at);
CREATE TABLE IF NOT EXISTS conversation_libraries (
  session_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  library_id TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  attached_at TEXT NOT NULL,
  PRIMARY KEY(session_id, library_id)
);
CREATE TABLE IF NOT EXISTS plugins (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  transport TEXT NOT NULL DEFAULT 'stdio',
  command TEXT NOT NULL DEFAULT '',
  args_json TEXT NOT NULL DEFAULT '[]',
  env_json TEXT NOT NULL DEFAULT '{}',
  enabled INTEGER NOT NULL DEFAULT 1,
  scope TEXT NOT NULL DEFAULT 'global',
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
UPDATE projects SET color='#ff7417' WHERE lower(color) IN ('#7c6ff2','#9b72d9','#8f72dc');
UPDATE libraries SET color='#ff7417' WHERE lower(color) IN ('#7c6ff2','#9b72d9','#8f72dc');`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	if err := s.ensureProjectColumns(); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureProjectColumns() error {
	rows, err := s.db.Query(`PRAGMA table_info(projects)`)
	if err != nil {
		return fmt.Errorf("inspect projects schema: %w", err)
	}
	found := map[string]bool{}
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan projects schema: %w", err)
		}
		found[name] = true
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !found["kind"] {
		if _, err := s.db.Exec(`ALTER TABLE projects ADD COLUMN kind TEXT NOT NULL DEFAULT 'code'`); err != nil {
			return fmt.Errorf("add project kind: %w", err)
		}
	}
	if !found["icon"] {
		if _, err := s.db.Exec(`ALTER TABLE projects ADD COLUMN icon TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add project icon: %w", err)
		}
	}
	return nil
}

func NewID(prefix string) string {
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}

func (s *Store) Projects(ctx context.Context) ([]model.Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,path,kind,color,icon,pinned,last_opened,created_at FROM projects ORDER BY pinned DESC,last_opened DESC`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	projects := make([]model.Project, 0)
	for rows.Next() {
		var p model.Project
		var pinned int
		var lastOpened, createdAt string
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Kind, &p.Color, &p.Icon, &pinned, &lastOpened, &createdAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		p.Pinned = pinned != 0
		p.LastOpened, _ = time.Parse(time.RFC3339Nano, lastOpened)
		p.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	// The store deliberately uses one SQLite connection. Release the project
	// result set before loading children so nested queries cannot deadlock.
	for i := range projects {
		projects[i].Sessions, err = s.Conversations(ctx, projects[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return projects, nil
}

func (s *Store) Project(ctx context.Context, projectID string) (model.Project, error) {
	var p model.Project
	var pinned int
	var lastOpened, createdAt string
	err := s.db.QueryRowContext(ctx, `SELECT id,name,path,kind,color,icon,pinned,last_opened,created_at FROM projects WHERE id=?`, projectID).
		Scan(&p.ID, &p.Name, &p.Path, &p.Kind, &p.Color, &p.Icon, &pinned, &lastOpened, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return p, errors.New("project not found")
	}
	if err != nil {
		return p, err
	}
	p.Pinned = pinned != 0
	p.LastOpened, _ = time.Parse(time.RFC3339Nano, lastOpened)
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	p.Sessions, err = s.Conversations(ctx, projectID)
	return p, err
}

func (s *Store) AddProject(ctx context.Context, path string) (model.Project, error) {
	cleanPath, err := filepath.Abs(filepath.Clean(strings.TrimSpace(path)))
	if err != nil {
		return model.Project{}, fmt.Errorf("resolve project path: %w", err)
	}
	if cleanPath == string(filepath.Separator) {
		return model.Project{}, errors.New("the filesystem root cannot be added as a project")
	}
	now := time.Now().UTC()
	p := model.Project{
		ID: NewID("prj"), Name: filepath.Base(cleanPath), Path: cleanPath, Kind: "code",
		Color: projectColor(cleanPath), LastOpened: now, CreatedAt: now, Sessions: []model.Conversation{},
	}
	if err := s.insertProject(ctx, p); err != nil {
		return model.Project{}, err
	}
	return p, nil
}

func (s *Store) AddChatProject(ctx context.Context, name, path string) (model.Project, error) {
	return s.AddManagedProject(ctx, name, path, "chat")
}

func (s *Store) AddManagedProject(ctx context.Context, name, path, kind string) (model.Project, error) {
	return s.AddManagedProjectWithAppearance(ctx, name, path, kind, "", "")
}

func (s *Store) AddManagedProjectWithAppearance(ctx context.Context, name, path, kind, icon, color string) (model.Project, error) {
	if kind != "chat" && kind != "work" {
		return model.Project{}, errors.New("unsupported managed project kind")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		if kind == "work" {
			name = "Work"
		} else {
			name = "Chats"
		}
	}
	if len(name) > 120 {
		return model.Project{}, errors.New("project name is too long")
	}
	cleanPath, err := filepath.Abs(filepath.Clean(strings.TrimSpace(path)))
	if err != nil {
		return model.Project{}, fmt.Errorf("resolve managed workspace: %w", err)
	}
	now := time.Now().UTC()
	icon, err = validatedProjectIcon(icon, kind)
	if err != nil {
		return model.Project{}, err
	}
	color, err = validatedProjectColor(color, kind+":"+name)
	if err != nil {
		return model.Project{}, err
	}
	p := model.Project{
		ID: NewID("prj"), Name: name, Path: cleanPath, Kind: kind,
		Color: color, Icon: icon, LastOpened: now, CreatedAt: now, Sessions: []model.Conversation{},
	}
	if err := s.insertProject(ctx, p); err != nil {
		return model.Project{}, err
	}
	return p, nil
}

func (s *Store) insertProject(ctx context.Context, p model.Project) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO projects(id,name,path,kind,color,icon,pinned,last_opened,created_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		p.ID, p.Name, p.Path, p.Kind, p.Color, p.Icon, 0, p.LastOpened.Format(time.RFC3339Nano), p.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return errors.New("this project has already been added")
		}
		return fmt.Errorf("add project: %w", err)
	}
	return nil
}

func (s *Store) UpdateProjectAppearance(ctx context.Context, projectID, icon, color string) (model.Project, error) {
	project, err := s.Project(ctx, projectID)
	if err != nil {
		return model.Project{}, err
	}
	if project.Kind != "chat" && project.Kind != "work" {
		return model.Project{}, errors.New("only chat and work groups can be customized")
	}
	icon, err = validatedProjectIcon(icon, project.Kind)
	if err != nil {
		return model.Project{}, err
	}
	color, err = validatedProjectColor(color, project.Kind+":"+project.Name)
	if err != nil {
		return model.Project{}, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE projects SET icon=?,color=? WHERE id=?`, icon, color, projectID); err != nil {
		return model.Project{}, fmt.Errorf("update project appearance: %w", err)
	}
	project.Icon = icon
	project.Color = color
	return project, nil
}

func (s *Store) RemoveProject(ctx context.Context, projectID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id=?`, projectID)
	if err != nil {
		return fmt.Errorf("remove project: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return errors.New("project not found")
	}
	return nil
}

func (s *Store) TouchProject(ctx context.Context, projectID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE projects SET last_opened=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), projectID)
	return err
}

func (s *Store) Conversations(ctx context.Context, projectID string) ([]model.Conversation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,project_id,title,mode,agent_session_id,preview,updated_at,created_at FROM conversations WHERE project_id=? ORDER BY updated_at DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	items := make([]model.Conversation, 0)
	for rows.Next() {
		item, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].LibraryIDs, err = s.ConversationLibraryIDs(ctx, items[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Store) Conversation(ctx context.Context, sessionID string) (model.Conversation, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,project_id,title,mode,agent_session_id,preview,updated_at,created_at FROM conversations WHERE id=?`, sessionID)
	item, err := scanConversation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Conversation{}, errors.New("conversation not found")
	}
	if err != nil {
		return item, err
	}
	item.LibraryIDs, err = s.ConversationLibraryIDs(ctx, item.ID)
	return item, err
}

type scanner interface{ Scan(...any) error }

func scanConversation(row scanner) (model.Conversation, error) {
	var c model.Conversation
	var updatedAt, createdAt string
	if err := row.Scan(&c.ID, &c.ProjectID, &c.Title, &c.Mode, &c.AgentSessionID, &c.Preview, &updatedAt, &createdAt); err != nil {
		return c, err
	}
	c.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	c.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	return c, nil
}

func (s *Store) CreateConversation(ctx context.Context, projectID, title string) (model.Conversation, error) {
	if strings.TrimSpace(title) == "" {
		title = "New conversation"
	}
	now := time.Now().UTC()
	c := model.Conversation{ID: NewID("ses"), ProjectID: projectID, Title: strings.TrimSpace(title), Mode: "default", UpdatedAt: now, CreatedAt: now, LibraryIDs: []string{}}
	_, err := s.db.ExecContext(ctx, `INSERT INTO conversations(id,project_id,title,mode,agent_session_id,preview,updated_at,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		c.ID, c.ProjectID, c.Title, c.Mode, "", "", now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return model.Conversation{}, fmt.Errorf("create conversation: %w", err)
	}
	_ = s.TouchProject(ctx, projectID)
	return c, nil
}

func (s *Store) DeleteConversation(ctx context.Context, sessionID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM conversations WHERE id=?`, sessionID)
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return errors.New("conversation not found")
	}
	return nil
}

func (s *Store) SetConversationTitle(ctx context.Context, sessionID, title string) error {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 160 {
		return errors.New("conversation title is invalid")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE conversations SET title=?,updated_at=? WHERE id=?`, title, time.Now().UTC().Format(time.RFC3339Nano), sessionID)
	if err != nil {
		return fmt.Errorf("update conversation title: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return errors.New("conversation not found")
	}
	return nil
}

func (s *Store) SetConversationMode(ctx context.Context, sessionID, mode string) error {
	mode = strings.TrimSpace(mode)
	if mode == "" || len(mode) > 256 {
		return errors.New("unsupported conversation mode")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE conversations SET mode=?,updated_at=? WHERE id=?`, mode, time.Now().UTC().Format(time.RFC3339Nano), sessionID)
	return err
}

func (s *Store) SetAgentSessionID(ctx context.Context, sessionID, agentSessionID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE conversations SET agent_session_id=? WHERE id=?`, agentSessionID, sessionID)
	return err
}

func (s *Store) Messages(ctx context.Context, sessionID string) ([]model.Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,session_id,role,kind,content,status,metadata_json,created_at FROM messages WHERE session_id=? ORDER BY created_at`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()
	items := make([]model.Message, 0)
	for rows.Next() {
		var m model.Message
		var metadata, createdAt string
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Kind, &m.Content, &m.Status, &metadata, &createdAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(metadata), &m.Metadata)
		m.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		items = append(items, m)
	}
	return items, rows.Err()
}

func (s *Store) AddMessage(ctx context.Context, m model.Message) (model.Message, error) {
	if m.ID == "" {
		m.ID = NewID("msg")
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	if m.Status == "" {
		m.Status = "complete"
	}
	metadata, _ := json.Marshal(m.Metadata)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Message{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO messages(id,session_id,role,kind,content,status,metadata_json,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		m.ID, m.SessionID, m.Role, m.Kind, m.Content, m.Status, string(metadata), m.CreatedAt.Format(time.RFC3339Nano)); err != nil {
		return model.Message{}, fmt.Errorf("add message: %w", err)
	}
	preview := strings.TrimSpace(strings.ReplaceAll(m.Content, "\n", " "))
	if len(preview) > 120 {
		preview = preview[:117] + "..."
	}
	if _, err := tx.ExecContext(ctx, `UPDATE conversations SET preview=?,updated_at=? WHERE id=?`, preview, m.CreatedAt.Format(time.RFC3339Nano), m.SessionID); err != nil {
		return model.Message{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Message{}, err
	}
	return m, nil
}

func (s *Store) Libraries(ctx context.Context) ([]model.Library, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,description,color,remote_id,created_at,updated_at FROM libraries ORDER BY updated_at DESC,name`)
	if err != nil {
		return nil, fmt.Errorf("list libraries: %w", err)
	}
	items := make([]model.Library, 0)
	for rows.Next() {
		item, scanErr := scanLibrary(rows)
		if scanErr != nil {
			_ = rows.Close()
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Documents, err = s.LibraryDocuments(ctx, items[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Store) Library(ctx context.Context, libraryID string) (model.Library, error) {
	item, err := scanLibrary(s.db.QueryRowContext(ctx, `SELECT id,name,description,color,remote_id,created_at,updated_at FROM libraries WHERE id=?`, libraryID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Library{}, errors.New("library not found")
	}
	if err != nil {
		return model.Library{}, err
	}
	item.Documents, err = s.LibraryDocuments(ctx, item.ID)
	return item, err
}

func scanLibrary(row scanner) (model.Library, error) {
	var item model.Library
	var createdAt, updatedAt string
	if err := row.Scan(&item.ID, &item.Name, &item.Description, &item.Color, &item.RemoteID, &createdAt, &updatedAt); err != nil {
		return item, err
	}
	item.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	item.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	item.Documents = []model.LibraryDocument{}
	return item, nil
}

func (s *Store) CreateLibrary(ctx context.Context, name, description string) (model.Library, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return model.Library{}, errors.New("library name is required")
	}
	if len(name) > 120 {
		return model.Library{}, errors.New("library name is too long")
	}
	if len(description) > 500 {
		return model.Library{}, errors.New("library description is too long")
	}
	now := time.Now().UTC()
	item := model.Library{
		ID: NewID("lib"), Name: name, Description: description, Color: projectColor("library:" + name),
		Documents: []model.LibraryDocument{}, CreatedAt: now, UpdatedAt: now,
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO libraries(id,name,description,color,remote_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`,
		item.ID, item.Name, item.Description, item.Color, "", now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return model.Library{}, fmt.Errorf("create library: %w", err)
	}
	return item, nil
}

func (s *Store) DeleteLibrary(ctx context.Context, libraryID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM libraries WHERE id=?`, libraryID)
	if err != nil {
		return fmt.Errorf("delete library: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return errors.New("library not found")
	}
	return nil
}

func (s *Store) LibraryDocuments(ctx context.Context, libraryID string) ([]model.LibraryDocument, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,library_id,name,kind,source,local_path,size,status,created_at FROM library_documents WHERE library_id=? ORDER BY created_at,name`, libraryID)
	if err != nil {
		return nil, fmt.Errorf("list library documents: %w", err)
	}
	defer rows.Close()
	items := make([]model.LibraryDocument, 0)
	for rows.Next() {
		var item model.LibraryDocument
		var createdAt string
		if err := rows.Scan(&item.ID, &item.LibraryID, &item.Name, &item.Kind, &item.Source, &item.LocalPath, &item.Size, &item.Status, &createdAt); err != nil {
			return nil, err
		}
		item.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AddLibraryDocument(ctx context.Context, item model.LibraryDocument) (model.LibraryDocument, error) {
	if item.ID == "" {
		item.ID = NewID("doc")
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	if item.Status == "" {
		item.Status = "ready"
	}
	if item.Kind != "file" && item.Kind != "webpage" {
		return model.LibraryDocument{}, errors.New("unsupported library document kind")
	}
	if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Source) == "" {
		return model.LibraryDocument{}, errors.New("library document name and source are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.LibraryDocument{}, err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO library_documents(id,library_id,name,kind,source,local_path,size,status,created_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		item.ID, item.LibraryID, item.Name, item.Kind, item.Source, item.LocalPath, item.Size, item.Status, item.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return model.LibraryDocument{}, fmt.Errorf("add library document: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE libraries SET updated_at=? WHERE id=?`, item.CreatedAt.Format(time.RFC3339Nano), item.LibraryID); err != nil {
		return model.LibraryDocument{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.LibraryDocument{}, err
	}
	return item, nil
}

func (s *Store) DeleteLibraryDocument(ctx context.Context, documentID string) error {
	var libraryID string
	if err := s.db.QueryRowContext(ctx, `SELECT library_id FROM library_documents WHERE id=?`, documentID).Scan(&libraryID); errors.Is(err, sql.ErrNoRows) {
		return errors.New("library document not found")
	} else if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM library_documents WHERE id=?`, documentID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE libraries SET updated_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), libraryID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ConversationLibraryIDs(ctx context.Context, sessionID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT library_id FROM conversation_libraries WHERE session_id=? ORDER BY attached_at`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) SetConversationLibraries(ctx context.Context, sessionID string, libraryIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM conversation_libraries WHERE session_id=?`, sessionID); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, libraryID := range libraryIDs {
		libraryID = strings.TrimSpace(libraryID)
		if libraryID == "" || seen[libraryID] {
			continue
		}
		seen[libraryID] = true
		if _, err := tx.ExecContext(ctx, `INSERT INTO conversation_libraries(session_id,library_id,attached_at) VALUES(?,?,?)`, sessionID, libraryID, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("attach library: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) ConversationLibraries(ctx context.Context, sessionID string) ([]model.Library, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT l.id,l.name,l.description,l.color,l.remote_id,l.created_at,l.updated_at FROM libraries l JOIN conversation_libraries cl ON cl.library_id=l.id WHERE cl.session_id=? ORDER BY cl.attached_at`, sessionID)
	if err != nil {
		return nil, err
	}
	items := make([]model.Library, 0)
	for rows.Next() {
		item, scanErr := scanLibrary(rows)
		if scanErr != nil {
			_ = rows.Close()
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Documents, err = s.LibraryDocuments(ctx, items[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Store) Plugins(ctx context.Context) ([]model.Plugin, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,description,transport,command,args_json,env_json,enabled,scope,updated_at FROM plugins ORDER BY enabled DESC,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.Plugin, 0)
	for rows.Next() {
		var p model.Plugin
		var argsJSON, envJSON, updatedAt string
		var enabled int
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Transport, &p.Command, &argsJSON, &envJSON, &enabled, &p.Scope, &updatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(argsJSON), &p.Args)
		_ = json.Unmarshal([]byte(envJSON), &p.Env)
		p.Enabled = enabled != 0
		p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		items = append(items, p)
	}
	return items, rows.Err()
}

func (s *Store) SavePlugin(ctx context.Context, p model.Plugin) (model.Plugin, error) {
	if p.ID == "" {
		p.ID = NewID("mcp")
	}
	if p.Transport == "" {
		p.Transport = "stdio"
	}
	if p.Scope == "" {
		p.Scope = "global"
	}
	p.UpdatedAt = time.Now().UTC()
	argsJSON, _ := json.Marshal(p.Args)
	envJSON, _ := json.Marshal(p.Env)
	_, err := s.db.ExecContext(ctx, `INSERT INTO plugins(id,name,description,transport,command,args_json,env_json,enabled,scope,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,description=excluded.description,transport=excluded.transport,command=excluded.command,args_json=excluded.args_json,env_json=excluded.env_json,enabled=excluded.enabled,scope=excluded.scope,updated_at=excluded.updated_at`,
		p.ID, p.Name, p.Description, p.Transport, p.Command, string(argsJSON), string(envJSON), boolInt(p.Enabled), p.Scope, p.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return model.Plugin{}, fmt.Errorf("save plugin: %w", err)
	}
	return p, nil
}

func (s *Store) Setting(ctx context.Context, key string) string {
	var value string
	_ = s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=?`, key).Scan(&value)
	return value
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func projectColor(path string) string {
	colors := []string{"#ff7417", "#db6337", "#4fa78f", "#e49b26", "#5c91d8", "#c56a55"}
	var sum int
	for _, r := range path {
		sum += int(r)
	}
	return colors[sum%len(colors)]
}

func validatedProjectIcon(icon, kind string) (string, error) {
	icon = strings.TrimSpace(strings.ToLower(icon))
	if icon == "" {
		if kind == "work" {
			return "briefcase", nil
		}
		return "messages", nil
	}
	allowed := map[string]bool{
		"messages": true, "sparkles": true, "lightbulb": true, "book": true,
		"briefcase": true, "heart": true, "rocket": true, "palette": true,
		"globe": true, "chart": true, "graduation": true, "music": true,
		"gamepad": true, "plane": true, "home": true, "star": true,
		"coffee": true, "code": true, "bot": true, "brain": true,
	}
	if !allowed[icon] {
		return "", errors.New("unsupported project icon")
	}
	return icon, nil
}

func validatedProjectColor(color, seed string) (string, error) {
	color = strings.TrimSpace(strings.ToLower(color))
	if color == "" {
		return projectColor(seed), nil
	}
	if len(color) != 7 || color[0] != '#' {
		return "", errors.New("project color must be a six-digit hex color")
	}
	for _, character := range color[1:] {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return "", errors.New("project color must be a six-digit hex color")
		}
	}
	return color, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
