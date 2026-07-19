package repository

import (
	"crypto/md5"
	"errors"
	"fmt"
	"time"

	"github.com/zubayermd-dev/ivy/internal/model"
	"github.com/zubayermd-dev/ivy/pkg/logger"
	"gorm.io/gorm"
)

var ErrDuplicate = errors.New("duplicate message")

type SMSRepository struct {
	db *gorm.DB
}

func NewSMSRepository(db *gorm.DB) *SMSRepository {
	return &SMSRepository{db: db}
}

// generateHash creates a unique fingerprint for SMS deduplication
// Uses sender + SMSC timestamp + message body
func generateHash(phone, content string, timestamp time.Time) string {
	data := fmt.Sprintf("%s|%s|%s", phone, content, timestamp.Format(time.RFC3339))
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

func (r *SMSRepository) Create(sms *model.SMS) error {
	// Deduplication using hash of sender + timestamp + content
	// Check within 48 hours (carrier may re-deliver old messages)
	hash := generateHash(sms.Phone, sms.Content, sms.Timestamp)

	return r.db.Transaction(func(tx *gorm.DB) error {
		// Check for duplicate hash within 48 hours
		since := time.Now().Add(-48 * time.Hour)
		var count int64
		tx.Model(&model.SMS{}).
			Where("iccid = ? AND created_at > ?", sms.ICCID, since).
			Count(&count)

		// Manual hash check since we don't have a hash column yet
		// Check by phone + content + timestamp proximity
		var existing model.SMS
		err := tx.Where("phone = ? AND content = ? AND iccid = ?",
			sms.Phone, sms.Content, sms.ICCID).
			Order("created_at DESC").
			First(&existing).Error

		if err == nil {
			// Check if timestamps are within 48 hours
			diff := sms.Timestamp.Sub(existing.Timestamp)
			if diff < 0 {
				diff = -diff
			}
			if diff < 48*time.Hour {
				logger.Log.Infof("[Dedup] Skipped duplicate SMS from %s: %s (hash: %s)", sms.Phone, sms.Content, hash)
				return ErrDuplicate
			}
		}

		return tx.Create(sms).Error
	})
}

func (r *SMSRepository) FindByICCID(iccid string) ([]model.SMS, error) {
	var smsList []model.SMS
	err := r.db.Where("iccid = ?", iccid).Order("timestamp desc").Find(&smsList).Error
	return smsList, err
}

// DeleteOlderThan deletes messages received after the given time
// Used to clean up carrier re-deliveries on startup
func (r *SMSRepository) DeleteOlderThan(since time.Time) int64 {
	result := r.db.Where("created_at > ?", since).Delete(&model.SMS{})
	return result.RowsAffected
}
