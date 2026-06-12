package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edivilsondalacosta/feature-voting-system/internal/service"
)

type mockRequestRepo struct {
	insertFn func(ctx context.Context, authorID, title, description string) (*service.FeatureRequest, error)
	listNewFn func(ctx context.Context, params service.ListParams) ([]service.FeatureRequest, error)
	listTopFn func(ctx context.Context, params service.ListParams) ([]service.FeatureRequest, error)
	getFn     func(ctx context.Context, id, viewerID string) (*service.FeatureRequest, error)
}

func (m *mockRequestRepo) InsertFeatureRequest(ctx context.Context, authorID, title, description string) (*service.FeatureRequest, error) {
	return m.insertFn(ctx, authorID, title, description)
}
func (m *mockRequestRepo) ListRequestsByNew(ctx context.Context, params service.ListParams) ([]service.FeatureRequest, error) {
	return m.listNewFn(ctx, params)
}
func (m *mockRequestRepo) ListRequestsByTop(ctx context.Context, params service.ListParams) ([]service.FeatureRequest, error) {
	return m.listTopFn(ctx, params)
}
func (m *mockRequestRepo) GetFeatureRequest(ctx context.Context, id, viewerID string) (*service.FeatureRequest, error) {
	return m.getFn(ctx, id, viewerID)
}

func TestRequestSubmit_Validation(t *testing.T) {
	repo := &mockRequestRepo{
		insertFn: func(_ context.Context, _, _, _ string) (*service.FeatureRequest, error) {
			return &service.FeatureRequest{}, nil
		},
	}
	svc := service.NewRequestService(repo, nil)

	tests := []struct {
		name        string
		title       string
		description string
		wantErr     bool
		errMsg      string
	}{
		{"valid", "My feature", "A description", false, ""},
		{"empty title", "", "A description", true, "title is required"},
		{"whitespace title", "   ", "A description", true, "title is required"},
		{"title too long", string(make([]byte, 101)), "A description", true, "100 characters"},
		{"empty description", "My feature", "", true, "description is required"},
		{"whitespace description", "My feature", "   ", true, "description is required"},
		{"description too long", "My feature", string(make([]byte, 5001)), true, "5000 characters"},
		{"trims whitespace", " My feature ", " A description ", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Submit(context.Background(), "user-id", tt.title, tt.description)
			if tt.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, service.ErrValidation), "expected ErrValidation, got %v", err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
