package main

import "time"

type User struct {
	ID        string    `gorm:"primaryKey;column:id"`
	Name      string    `gorm:"column:name"`
	Email     string    `gorm:"uniqueIndex;column:email"`
	Password  string    `gorm:"column:password"`
	Image     *string   `gorm:"column:image"`
	CreatedAt time.Time `gorm:"column:createdAt"`
	UpdatedAt time.Time `gorm:"column:updatedAt"`

	Meetings []Meetings `gorm:"foreignKey:HostID;references:ID"`
}

type Meetings struct {
	ID        string    `gorm:"primaryKey;column:id"`
	CreatedAt time.Time `gorm:"column:createdAt"`
	HostID    string    `gorm:"column:hostId"`
	Host      User      `gorm:"foreignKey:HostID;references:ID"`
}
