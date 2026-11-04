package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SupabaseAdminRevoker permanently deletes the hosted identity before the
// BodySense privacy worker removes local health data.
type SupabaseAdminRevoker struct {
	config SupabaseConfig
	client *http.Client
}

func NewSupabaseAdminRevoker(config SupabaseConfig, client *http.Client) *SupabaseAdminRevoker {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return &SupabaseAdminRevoker{config: config, client: client}
}

func (a *SupabaseAdminRevoker) Ready() error {
	if a.config.SecretKey == "" {
		return errors.New("SUPABASE_SECRET_KEY is required for permanent account deletion")
	}
	return nil
}

func (a *SupabaseAdminRevoker) RevokeUser(ctx context.Context, userID uuid.UUID) error {
	if err := a.Ready(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		strings.TrimRight(a.config.URL, "/")+"/auth/v1/admin/users/"+userID.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("apikey", a.config.SecretKey)
	req.Header.Set("Authorization", "Bearer "+a.config.SecretKey)
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("delete Supabase identity: %w", err)
	}
	defer resp.Body.Close()
	// A retry may observe 404 if a previous attempt deleted the identity.
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusAccepted {
		return nil
	}
	return fmt.Errorf("Supabase identity deletion failed with HTTP %d", resp.StatusCode)
}
