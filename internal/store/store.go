// Package store provides SQLite-backed persistence for cost instances.
package store

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	ErrNotFound      = errors.New("instance not found")
	ErrAlreadyExists = errors.New("instance already exists for this target")
)

type Store struct {
	db *gorm.DB
}

func New(dbPath string) (*Store, error) {
	dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("opening database at %s: %w", dbPath, err)
	}
	if err := db.AutoMigrate(&CostInstance{}); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Create(inst *CostInstance) error {
	err := s.db.Create(inst).Error
	if err != nil && isUniqueConstraintError(err) {
		return ErrAlreadyExists
	}
	return err
}

// ReserveTarget atomically inserts a placeholder row to claim a target
// resource. Returns ErrAlreadyExists if the target is already claimed.
func (s *Store) ReserveTarget(inst *CostInstance) error {
	err := s.db.Create(inst).Error
	if err != nil && isUniqueConstraintError(err) {
		return ErrAlreadyExists
	}
	return err
}

// Update saves all non-zero fields on an existing instance.
func (s *Store) Update(inst *CostInstance) error {
	return s.db.Save(inst).Error
}

func (s *Store) Get(id string) (*CostInstance, error) {
	var inst CostInstance
	err := s.db.First(&inst, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &inst, err
}

func (s *Store) List(limit, offset int) ([]CostInstance, int64, error) {
	var instances []CostInstance
	var total int64
	s.db.Model(&CostInstance{}).Count(&total)
	err := s.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&instances).Error
	return instances, total, err
}

func (s *Store) UpdateStatus(id, status, message string) error {
	result := s.db.Model(&CostInstance{}).Where("id = ?", id).
		Updates(map[string]any{"status": status, "status_message": message})
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return result.Error
}

func (s *Store) Delete(id string) error {
	result := s.db.Delete(&CostInstance{}, "id = ?", id)
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return result.Error
}

func (s *Store) ListByStatus(status string) ([]CostInstance, error) {
	var instances []CostInstance
	err := s.db.Where("status = ?", status).Find(&instances).Error
	return instances, err
}

func isUniqueConstraintError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique_violation")
}
