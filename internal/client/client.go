package client

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustin/go-humanize"
	"github.com/gaurav-gosain/gollama/internal/api"
	"github.com/jmoiron/sqlx"
)

var GollamaInstance Gollama

type Gollama struct {
	API     api.OllamaAPI
	Program *tea.Program
	DB      *sqlx.DB
}

type Chat struct {
	UpdatedAt     time.Time `db:"updated_at"`
	ID            string    `db:"id"`
	ChatTitle     string    `db:"title"`
	SystemMessage string    `db:"system_message"`
	ModelName     string    `db:"model_name"`
	IsAnonymous   bool      `db:"is_anonymous"`
	IsMultiModal  bool      `db:"is_multi_modal"`
}

func (i Chat) Title() string       { return i.ChatTitle }
func (i Chat) Description() string { return humanize.Time(i.UpdatedAt) + " • " + i.ModelName }
func (i Chat) FilterValue() string { return i.ChatTitle + i.ModelName }

func initDatabase() (*sqlx.DB, error) {
	CachePath := filepath.Join(xdg.DataHome, "gollama", "chats")

	if err := os.MkdirAll(CachePath, 0o700); err != nil { //nolint:mnd
		return nil, fmt.Errorf("could not create cache directory")
	}

	return sqlx.Open("sqlite3", filepath.Join(CachePath, "chats.db"))
}

// TODO: handle sqlite errors more gracefully
func handleSqliteErr(err error) error {
	return err
}

func (g *Gollama) Migrate() error {
	if _, err := g.DB.Exec(`
		CREATE TABLE
		  IF NOT EXISTS chats (
		    id string NOT NULL PRIMARY KEY,
		    title string NOT NULL,
        system_message string NOT NULL,
        is_anonymous boolean NOT NULL,
        model_name string NOT NULL,
        is_multi_modal boolean NOT NULL,
		    updated_at datetime NOT NULL DEFAULT (strftime ('%Y-%m-%d %H:%M:%f', 'now')),
		    CHECK (id <> ''),
		    CHECK (title <> '')
		  )
	`); err != nil {
		return fmt.Errorf("could not migrate db: %w", err)
	}

	if _, err := g.DB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_chat_id ON chats (id)
	`); err != nil {
		return fmt.Errorf("could not migrate db: %w", err)
	}
	if _, err := g.DB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_chat_title ON chats (title)
	`); err != nil {
		return fmt.Errorf("could not migrate db: %w", err)
	}

	return nil
}

func (g *Gollama) InitDB() error {
	db, err := initDatabase()
	if err != nil {
		return fmt.Errorf("could not create db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf(
			"could not ping db: %w",
			handleSqliteErr(err),
		)
	}

	g.DB = db

	return g.Migrate()
}

func (g *Gollama) Connect(api api.OllamaAPI, program *tea.Program) {
	g.API = api
	g.Program = program
}

func (g *Gollama) ListChats() ([]Chat, error) {
	var chats []Chat

	err := g.DB.Select(
		&chats,
		"SELECT * FROM chats ORDER BY updated_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("could not list chats: %w", err)
	}

	return chats, nil
}

func (g *Gollama) CreateChat(chat Chat) error {
	_, err := g.DB.Exec(
		`
        INSERT INTO chats (id, title, system_message, is_anonymous, model_name, is_multi_modal)
        VALUES (?, ?, ?, ?, ?, ?)
    `,
		chat.ID,
		chat.ChatTitle,
		chat.SystemMessage,
		chat.IsAnonymous,
		chat.ModelName,
		chat.IsMultiModal,
	)
	if err != nil {
		return fmt.Errorf("could not create chat: %w", err)
	}
	return nil
}

func (g *Gollama) DeleteChat(id string) error {
	_, err := g.DB.Exec(
		`
        DELETE FROM chats WHERE id = ?
    `,
		id,
	)
	if err != nil {
		return fmt.Errorf("could not delete chat: %w", err)
	}
	return nil
}
