package main

import (
	"errors"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	dsn := os.Getenv("DATABASE_URL")
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	log.Println("Database connection established")
}

func (Meetings) TableName() string {
	return "Meetings"
}

func FindMeetingByID(meetingID string) (string, error) {
	var meeting struct {
		HostID string `gorm:"column:hostId"`
	}

	err := DB.Model(&Meetings{}).
		Select(`"hostId"`).
		Where("id = ?", meetingID).
		Take(&meeting).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errors.New("meeting not found")
		}
		return "", err
	}

	return meeting.HostID, nil
}
