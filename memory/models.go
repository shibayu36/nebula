package memory

import "time"

// Session represents a conversation session
type Session struct {
	ID          string     `json:"id"`
	StartedAt   time.Time  `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
	ProjectPath string     `json:"project_path"`
	ModelUsed   string     `json:"model_used"`
}

// Message represents a single message in the conversation
type Message struct {
	ID          int       `json:"id"`
	SessionID   string    `json:"session_id"`
	Timestamp   time.Time `json:"timestamp"`
	Role        string    `json:"role"` // user, assistant, tool
	Content     string    `json:"content"`
	ToolCalls   *string   `json:"tool_calls,omitempty"`
	ToolResults *string   `json:"tool_results,omitempty"`
}

// SessionSummary represents a brief summary of a session for listing
type SessionSummary struct {
	ID           string     `json:"id"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
	ProjectPath  string     `json:"project_path"`
	ModelUsed    string     `json:"model_used"`
	MessageCount int        `json:"message_count"`
	LastMessage  string     `json:"last_message"`
}

func (s *Session) IsActive() bool {
	return s.EndedAt == nil
}

func (s *Session) Duration() time.Duration {
	if s.EndedAt == nil {
		return time.Since(s.StartedAt)
	}
	return s.EndedAt.Sub(s.StartedAt)
}
