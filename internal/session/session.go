package session

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type DB struct {
	*gorm.DB
}

type Session struct {
	SessionID     string `gorm:"session_id;primaryKey"`
	Name          string `gorm:"name"`
	Content       string `gorm:"json"`
	CreatedAtUsec int64  `gorm:"created_at_usec;index"`
	UpdatedAtUsec int64  `gorm:"updated_at_usec;index"`
}

func getDBPath() (string, error) {
	if os.Getenv("GPT_DB_PATH") != "" {
		return os.Getenv("GPT_DB_PATH"), nil
	}
	path, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get config dir: %w", err)
	}
	path = filepath.Join(path, "gpt-cli", "gpt.db")
	return path, nil
}

func GetDB() (*DB, error) {
	path, err := getDBPath()
	if err != nil {
		return nil, err
	}

	// Create DB file if it doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
		f, err := os.Create(path)
		if err != nil {
			return nil, fmt.Errorf("create db file: %w", err)
		}
		_ = f.Close()
	}

	// Open DB and auto-migrate
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.AutoMigrate(&Session{}); err != nil {
		return nil, fmt.Errorf("migrate db: %w", err)
	}

	return &DB{db}, nil
}

func GetSessions(db *DB) ([]Session, error) {
	var sessions []Session
	err := db.Order("updated_at_usec DESC").Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("get sessions: %w", err)
	}
	return sessions, nil
}

func SaveSession(db *DB, session Session) error {
	now := time.Now()
	if session.SessionID == "" {
		session.SessionID = uuid.New().String()
	}
	if session.CreatedAtUsec == 0 {
		session.CreatedAtUsec = now.UnixMicro()
	}
	session.UpdatedAtUsec = time.Now().UnixMicro()

	if err := db.Save(&session).Error; err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	return nil
}
