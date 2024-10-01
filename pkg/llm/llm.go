package llm

import (
	"context"
	"time"
)

type Client interface {
	Generate(ctx context.Context, request GenerateRequest) (*GenerateResponse, error)
}

type GenerateRequest struct {
	Prompt        string        `json:"prompt" yaml:"prompt"`
	Model         string        `json:"model" yaml:"model"`
	MaxRetries    int           `json:"maxRetries" yaml:"maxRetries"`
	RetryCooldown time.Duration `json:"retryCooldown" yaml:"retryCooldown"`
}

type GenerateResponse struct {
	Response string `json:"response" yaml:"response"`
}
