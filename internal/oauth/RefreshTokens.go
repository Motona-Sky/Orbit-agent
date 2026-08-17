package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"orbit/internal/config"
	"orbit/internal/utils"
	"strings"
	"time"

	"github.com/imroc/req/v3"
)

type RefreshTokensRequest struct {
	ClientID     string `url:"client_id"`
	GrantType    string `url:"grant_type"`
	Scope        string `url:"scope"`
	RefreshToken string `url:"refresh_token"`
}
type ReReqToken struct {
	AccessToken string `json:"access_token"`
	// TokenType         string `json:"-"`
	// ExpiresIn         int64  `json:"-"`
	// Scope             string `json:"-"`
	IDToken string `json:"id_token"`
	// EarliestRefreshAt string `json:"earliest_refresh_at"`
	RefreshToken string `json:"refresh_token"`
	// OaiIs             string `json:"-"`
}

func (r *ReReqToken) GetAccessToken() string {
	return r.AccessToken
}
func RefreshTokens() (error, string) {
	provider, err := config.LoadDefaultProviderConfig()
	if err != nil {
		return err, ""
	}
	if strings.TrimSpace(provider.RefreshToken) == "" {
		return fmt.Errorf("provider %q refresh token is empty", provider.Name), ""
	}

	rtr := &RefreshTokensRequest{
		ClientID:     "app_EMoamEEZ73f0CkXaXp7hrann",
		GrantType:    "refresh_token",
		RefreshToken: provider.RefreshToken,
		Scope:        "openid email profile",
	}
	ctx := context.Background()
	err, body := RequestRefresh(&ctx, *rtr)
	if err != nil {
		return err, ""
	}
	tokens, err := ParseReReqToken(body)
	if err != nil {
		return err, ""
	}
	if strings.TrimSpace(tokens.AccessToken) == "" {
		return fmt.Errorf("token response does not contain access_token"), ""
	}

	updatedProvider, err := config.UpdateProviderOAuthTokens(
		provider.Name,
		tokens.AccessToken,
		tokens.IDToken,
		tokens.RefreshToken,
		accountIDFromIDToken(tokens.IDToken),
	)
	if err != nil {
		return err, ""
	}
	utils.AccessToken = updatedProvider.AccessToken
	utils.RefreshToken = updatedProvider.RefreshToken
	utils.AccountID = updatedProvider.AccountID
	return nil, updatedProvider.AccessToken
}

// ParseReReqToken 解析刷新令牌响应体
func ParseReReqToken(body []byte) (*ReReqToken, error) {
	var token ReReqToken
	err := json.Unmarshal(body, &token)
	return &token, err
}

func RequestRefresh(ctx *context.Context, rtr RefreshTokensRequest) (error, []byte) {
	client := req.C().SetTimeout(30 * time.Second)
	request := client.R()
	request.SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetContext(*ctx).
		SetFormData(map[string]string{
			"client_id":     rtr.ClientID,
			"grant_type":    rtr.GrantType,
			"scope":         rtr.Scope,
			"refresh_token": rtr.RefreshToken,
		})
	resp, err := request.Post("https://auth.openai.com/oauth/token")

	if err != nil {
		return err, nil
	}
	if !resp.IsSuccessState() {
		return fmt.Errorf("http status error: %d, %s", resp.StatusCode, resp.Status), nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err, nil
	}
	defer resp.Body.Close()

	return nil, body
}
