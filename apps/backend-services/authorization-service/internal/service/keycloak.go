package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/002aic/authorization-service/internal/config"
	"go.uber.org/zap"
)

type KeycloakService struct {
	config config.KeycloakConfig
	logger *zap.Logger
	client *http.Client
}

type KeycloakUser struct {
	ID         string              `json:"id"`
	Username   string              `json:"username"`
	Email      string              `json:"email"`
	FirstName  string              `json:"firstName"`
	LastName   string              `json:"lastName"`
	Enabled    bool                `json:"enabled"`
	Attributes map[string][]string `json:"attributes,omitempty"`
}

type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	NotBeforePolicy  int    `json:"not-before-policy"`
	SessionState     string `json:"session_state"`
	Scope            string `json:"scope"`
}

func NewKeycloakService(config config.KeycloakConfig, logger *zap.Logger) *KeycloakService {
	return &KeycloakService{
		config: config,
		logger: logger,
		client: &http.Client{},
	}
}

func (s *KeycloakService) GetAdminToken() (string, error) {
	tokenURL := fmt.Sprintf("%s/realms/master/protocol/openid-connect/token", s.config.BaseURL)
	
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", s.config.ClientID)
	data.Set("client_secret", s.config.ClientSecret)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get admin token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get admin token, status: %d, body: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	return tokenResp.AccessToken, nil
}

func (s *KeycloakService) GetUser(userID string) (*KeycloakUser, error) {
	token, err := s.GetAdminToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get admin token: %w", err)
	}

	userURL := fmt.Sprintf("%s/admin/realms/%s/users/%s", s.config.BaseURL, s.config.Realm, userID)
	
	req, err := http.NewRequest("GET", userURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user, status: %d, body: %s", resp.StatusCode, string(body))
	}

	var user KeycloakUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user response: %w", err)
	}

	return &user, nil
}

func (s *KeycloakService) GetUserByUsername(username string) (*KeycloakUser, error) {
	token, err := s.GetAdminToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get admin token: %w", err)
	}

	usersURL := fmt.Sprintf("%s/admin/realms/%s/users?username=%s", s.config.BaseURL, s.config.Realm, username)
	
	req, err := http.NewRequest("GET", usersURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create users request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get users, status: %d, body: %s", resp.StatusCode, string(body))
	}

	var users []KeycloakUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("failed to decode users response: %w", err)
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return &users[0], nil
}

func (s *KeycloakService) CreateUser(user *KeycloakUser) error {
	token, err := s.GetAdminToken()
	if err != nil {
		return fmt.Errorf("failed to get admin token: %w", err)
	}

	usersURL := fmt.Sprintf("%s/admin/realms/%s/users", s.config.BaseURL, s.config.Realm)
	
	userJSON, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	req, err := http.NewRequest("POST", usersURL, bytes.NewBuffer(userJSON))
	if err != nil {
		return fmt.Errorf("failed to create user request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create user, status: %d, body: %s", resp.StatusCode, string(body))
	}

	s.logger.Info("User created successfully", zap.String("username", user.Username))
	return nil
}
