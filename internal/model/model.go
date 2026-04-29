package model

import (
	"encoding/json"
	"time"
)

type Fact struct {
	ID         string          `json:"id"`
	Key        string          `json:"key"`
	Value      json.RawMessage `json:"value"`
	Source     string          `json:"source"`
	SourcePath string          `json:"source_path"`
	SourceHash string          `json:"source_hash"`
	Confidence float64         `json:"confidence"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	ExpiresAt  *time.Time      `json:"expires_at,omitempty"`
	Stale      bool            `json:"stale"`
}

type Source struct {
	ID       string    `json:"id"`
	Root     string    `json:"root"`
	Path     string    `json:"path"`
	AbsPath  string    `json:"abs_path"`
	Hash     string    `json:"hash"`
	Kind     string    `json:"kind"`
	Size     int64     `json:"size"`
	Stale    bool      `json:"stale"`
	Modified time.Time `json:"modified_at"`
}

type Candidate struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Key        string   `json:"key,omitempty"`
	Value      string   `json:"value,omitempty"`
	SourcePath string   `json:"source_path"`
	SourceHash string   `json:"source_hash,omitempty"`
	Text       string   `json:"text,omitempty"`
	Score      float64  `json:"score"`
	Reasons    []string `json:"reasons"`
	Tokens     int      `json:"tokens"`
}

type Task struct {
	Intent string `json:"intent"`
	Query  string `json:"query"`
}

type ContextItem struct {
	ID         string `json:"id"`
	Key        string `json:"key"`
	Value      string `json:"value"`
	SourcePath string `json:"source_path,omitempty"`
	Tokens     int    `json:"tokens,omitempty"`
	Collapsed  bool   `json:"collapsed,omitempty"`
}

type ContextPacket struct {
	Task    Task          `json:"task"`
	Context []ContextItem `json:"context"`
	Meta    PacketMeta    `json:"meta"`
}

type PacketMeta struct {
	TokensUsed   int      `json:"tokens_used"`
	Budget       int      `json:"budget"`
	RulesApplied []string `json:"rules_applied"`
	ContextRef   string   `json:"ctx_ref,omitempty"`
}

type Explanation struct {
	Included  []ExplainEntry `json:"included"`
	Excluded  []ExplainEntry `json:"excluded"`
	Collapsed []ExplainEntry `json:"collapsed"`
	Budget    BudgetReport   `json:"budget"`
}

type ExplainEntry struct {
	ID         string  `json:"id"`
	SourcePath string  `json:"source_path,omitempty"`
	Reason     string  `json:"reason"`
	Score      float64 `json:"score,omitempty"`
	Tokens     int     `json:"tokens,omitempty"`
}

type BudgetReport struct {
	Max  int `json:"max"`
	Used int `json:"used"`
}
