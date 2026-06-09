package users

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func captchaHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestNewEmailSenderFromEnv(t *testing.T) {
	t.Run("falls back to logger without SMTP host or sender", func(t *testing.T) {
		t.Setenv("SMTP_HOST", "")
		t.Setenv("SMTP_FROM", "")

		_, ok := NewEmailSenderFromEnv().(LogEmailSender)
		assert.True(t, ok)
	})

	t.Run("builds SMTP sender from env and defaults port", func(t *testing.T) {
		t.Setenv("SMTP_HOST", " smtp.example.com ")
		t.Setenv("SMTP_PORT", "")
		t.Setenv("SMTP_USERNAME", " user ")
		t.Setenv("SMTP_PASSWORD", " pass with spaces ")
		t.Setenv("SMTP_FROM", " Chess E Net <no-reply@example.com> ")

		sender, ok := NewEmailSenderFromEnv().(*SMTPEmailSender)
		require.True(t, ok)
		assert.Equal(t, "smtp.example.com", sender.Host)
		assert.Equal(t, "587", sender.Port)
		assert.Equal(t, "user", sender.Username)
		assert.Equal(t, " pass with spaces ", sender.Password)
		assert.Equal(t, "Chess E Net <no-reply@example.com>", sender.From)
	})
}

func TestEmailSendersValidation(t *testing.T) {
	assert.NoError(t, NewLogEmailSender().SendVerificationCode("user@example.com", "123456"))

	t.Run("rejects invalid from address before SMTP connection", func(t *testing.T) {
		sender := &SMTPEmailSender{From: "not an address"}

		err := sender.SendVerificationCode("user@example.com", "123456")

		assert.ErrorIs(t, err, ErrEmailDelivery)
	})

	t.Run("rejects invalid recipient before SMTP connection", func(t *testing.T) {
		sender := &SMTPEmailSender{From: "Chess E Net <no-reply@example.com>"}

		err := sender.SendVerificationCode("bad recipient", "123456")

		assert.ErrorIs(t, err, ErrEmailDelivery)
	})
}

func TestNewTurnstileVerifierFromEnv(t *testing.T) {
	t.Run("disabled without secret key", func(t *testing.T) {
		t.Setenv("TURNSTILE_SITE_KEY", " site-key ")
		t.Setenv("TURNSTILE_SECRET_KEY", "")

		verifier, ok := NewTurnstileVerifierFromEnv().(*TurnstileVerifier)
		require.True(t, ok)
		assert.False(t, verifier.Enabled())
		assert.Equal(t, "site-key", verifier.SiteKey())
	})

	t.Run("enabled with secret key", func(t *testing.T) {
		t.Setenv("TURNSTILE_SITE_KEY", " site-key ")
		t.Setenv("TURNSTILE_SECRET_KEY", " secret-key ")

		verifier, ok := NewTurnstileVerifierFromEnv().(*TurnstileVerifier)
		require.True(t, ok)
		assert.True(t, verifier.Enabled())
		assert.Equal(t, "site-key", verifier.SiteKey())
		assert.Equal(t, turnstileSiteVerifyURL, verifier.verifyURL)
		require.NotNil(t, verifier.httpClient)
	})
}

func TestTurnstileVerifierVerifyBypassesWhenDisabled(t *testing.T) {
	var nilVerifier *TurnstileVerifier
	assert.NoError(t, nilVerifier.Verify(context.Background(), "", ""))
	assert.NoError(t, (&TurnstileVerifier{}).Verify(context.Background(), "", ""))
}

func TestTurnstileVerifierVerifyRequiresToken(t *testing.T) {
	verifier := &TurnstileVerifier{secretKey: "secret"}

	err := verifier.Verify(context.Background(), "   ", "")

	assert.ErrorIs(t, err, ErrCaptchaRequired)
}

func TestTurnstileVerifierVerifySuccessSendsExpectedForm(t *testing.T) {
	verifier := &TurnstileVerifier{
		secretKey: " secret ",
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, http.MethodPost, req.Method)
			assert.Equal(t, turnstileSiteVerifyURL, req.URL.String())
			assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))

			rawBody, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			values, err := url.ParseQuery(string(rawBody))
			require.NoError(t, err)
			assert.Equal(t, "secret", values.Get("secret"))
			assert.Equal(t, "token", values.Get("response"))
			assert.Equal(t, "203.0.113.10", values.Get("remoteip"))

			return captchaHTTPResponse(http.StatusOK, `{"success":true}`), nil
		})},
	}

	err := verifier.Verify(context.Background(), " token ", " 203.0.113.10 ")

	assert.NoError(t, err)
}

func TestTurnstileVerifierVerifyFailureModes(t *testing.T) {
	t.Run("bad verify URL", func(t *testing.T) {
		verifier := &TurnstileVerifier{secretKey: "secret", verifyURL: "://bad-url"}

		err := verifier.Verify(context.Background(), "token", "")

		assert.ErrorIs(t, err, ErrCaptchaUnavailable)
	})

	t.Run("transport error", func(t *testing.T) {
		verifier := &TurnstileVerifier{
			secretKey: "secret",
			httpClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("network down")
			})},
		}

		err := verifier.Verify(context.Background(), "token", "")

		assert.ErrorIs(t, err, ErrCaptchaUnavailable)
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		verifier := &TurnstileVerifier{
			secretKey: "secret",
			httpClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return captchaHTTPResponse(http.StatusOK, `{bad-json`), nil
			})},
		}

		err := verifier.Verify(context.Background(), "token", "")

		assert.ErrorIs(t, err, ErrCaptchaUnavailable)
	})

	t.Run("non success status", func(t *testing.T) {
		verifier := &TurnstileVerifier{
			secretKey: "secret",
			httpClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return captchaHTTPResponse(http.StatusServiceUnavailable, `{"success":true}`), nil
			})},
		}

		err := verifier.Verify(context.Background(), "token", "")

		assert.ErrorIs(t, err, ErrCaptchaUnavailable)
	})

	t.Run("failed captcha with error codes", func(t *testing.T) {
		verifier := &TurnstileVerifier{
			secretKey: "secret",
			httpClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return captchaHTTPResponse(http.StatusOK, `{"success":false,"error-codes":["timeout-or-duplicate"]}`), nil
			})},
		}

		err := verifier.Verify(context.Background(), "token", "")

		assert.ErrorIs(t, err, ErrCaptchaInvalid)
		assert.Contains(t, err.Error(), "timeout-or-duplicate")
	})

	t.Run("failed captcha without error codes", func(t *testing.T) {
		verifier := &TurnstileVerifier{
			secretKey: "secret",
			httpClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return captchaHTTPResponse(http.StatusOK, `{"success":false}`), nil
			})},
		}

		err := verifier.Verify(context.Background(), "token", "")

		assert.ErrorIs(t, err, ErrCaptchaInvalid)
	})
}

func TestVerificationCodeMatchesRejectsBlankValues(t *testing.T) {
	assert.False(t, verificationCodeMatches("", "123456"))
	assert.False(t, verificationCodeMatches("hash", "   "))
}
