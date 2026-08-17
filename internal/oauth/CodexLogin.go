package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"orbit/internal/config"
	"strings"
	"time"
)

const (
	authURL     = "https://auth.openai.com/oauth/authorize"
	tokenURL    = "https://auth.openai.com/oauth/token"
	clientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	redirectURI = "http://localhost:1455/auth/callback"
	listenAddr  = "localhost:1455"
)

type pkceCodes struct {
	CodeVerifier  string
	CodeChallenge string
}

type oauthCallback struct {
	Code             string
	State            string
	Error            string
	ErrorDescription string
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type CodexLoginSession struct {
	AuthorizationURL string
	pkce             *pkceCodes
	state            string
	server           *http.Server
	callbackCh       <-chan oauthCallback
	serverErrCh      <-chan error
}

func PrepareCodexLogin() (*CodexLoginSession, error) {
	pkce, err := generatePKCECodes()
	if err != nil {
		return nil, fmt.Errorf("generate PKCE codes: %w", err)
	}

	state, err := randomURLSafeString(32)
	if err != nil {
		return nil, fmt.Errorf("generate OAuth state: %w", err)
	}

	server, callbackCh, serverErrCh, err := startCallbackServer()
	if err != nil {
		return nil, fmt.Errorf("start OAuth callback server: %w", err)
	}

	authorizationURL, err := generateAuthURL(state, pkce)
	if err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return nil, fmt.Errorf("generate authorization URL: %w", err)
	}

	return &CodexLoginSession{
		AuthorizationURL: authorizationURL,
		pkce:             pkce,
		state:            state,
		server:           server,
		callbackCh:       callbackCh,
		serverErrCh:      serverErrCh,
	}, nil
}

func (s *CodexLoginSession) wait(ctx context.Context) (*tokenResponse, error) {
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
	}()

	callback, err := waitForCallback(ctx, s.callbackCh, s.serverErrCh, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("wait for OAuth callback: %w", err)
	}

	if callback.Error != "" {
		if callback.ErrorDescription != "" {
			return nil, fmt.Errorf("OAuth authorization: %s: %s", callback.Error, callback.ErrorDescription)
		}
		return nil, fmt.Errorf("OAuth authorization: %s", callback.Error)
	}

	if callback.Code == "" {
		return nil, errors.New("OAuth callback: authorization code is empty")
	}

	if callback.State != s.state {
		return nil, errors.New("OAuth callback: state mismatch; refusing to exchange the authorization code")
	}

	exchangeCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	tokens, err := exchangeCodeForTokens(exchangeCtx, callback.Code, s.pkce)
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code for tokens: %w", err)
	}

	return tokens, nil
}

func (s *CodexLoginSession) Config(ctx context.Context) (config.ProviderConfig, error) {
	tokens, err := s.wait(ctx)
	if err != nil {
		return config.ProviderConfig{}, err
	}
	return codexProviderConfig(
		tokens.IDToken,
		tokens.AccessToken,
		tokens.RefreshToken,
		accountIDFromIDToken(tokens.IDToken),
	), nil
}

func CodexLogin(ctx context.Context) (*tokenResponse, error) {
	session, err := PrepareCodexLogin()
	if err != nil {
		return nil, err
	}
	return session.wait(ctx)
}

func CodexLoginConfig(ctx context.Context) (config.ProviderConfig, error) {
	tokens, err := CodexLogin(ctx)
	if err != nil {
		return config.ProviderConfig{}, err
	}
	return codexProviderConfig(
		tokens.IDToken,
		tokens.AccessToken,
		tokens.RefreshToken,
		accountIDFromIDToken(tokens.IDToken),
	), nil
}

func accountIDFromIDToken(idToken string) string {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	for _, key := range []string{"chatgpt_account_id", "account_id"} {
		if accountID, ok := claims[key].(string); ok {
			return accountID
		}
	}
	if authClaims, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
		if accountID, ok := authClaims["chatgpt_account_id"].(string); ok {
			return accountID
		}
	}
	return ""
}

func generatePKCECodes() (*pkceCodes, error) {
	verifier, err := randomURLSafeString(96)
	if err != nil {
		return nil, fmt.Errorf("generate code verifier: %w", err)
	}

	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return &pkceCodes{
		CodeVerifier:  verifier,
		CodeChallenge: challenge,
	}, nil
}

func randomURLSafeString(size int) (string, error) {
	randomBytes := make([]byte, size)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("read secure random bytes: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

func generateAuthURL(state string, pkce *pkceCodes) (string, error) {
	if pkce == nil {
		return "", errors.New("PKCE codes are required")
	}

	if strings.TrimSpace(state) == "" {
		return "", errors.New("OAuth state is required")
	}

	params := url.Values{
		"client_id":                  {clientID},
		"response_type":              {"code"},
		"redirect_uri":               {redirectURI},
		"scope":                      {"openid email profile offline_access"},
		"state":                      {state},
		"code_challenge":             {pkce.CodeChallenge},
		"code_challenge_method":      {"S256"},
		"prompt":                     {"login"},
		"id_token_add_organizations": {"true"},
		"codex_cli_simplified_flow":  {"true"},
	}

	return authURL + "?" + params.Encode(), nil
}

func startCallbackServer() (
	*http.Server,
	<-chan oauthCallback,
	<-chan error,
	error,
) {
	callbackCh := make(chan oauthCallback, 1)
	serverErrCh := make(chan error, 1)

	mux := http.NewServeMux()

	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		query := r.URL.Query()

		result := oauthCallback{
			Code:             query.Get("code"),
			State:            query.Get("state"),
			Error:            query.Get("error"),
			ErrorDescription: query.Get("error_description"),
		}

		select {
		case callbackCh <- result:
		default:
			// Ignore duplicate callbacks after the first one.
		}

		if result.Error != "" {
			http.Error(w, "OAuth authorization failed", http.StatusBadRequest)
			return
		}

		if result.Code == "" {
			http.Error(w, "authorization code is missing", http.StatusBadRequest)
			return
		}

		if result.State == "" {
			http.Error(w, "state is missing", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "Codex authenticated successfully. You can close this window.\n")
	})

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf(
			"listen on %s: %w",
			listenAddr,
			err,
		)
	}

	server := &http.Server{
		Handler: mux,
	}

	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case serverErrCh <- err:
			default:
			}
		}
	}()

	return server, callbackCh, serverErrCh, nil
}

func waitForCallback(
	ctx context.Context,
	callbackCh <-chan oauthCallback,
	serverErrCh <-chan error,
	timeout time.Duration,
) (oauthCallback, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case callback := <-callbackCh:
		return callback, nil

	case err := <-serverErrCh:
		return oauthCallback{}, fmt.Errorf("callback server failed: %w", err)

	case <-ctx.Done():
		return oauthCallback{}, ctx.Err()

	case <-timer.C:
		return oauthCallback{}, errors.New("timeout waiting for OAuth callback")
	}
}

func exchangeCodeForTokens(
	ctx context.Context,
	code string,
	pkce *pkceCodes,
) (*tokenResponse, error) {
	if pkce == nil {
		return nil, errors.New("PKCE codes are required")
	}

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {pkce.CodeVerifier},
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		tokenURL,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}

	req.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)
	req.Header.Set("Accept", "application/json")

	// No request timeout is set here. The context controls the
	// credential-acquisition timeout.
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send token request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"token exchange failed with status %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var tokens tokenResponse
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	if tokens.AccessToken == "" {
		return nil, errors.New("token response does not contain access_token")
	}

	return &tokens, nil
}
