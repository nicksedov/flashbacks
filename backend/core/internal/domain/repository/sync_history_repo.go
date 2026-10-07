package repository

import (
	"time"

	"github.com/flashbacks/api-service/internal/domain"
	"gorm.io/gorm"
)

// SyncHistoryRepository provides data access for the sync_history table.
type SyncHistoryRepository interface {
	// FindByDateRange returns sync history entries within the specified time range, ordered by created_at DESC.
	FindByDateRange(from, to time.Time) ([]domain.SyncHistory, error)
	// Create inserts a new sync history record.
	Create(entry *domain.SyncHistory) error
}

type gormSyncHistoryRepo struct {
	db *gorm.DB
}

// NewSyncHistoryRepository creates a GORM-backed SyncHistoryRepository.
func NewSyncHistoryRepository(db *gorm.DB) SyncHistoryRepository {
	return &gormSyncHistoryRepo{db: db}
}

func (r *gormSyncHistoryRepo) FindByDateRange(from, to time.Time) ([]domain.SyncHistory, error) {
	var entries []domain.SyncHistory
	if err := r.db.Where("created_at >= ? AND created_at <= ?", from, to).
		Order("created_at DESC").
		Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *gormSyncHistoryRepo) Create(entry *domain.SyncHistory) error {
	return r.db.Create(entry).Error
}
