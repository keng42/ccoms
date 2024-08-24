// Package model defines the database models, keeping mysql and redis connection instances.
package model

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"time"
)

type Model struct {
	Status    int8      `json:"status" gorm:"omitempty; not null; type:tinyint; default:1;"`
	CreatedAt time.Time `json:"createdAt" gorm:"omitempty; not null; default:CURRENT_TIMESTAMP(3);"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"omitempty; not null; default:CURRENT_TIMESTAMP(3);"`
}

// GormMap is a gorm customer datatype, for storing string arrays in mysql using json
type GormMap map[string]interface{}

func (a GormMap) Value() (driver.Value, error) {
	b, err := json.Marshal(a)
	return string(b), err
}

func (a *GormMap) Scan(input interface{}) error {
	return json.Unmarshal(input.([]byte), a)
}

func (a GormMap) GormDataType() string {
	return "json"
}

func (a GormMap) V() map[string]interface{} {
	return map[string]interface{}(a)
}

// GormArray is a gorm customer datatype, for storing string arrays in mysql using json
type GormArray []string

func (a GormArray) Value() (driver.Value, error) {
	b, err := json.Marshal(a)
	return string(b), err
}

func (a *GormArray) Scan(input interface{}) error {
	return json.Unmarshal(input.([]byte), a)
}

func (a GormArray) GormDataType() string {
	return "json"
}

func (a GormArray) Array() []string {
	return []string(a)
}

// GormTime is a gorm customer datatype, for solving mysql's NO_ZERO_DATE problem
// Incorrect datetime value: '1000-01-01 08:00:00.000+08:00' in mysql 8.0 default config.
type GormTime time.Time

func (t GormTime) Value() (driver.Value, error) {
	tt := time.Time(t)
	if tt.IsZero() {
		return "1000-01-01 08:00:00.000", nil
	}
	return tt.Format("2006-01-02 15:04:05.999"), nil
}

func (t *GormTime) Scan(value interface{}) error {
	nullTime := &sql.NullTime{}
	err := nullTime.Scan(value)
	*t = GormTime(nullTime.Time)
	return err
}

func (t GormTime) GormDataType() string {
	return "datetime(3)"
}

func (t GormTime) String() string {
	return t.Time().String()
}

func (t GormTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Time())
}

func (t GormTime) Time() time.Time {
	return time.Time(t)
}
