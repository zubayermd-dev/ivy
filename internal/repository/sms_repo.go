package repository

import (
	"errors"
	"time"

	"github.com/zubayermd-dev/ivy/internal/model"
	"gorm.io/gorm"
)

var ErrDuplicate = errors.New("duplicate message")

type SMSRepository struct {
	db *gorm.DB
}

func NewSMSRepository(db *gorm.DB) *SMSRepository {
	return &SMSRepository{db: db}
}

func (r *SMSRepository) Create(sms *model.SMS) error {
	// Deduplication: skip if same phone + content exists within 1 minute
	// Use simple time comparison that works on both SQLite and MySQL
	since := time.Now().Add(-1 * time.Minute)
	var count int64
	r.db.Model(&model.SMS{}).
		Where("phone = ? AND content = ? AND created_at > ?",
			sms.Phone, sms.Content, since).
		Count(&count)
	if count > 0 {
		return ErrDuplicate // Duplicate, skip
	}
	return r.db.Create(sms).Error
}

func (r *SMSRepository) FindByICCID(iccid string) ([]model.SMS, error) {
	var smsList []model.SMS
	err := r.db.Where("iccid = ?", iccid).Order("timestamp desc").Find(&smsList).Error
	return smsList, err
}
