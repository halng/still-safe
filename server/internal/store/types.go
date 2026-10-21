package store

import "time"

type User struct {
	ID             string
	DisplayName    string
	PassphraseHash string
	IDOnFile       bool
}

type Identity struct {
	LegalName     string
	DateOfBirth   time.Time
	DocumentType  string
	DocumentLast4 string
	Status        string
}

type Circle struct {
	SupervisorLimit   int
	SharingEnabled    bool
	ActiveSupervisors int
}

type Supervisor struct {
	LinkID      string
	UserID      string
	DisplayName string
}

type Location struct {
	Latitude   float64
	Longitude  float64
	AccuracyM  float64
	RecordedAt time.Time
}

type Place struct {
	ID        string
	Label     string
	Latitude  float64
	Longitude float64
}

type Check struct {
	ID              string
	PersonID        string
	PersonName      string
	StartedBy       string
	StartedByName   string
	Reason          string
	Status          string
	StartedAt       time.Time
	RespondBy       time.Time
	ResolvedAt      *time.Time
	ConfirmedByName string
}

type Watched struct {
	PersonID       string
	DisplayName    string
	SharingEnabled bool
	LastLocation   *Location
	Check          *Check
}
