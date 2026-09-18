package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// FaceVerification stores the *result* of a verification check, never
// raw camera footage. This is IDENTITY DATA and is access-controlled
// separately from exam/AI event data.
type FaceVerification struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	StudentID   uuid.UUID `gorm:"type:uuid;index;not null" json:"student_id"`
	ExamID      uuid.UUID `gorm:"type:uuid;index" json:"exam_id,omitempty"`
	AttemptID   *uuid.UUID `gorm:"type:uuid;index" json:"attempt_id,omitempty"`
	Matched     bool      `json:"matched"`
	Confidence  float64   `json:"confidence"`
	IsReference bool      `gorm:"default:false" json:"is_reference"` // true = registration of reference face
	CreatedAt   time.Time `json:"created_at"`
}

func (f *FaceVerification) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}

// MonitoringSession represents one continuous camera/connection session
// for a student during an active attempt. Used to derive camera/network
// status shown in the live dashboard.
type MonitoringSession struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	AttemptID   uuid.UUID  `gorm:"type:uuid;index;not null" json:"attempt_id"`
	StudentID   uuid.UUID  `gorm:"type:uuid;index;not null" json:"student_id"`
	ExamID      uuid.UUID  `gorm:"type:uuid;index;not null" json:"exam_id"`
	CameraActive bool      `gorm:"default:false" json:"camera_active"`
	ConnectionQuality string `gorm:"default:'UNKNOWN'" json:"connection_quality"` // GOOD/POOR/DISCONNECTED
	StartedAt   time.Time  `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
}

func (m *MonitoringSession) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityLow      Severity = "LOW"
	SeverityMedium   Severity = "MEDIUM"
	SeverityHigh     Severity = "HIGH"
	SeverityCritical Severity = "CRITICAL"
)

// AIEvent is a structured detection event produced by the Python AI
// service and relayed through Go. AI events NEVER carry an automatic
// "guilty" verdict — only a classified severity for human review.
type AIEvent struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	AttemptID   uuid.UUID `gorm:"type:uuid;index;not null" json:"attempt_id"`
	StudentID   uuid.UUID `gorm:"type:uuid;index;not null" json:"student_id"`
	ExamID      uuid.UUID `gorm:"type:uuid;index;not null" json:"exam_id"`
	EventType   string    `gorm:"not null;index" json:"event_type"`
	Severity    Severity  `gorm:"not null;index" json:"severity"`
	Confidence  float64   `json:"confidence"`
	Description string    `json:"description"`
	Reviewed    bool      `gorm:"default:false" json:"reviewed"`
	ReviewedBy  *uuid.UUID `gorm:"type:uuid" json:"reviewed_by,omitempty"`
	Timestamp   time.Time `gorm:"index" json:"timestamp"`
}

func (e *AIEvent) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

// SecurityEvent captures non-AI security-relevant occurrences, such as
// suspicious session behavior, unauthorized access attempts, or
// duplicate-submission attempts blocked server-side.
type SecurityEvent struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      *uuid.UUID `gorm:"type:uuid;index" json:"user_id,omitempty"`
	ExamID      *uuid.UUID `gorm:"type:uuid;index" json:"exam_id,omitempty"`
	EventType   string    `gorm:"not null" json:"event_type"`
	Severity    Severity  `gorm:"not null" json:"severity"`
	Description string    `json:"description"`
	IPAddress   string    `json:"ip_address,omitempty"`
	Timestamp   time.Time `gorm:"index" json:"timestamp"`
}

func (e *SecurityEvent) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

// AuditLog records every significant actor action across the platform
// for compliance and forensic review.
type AuditLog struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	ActorID    uuid.UUID `gorm:"type:uuid;index" json:"actor_id"`
	ActorRole  Role      `json:"actor_role"`
	Action     string    `gorm:"not null;index" json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resource_id,omitempty"`
	IPAddress  string    `json:"ip_address,omitempty"`
	Metadata   string    `json:"metadata,omitempty"` // JSON-encoded free-form context
	Timestamp  time.Time `gorm:"index" json:"timestamp"`
}

func (a *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
