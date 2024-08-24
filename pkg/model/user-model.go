package model

// User model
type User struct {
	ID int64 `json:"id" gorm:"omitempty; primaryKey;"`

	// Basic verification information
	Username string `json:"username" gorm:"omitempty; not null; type:varchar(48); unique;"`
	Email    string `json:"email" gorm:"omitempty; not null; type:varchar(64); unique;"`
	RawEmail string `json:"rawEmail" gorm:"omitempty; not null; type:varchar(64); default:'';"`
	Password string `json:"password" gorm:"omitempty; not null; type:varchar(128); default:'';"`

	// Other identity information
	Nickname      string `json:"nickname" gorm:"omitempty; not null; type:varchar(64); default:'';"`
	Phone         string `json:"phone" gorm:"omitempty; not null; type:varchar(16); default:''; index;"`
	PhoneVerified bool   `json:"phoneVerified" gorm:"omitempty; not null; type:tinyint(1); default:0;"`

	Model
}
