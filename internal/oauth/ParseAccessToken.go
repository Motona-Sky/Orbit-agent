package oauth

import (
	"errors"
	"fmt"
	"orbit/internal/utils"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ParseAccessToken 解析 JWT claims，但不校验令牌签名。
func ParseJWT(accessToken string) (jwt.MapClaims, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return nil, errors.New("access token is empty")
	}

	token, _, err := jwt.NewParser().ParseUnverified(accessToken, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("parse access token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("access token contains invalid claims")
	}

	return claims, nil
}

// ParseAccessToken reports whether the token remains usable outside the refresh window.
func ParseAccessToken(accessToken string) (bool, error) {
	claims, err := ParseJWT(accessToken)
	if err != nil {
		return false, err
	}
	exp, ok := claims["exp"]
	if !ok {
		return false, errors.New("access token does not contain exp")
	}
	expiresAt, ok := exp.(float64)
	if !ok {
		return false, fmt.Errorf("access token exp has invalid type %T", exp)
	}

	return time.Now().Add(5 * time.Minute).Before(time.Unix(int64(expiresAt), 0)), nil
}

func GetAccountToken() (string, error) {
	accessjwt := utils.AccessToken
	token, err := ParseJWT(accessjwt)
	if err != nil {
		return "", err
	}
	return token["accountID"].(string), nil
}

type OauthUser struct {
	User      string
	Workspace string
}

func GetOauthUser(accessTokens ...string) (OauthUser, error) {
	accessToken := utils.AccessToken
	if len(accessTokens) > 0 {
		accessToken = accessTokens[0]
	}
	token, err := ParseJWT(accessToken)
	if err != nil {
		return OauthUser{}, err
	}
	profile, ok := token["https://api.openai.com/profile"].(map[string]interface{})
	if !ok {
		return OauthUser{}, errors.New("access token does not contain profile")
	}
	user, ok := profile["email"].(string)
	if !ok {
		return OauthUser{}, errors.New("access token profile does not contain email")
	}
	workspace, ok := token["workspace"].(string)
	if !ok {
		return OauthUser{}, errors.New("access token does not contain workspace")
	}
	return OauthUser{User: user, Workspace: workspace}, nil
}

func (o *OauthUser) GetOauthUser() error {
	user, err := GetOauthUser()
	if err != nil {
		return err
	}
	*o = user
	return nil
}
