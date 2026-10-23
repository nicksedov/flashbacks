package repository

import (
	"github.com/flashbacks/core/internal/domain"
	"gorm.io/gorm"
)

// ScheduleRepository provides data access for the unified schedules table.
type ScheduleRepository interface {
	// Get returns the schedule of a job kind. It returns gorm.ErrRecordNotFound
	// when the kind has no stored schedule yet.
	Get(kind domain.ScheduleKind) (*domain.Schedule, error)
	// GetAll returns every stored schedule ordered by kind.
	GetAll() ([]domain.Schedule, error)
	// Upsert creates or updates the schedule of its kind.
	Upsert(schedule *domain.Schedule) error
}

type gormScheduleRepo struct {
	db *gorm.DB
}

// NewScheduleRepository creates a GORM-backed ScheduleRepository.
func NewScheduleRepository(db *gorm.DB) ScheduleRepository {
	return &gormScheduleRepo{db: db}
}

func (r *gormScheduleRepo) Get(kind domain.ScheduleKind) (*domain.Schedule, error) {
	var schedule domain.Schedule
	if err := r.db.Where("kind = ?", string(kind)).First(&schedule).Error; err != nil {
		return nil, err
	}
	return &schedule, nil
}

func (r *gormScheduleRepo) GetAll() ([]domain.Schedule, error) {
	var schedules []domain.Schedule
	if err := r.db.Order("kind").Find(&schedules).Error; err != nil {
		return nil, err
	}
	return schedules, nil
}

func (r *gormScheduleRepo) Upsert(schedule *domain.Schedule) error {
	var existing domain.Schedule
	err := r.db.Where("kind = ?", schedule.Kind).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(schedule).Error
	}
	if err != nil {
		return err
	}

	// Update through a map so zero values (e.g. an empty weekday set, hour 0)
	// are persisted instead of being skipped as struct zero values.
	return r.db.Model(&existing).Updates(map[string]interface{}{
		"enabled":         schedule.Enabled,
		"days":            schedule.Days,
		"start_hour":      schedule.StartHour,
		"start_minute":    schedule.StartMinute,
		"end_hour":        schedule.EndHour,
		"end_minute":      schedule.EndMinute,
		"timezone_offset": schedule.TimezoneOffset,
	}).Error
}
