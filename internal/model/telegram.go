package model

type TelegramSubscription struct {
	ID     uint   `gorm:"primarykey"`
	ChatID *int64 `gorm:"not null"`
	UserID *uint  `gorm:"not null"`
	User   *User
	// we use a 16-bit permission mask for all subscriptions
	Mask uint16 `gorm:"not null"`
	DBTime
}
