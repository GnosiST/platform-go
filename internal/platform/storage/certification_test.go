package storage

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type certificationRecord struct {
	ID       uint64 `gorm:"primaryKey"`
	Code     string `gorm:"size:64;uniqueIndex;not null"`
	SortRank int    `gorm:"not null;index"`
}

func TestGORMCertificationSmoke(t *testing.T) {
	for _, testCase := range []struct {
		driver string
		env    string
	}{
		{driver: "mysql", env: "PLATFORM_CERTIFICATION_MYSQL_DSN"},
		{driver: "postgres", env: "PLATFORM_CERTIFICATION_POSTGRES_DSN"},
	} {
		t.Run(testCase.driver, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(testCase.env))
			if dsn == "" {
				t.Skipf("%s is not configured", testCase.env)
			}
			db, err := OpenGORM(Config{Driver: testCase.driver, DSN: dsn})
			if err != nil {
				t.Fatalf("OpenGORM() error = %v", err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			if err := db.AutoMigrate(&certificationRecord{}); err != nil {
				t.Fatalf("AutoMigrate() error = %v", err)
			}
			prefix := "certification-" + testCase.driver + "-"
			committedCode := prefix + "committed"
			abortedCode := prefix + "aborted"
			if err := db.Where("code LIKE ?", prefix+"%").Delete(&certificationRecord{}).Error; err != nil {
				t.Fatalf("Delete(previous records) error = %v", err)
			}
			if err := db.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
				return tx.Create(&certificationRecord{Code: committedCode, SortRank: 1}).Error
			}); err != nil {
				t.Fatalf("Transaction(create) error = %v", err)
			}
			if err := db.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
				if err := tx.Create(&certificationRecord{Code: abortedCode, SortRank: 2}).Error; err != nil {
					return err
				}
				return errCertificationRollback
			}); err == nil || !strings.Contains(err.Error(), errCertificationRollback.Error()) {
				t.Fatalf("Transaction(rollback) error = %v", err)
			}
			var count int64
			if err := db.Model(&certificationRecord{}).Where("code = ?", abortedCode).Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("rolled-back record count = %d, %v", count, err)
			}
			if err := db.Create([]certificationRecord{{Code: prefix + "page-1", SortRank: 3}, {Code: prefix + "page-2", SortRank: 4}, {Code: prefix + "page-3", SortRank: 5}}).Error; err != nil {
				t.Fatalf("Create(pagination records) error = %v", err)
			}
			var page []certificationRecord
			if err := db.Where("code LIKE ?", prefix+"%").Order("sort_rank ASC").Offset(1).Limit(2).Find(&page).Error; err != nil || len(page) != 2 || page[0].Code != prefix+"page-1" || page[1].Code != prefix+"page-2" {
				t.Fatalf("pagination result = %+v, %v", page, err)
			}
			if err := db.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
				var locked certificationRecord
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", committedCode).Take(&locked).Error; err != nil {
					return err
				}
				return tx.Model(&locked).Update("sort_rank", 10).Error
			}); err != nil {
				t.Fatalf("Transaction(lock and update) error = %v", err)
			}
		})
	}
}

var errCertificationRollback = errors.New("certification rollback")
