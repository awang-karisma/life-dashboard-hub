package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/owen282000/life-dashboard-hub/internal/config"
	"golang.org/x/oauth2"
)

type OIDCClient struct {
	Enabled      bool
	DisplayName  string
	IssuerURL    string
	ClientID     string
	OAuth2Config *oauth2.Config
	Verifier     *oidc.IDTokenVerifier
	ExchangeFunc func(ctx context.Context, code string) (*OIDCClaims, error)
}

type OIDCClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func NewOIDCClient(ctx context.Context, cfg *config.Config) *OIDCClient {
	if cfg.OIDCIssuerURL == "" {
		return &OIDCClient{
			Enabled:     false,
			DisplayName: cfg.OIDCDisplayName,
		}
	}

	provider, err := oidc.NewProvider(ctx, cfg.OIDCIssuerURL)
	if err != nil {
		slog.Error("Failed to initialize OIDC provider; continuing with OIDC disabled", "issuer", cfg.OIDCIssuerURL, "error", err)
		return &OIDCClient{
			Enabled:     false,
			DisplayName: cfg.OIDCDisplayName,
		}
	}

	scopes := strings.Split(cfg.OIDCScopes, " ")
	if len(scopes) == 0 || scopes[0] == "" {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}

	redirectURL := strings.TrimRight(cfg.PublicURL, "/") + "/api/v1/auth/oidc/callback"

	oauth2Config := &oauth2.Config{
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: cfg.OIDCClientID,
	})

	slog.Info("OIDC provider initialized successfully", "issuer", cfg.OIDCIssuerURL)

	return &OIDCClient{
		Enabled:      true,
		DisplayName:  cfg.OIDCDisplayName,
		IssuerURL:    cfg.OIDCIssuerURL,
		ClientID:     cfg.OIDCClientID,
		OAuth2Config: oauth2Config,
		Verifier:     verifier,
	}
}

func (c *OIDCClient) AuthCodeURL(state string) string {
	if !c.Enabled || c.OAuth2Config == nil {
		return ""
	}
	return c.OAuth2Config.AuthCodeURL(state)
}

func (c *OIDCClient) ExchangeAndVerify(ctx context.Context, code string) (*OIDCClaims, error) {
	if c.ExchangeFunc != nil {
		return c.ExchangeFunc(ctx, code)
	}
	if !c.Enabled {
		return nil, fmt.Errorf("OIDC is not enabled")
	}

	token, err := c.OAuth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth2 exchange failed: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, fmt.Errorf("missing id_token in token response")
	}

	idToken, err := c.Verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify id_token: %w", err)
	}

	var claims OIDCClaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse id_token claims: %w", err)
	}

	return &claims, nil
}
