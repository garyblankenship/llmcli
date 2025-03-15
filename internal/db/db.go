package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store represents the database connection and operations
type Store struct {
	db *sql.DB
}

// Model represents a model in the database
type Model struct {
	ID        int
	Slug      string
	ModelID   string
	FileName  string
	FilePath  string
	FileSize  string
	CreatedAt time.Time
	LastUsed  sql.NullTime
}

// New creates a new database connection and initializes the schema
func New(dbPath string) (*Store, error) {
	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	// Create tables if they don't exist
	if err := initSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// initSchema creates the necessary tables
func initSchema(db *sql.DB) error {
	schema := `
    CREATE TABLE IF NOT EXISTS models (
        id INTEGER PRIMARY KEY,
        slug TEXT UNIQUE,
        model_id TEXT,
        file_name TEXT,
        file_path TEXT,
        file_size TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        last_used DATETIME
    );
    `

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("creating schema: %w", err)
	}

	return nil
}

// GetModelBySlug retrieves a model by its slug
func (s *Store) GetModelBySlug(slug string) (*Model, error) {
	query := `SELECT id, slug, model_id, file_name, file_path, file_size, created_at, last_used 
              FROM models WHERE slug = ?`
	
	var model Model
	err := s.db.QueryRow(query, slug).Scan(
		&model.ID, &model.Slug, &model.ModelID, &model.FileName, 
		&model.FilePath, &model.FileSize, &model.CreatedAt, &model.LastUsed,
	)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("model with slug '%s' not found", slug)
	} else if err != nil {
		return nil, fmt.Errorf("querying model: %w", err)
	}
	
	return &model, nil
}

// GetAllModels retrieves all models from the database
func (s *Store) GetAllModels() ([]Model, error) {
	query := `SELECT id, slug, model_id, file_name, file_path, file_size, created_at, last_used 
              FROM models ORDER BY last_used DESC, created_at DESC`
	
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("querying models: %w", err)
	}
	defer rows.Close()
	
	var models []Model
	for rows.Next() {
		var model Model
		if err := rows.Scan(
			&model.ID, &model.Slug, &model.ModelID, &model.FileName, 
			&model.FilePath, &model.FileSize, &model.CreatedAt, &model.LastUsed,
		); err != nil {
			return nil, fmt.Errorf("scanning model row: %w", err)
		}
		models = append(models, model)
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating model rows: %w", err)
	}
	
	return models, nil
}

// UpdateModelLastUsed updates the last_used timestamp for a model
func (s *Store) UpdateModelLastUsed(slug string) error {
	query := `UPDATE models SET last_used = CURRENT_TIMESTAMP WHERE slug = ?`
	
	result, err := s.db.Exec(query, slug)
	if err != nil {
		return fmt.Errorf("updating last used: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("no model with slug '%s' found", slug)
	}
	
	return nil
}

// AddModel adds a new model to the database
func (s *Store) AddModel(slug, modelID, fileName, filePath, fileSize string) error {
	query := `INSERT OR REPLACE INTO models (slug, model_id, file_name, file_path, file_size)
              VALUES (?, ?, ?, ?, ?)`
	
	_, err := s.db.Exec(query, slug, modelID, fileName, filePath, fileSize)
	if err != nil {
		return fmt.Errorf("inserting model: %w", err)
	}
	
	return nil
}

// RemoveModel removes a model from the database
func (s *Store) RemoveModel(slug string) error {
	query := `DELETE FROM models WHERE slug = ?`
	
	result, err := s.db.Exec(query, slug)
	if err != nil {
		return fmt.Errorf("deleting model: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("no model with slug '%s' found", slug)
	}
	
	return nil
}

// UpdateModelSlug updates a model's slug (alias)
func (s *Store) UpdateModelSlug(oldSlug, newSlug string) error {
	query := `UPDATE models SET slug = ? WHERE slug = ?`
	
	result, err := s.db.Exec(query, newSlug, oldSlug)
	if err != nil {
		return fmt.Errorf("updating model slug: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("no model with slug '%s' found", oldSlug)
	}
	
	return nil
}