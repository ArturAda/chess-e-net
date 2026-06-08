package users

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const turnstileSiteVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

type CaptchaVerifier interface {
	Enabled() bool
	SiteKey() string
	Verify(ctx context.Context, token string, remoteIP string) error
}

type TurnstileVerifier struct {
	siteKey    string
	secretKey  string
	httpClient *http.Client
	verifyURL  string
}

type turnstileSiteVerifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

func NewTurnstileVerifierFromEnv() CaptchaVerifier {
	siteKey := strings.TrimSpace(os.Getenv("TURNSTILE_SITE_KEY"))
	secretKey := strings.TrimSpace(os.Getenv("TURNSTILE_SECRET_KEY"))
	if secretKey == "" {
		return &TurnstileVerifier{siteKey: siteKey}
	}
	if siteKey == "" {
		log.Println("TURNSTILE_SECRET_KEY is configured but TURNSTILE_SITE_KEY is empty; frontend widget will not render")
	}

	return &TurnstileVerifier{
		siteKey:   siteKey,
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		verifyURL: turnstileSiteVerifyURL,
	}
}

func (v *TurnstileVerifier) Enabled() bool {
	return strings.TrimSpace(v.secretKey) != ""
}

func (v *TurnstileVerifier) SiteKey() string {
	return strings.TrimSpace(v.siteKey)
}

func (v *TurnstileVerifier) Verify(ctx context.Context, token string, remoteIP string) error {
	if v == nil || !v.Enabled() {
		return nil
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return ErrCaptchaRequired
	}

	client := v.httpClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	verifyURL := strings.TrimSpace(v.verifyURL)
	if verifyURL == "" {
		verifyURL = turnstileSiteVerifyURL
	}

	form := url.Values{}
	form.Set("secret", strings.TrimSpace(v.secretKey))
	form.Set("response", token)
	if strings.TrimSpace(remoteIP) != "" {
		form.Set("remoteip", strings.TrimSpace(remoteIP))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCaptchaUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCaptchaUnavailable, err)
	}
	defer resp.Body.Close()

	var result turnstileSiteVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("%w: %v", ErrCaptchaUnavailable, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: siteverify status %d", ErrCaptchaUnavailable, resp.StatusCode)
	}
	if !result.Success {
		if len(result.ErrorCodes) > 0 {
			return fmt.Errorf("%w: %s", ErrCaptchaInvalid, strings.Join(result.ErrorCodes, ","))
		}
		return ErrCaptchaInvalid
	}

	return nil
}
