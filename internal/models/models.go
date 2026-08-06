package models

import "time"

type User struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	DisplayName   *string    `json:"display_name"`
	PhotoURL      *string    `json:"photo_url"`
	Plan          string     `json:"plan"`
	ExpiryDate    *time.Time `json:"expiry_date"`
	IsActive      bool       `json:"is_active"`
	DataLimitMB   int        `json:"data_limit_mb"`
	DataUsedMB    int        `json:"data_used_mb"`
	BoundDeviceID string     `json:"bound_device_id"`
	LastLogin     *time.Time `json:"last_login"`
	CreatedAt     time.Time  `json:"created_at"`
}

type Server struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Region      string `json:"region"`
	Flag        string `json:"flag"`
	CountryCode string `json:"country_code"`
	Protocol    string `json:"protocol"`
	Plan        string `json:"plan"`
	IsActive    bool   `json:"is_active"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	RawConfig   string `json:"raw_config"`
	Note        string `json:"note"`
	SortOrder   int    `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Voucher struct {
	ID             string     `json:"id"`
	Code           string     `json:"code"`
	DurationInDays int        `json:"duration_in_days"`
	Status         string     `json:"status"`
	UsedByEmail    *string    `json:"used_by_email"`
	UsedAt         *time.Time `json:"used_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

type Announcement struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
