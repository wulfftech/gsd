package mfa

import (
	"encoding/base32"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"donetick.com/core/config"
	"github.com/pquerna/otp/totp"
)

func TestNewService_AppNameDefaultsWhenConfigNameEmpty(t *testing.T) {
	svc := NewService(&config.Config{})
	if svc.appName != "Donetick" {
		t.Errorf("appName = %q, want %q", svc.appName, "Donetick")
	}
}

func TestNewService_AppNameUsesConfiguredName(t *testing.T) {
	svc := NewService(&config.Config{Name: "MyGSD"})
	if svc.appName != "MyGSD" {
		t.Errorf("appName = %q, want %q", svc.appName, "MyGSD")
	}
}

func TestGenerateSecret(t *testing.T) {
	svc := NewService(&config.Config{Name: "GSD"})

	key, err := svc.GenerateSecret("user@example.com")
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}
	if key.Secret() == "" {
		t.Error("GenerateSecret() returned an empty secret")
	}
	if key.AccountName() != "user@example.com" {
		t.Errorf("AccountName() = %q, want %q", key.AccountName(), "user@example.com")
	}
	if key.Issuer() != "GSD" {
		t.Errorf("Issuer() = %q, want %q", key.Issuer(), "GSD")
	}
}

func TestGenerateSecret_RejectsEmptyAccountName(t *testing.T) {
	svc := NewService(&config.Config{})

	if _, err := svc.GenerateSecret(""); err == nil {
		t.Error("GenerateSecret(\"\") expected an error for missing account name, got nil")
	}
}

func TestGenerateSecret_ProducesDistinctSecretsPerCall(t *testing.T) {
	svc := NewService(&config.Config{})

	key1, err := svc.GenerateSecret("user@example.com")
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}
	key2, err := svc.GenerateSecret("user@example.com")
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}
	if key1.Secret() == key2.Secret() {
		t.Error("GenerateSecret() returned the same secret on consecutive calls")
	}
}

func TestVerifyTOTP(t *testing.T) {
	svc := NewService(&config.Config{})

	key, err := svc.GenerateSecret("user@example.com")
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}
	secret := key.Secret()

	otherKey, err := svc.GenerateSecret("other@example.com")
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}
	otherSecret := otherKey.Secret()

	validCode, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("totp.GenerateCode() error = %v", err)
	}

	expiredCode, err := totp.GenerateCode(secret, time.Now().Add(-10*time.Minute))
	if err != nil {
		t.Fatalf("totp.GenerateCode() error = %v", err)
	}

	tests := []struct {
		name   string
		secret string
		code   string
		want   bool
	}{
		{"valid code for the current window", secret, validCode, true},
		{"expired code well outside the skew window is rejected", secret, expiredCode, false},
		{"empty code is rejected", secret, "", false},
		{"zero-value secret is rejected", "", validCode, false},
		{"code generated for a different user's secret is rejected", otherSecret, validCode, false},
		{"garbage code is rejected", secret, "not-a-code", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.VerifyTOTP(tt.secret, tt.code); got != tt.want {
				t.Errorf("VerifyTOTP(%q, %q) = %v, want %v", tt.secret, tt.code, got, tt.want)
			}
		})
	}
}

var backupCodeFormat = regexp.MustCompile(`^[A-Z0-9]{4}-[A-Z0-9]{4}$`)

func TestGenerateBackupCodes(t *testing.T) {
	svc := NewService(&config.Config{})

	codes, err := svc.GenerateBackupCodes(8)
	if err != nil {
		t.Fatalf("GenerateBackupCodes() error = %v", err)
	}
	if len(codes) != 8 {
		t.Fatalf("GenerateBackupCodes() returned %d codes, want 8", len(codes))
	}

	seen := make(map[string]bool)
	for _, code := range codes {
		if !backupCodeFormat.MatchString(code) {
			t.Errorf("backup code %q does not match expected XXXX-XXXX format", code)
		}
		if seen[code] {
			t.Errorf("backup code %q was generated more than once", code)
		}
		seen[code] = true
	}
}

func TestGenerateBackupCodes_ZeroCount(t *testing.T) {
	svc := NewService(&config.Config{})

	codes, err := svc.GenerateBackupCodes(0)
	if err != nil {
		t.Fatalf("GenerateBackupCodes(0) error = %v", err)
	}
	if len(codes) != 0 {
		t.Errorf("GenerateBackupCodes(0) returned %d codes, want 0", len(codes))
	}
}

func TestVerifyBackupCode(t *testing.T) {
	svc := NewService(&config.Config{})
	backupCodes := mustJSON(t, []string{"ABCD-1234", "EFGH-5678"})

	tests := []struct {
		name            string
		backupCodesJSON string
		usedCodesJSON   string
		inputCode       string
		wantValid       bool
		wantErr         bool
	}{
		{
			name:            "unused code is accepted",
			backupCodesJSON: backupCodes,
			usedCodesJSON:   "",
			inputCode:       "ABCD-1234",
			wantValid:       true,
		},
		{
			name:            "matching is case-insensitive and hyphen-insensitive",
			backupCodesJSON: backupCodes,
			usedCodesJSON:   "",
			inputCode:       "abcd1234",
			wantValid:       true,
		},
		{
			name:            "already-used code is rejected",
			backupCodesJSON: backupCodes,
			usedCodesJSON:   mustJSON(t, []string{"ABCD-1234"}),
			inputCode:       "ABCD-1234",
			wantValid:       false,
		},
		{
			name:            "code not present in the backup list is rejected",
			backupCodesJSON: backupCodes,
			usedCodesJSON:   "",
			inputCode:       "ZZZZ-9999",
			wantValid:       false,
		},
		{
			name:            "empty code is rejected",
			backupCodesJSON: backupCodes,
			usedCodesJSON:   "",
			inputCode:       "",
			wantValid:       false,
		},
		{
			name:            "empty backup code list rejects any input",
			backupCodesJSON: "",
			usedCodesJSON:   "",
			inputCode:       "ABCD-1234",
			wantValid:       false,
		},
		{
			name:            "malformed backup codes JSON errors",
			backupCodesJSON: "{not-json",
			usedCodesJSON:   "",
			inputCode:       "ABCD-1234",
			wantErr:         true,
		},
		{
			name:            "malformed used codes JSON errors",
			backupCodesJSON: backupCodes,
			usedCodesJSON:   "{not-json",
			inputCode:       "ABCD-1234",
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, updatedUsedJSON, err := svc.VerifyBackupCode(tt.backupCodesJSON, tt.usedCodesJSON, tt.inputCode)
			if (err != nil) != tt.wantErr {
				t.Fatalf("VerifyBackupCode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if valid != tt.wantValid {
				t.Errorf("VerifyBackupCode() valid = %v, want %v", valid, tt.wantValid)
			}
			if tt.wantValid && updatedUsedJSON == "" {
				t.Error("VerifyBackupCode() returned empty updated used-codes JSON for a valid code")
			}
		})
	}
}

func TestVerifyBackupCode_SameCodeCannotBeReplayed(t *testing.T) {
	svc := NewService(&config.Config{})
	backupCodes := mustJSON(t, []string{"ABCD-1234"})

	valid, usedJSON, err := svc.VerifyBackupCode(backupCodes, "", "ABCD-1234")
	if err != nil {
		t.Fatalf("VerifyBackupCode() error = %v", err)
	}
	if !valid {
		t.Fatal("VerifyBackupCode() expected first use to be valid")
	}

	// Replaying the same code using the used-codes list returned from the
	// first call must be rejected.
	valid, _, err = svc.VerifyBackupCode(backupCodes, usedJSON, "ABCD-1234")
	if err != nil {
		t.Fatalf("VerifyBackupCode() error = %v", err)
	}
	if valid {
		t.Error("VerifyBackupCode() allowed a backup code to be used twice")
	}
}

func TestIsCodeValid(t *testing.T) {
	svc := NewService(&config.Config{})

	key, err := svc.GenerateSecret("user@example.com")
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}
	secret := key.Secret()

	otherKey, err := svc.GenerateSecret("other@example.com")
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}
	otherSecret := otherKey.Secret()

	validTOTP, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("totp.GenerateCode() error = %v", err)
	}

	backupCodes := mustJSON(t, []string{"ABCD-1234"})

	t.Run("valid TOTP code passes without touching used codes", func(t *testing.T) {
		valid, usedJSON, err := svc.IsCodeValid(secret, "", "", validTOTP)
		if err != nil {
			t.Fatalf("IsCodeValid() error = %v", err)
		}
		if !valid {
			t.Error("IsCodeValid() expected a valid TOTP code to pass")
		}
		if usedJSON != "" {
			t.Errorf("IsCodeValid() usedCodesJSON = %q, want unchanged empty string", usedJSON)
		}
	})

	t.Run("falls back to a valid backup code", func(t *testing.T) {
		valid, usedJSON, err := svc.IsCodeValid(secret, backupCodes, "", "ABCD-1234")
		if err != nil {
			t.Fatalf("IsCodeValid() error = %v", err)
		}
		if !valid {
			t.Error("IsCodeValid() expected the backup code to pass")
		}
		if usedJSON == "" {
			t.Error("IsCodeValid() expected updated used-codes JSON after consuming a backup code")
		}
	})

	t.Run("empty code is rejected outright", func(t *testing.T) {
		valid, _, err := svc.IsCodeValid(secret, backupCodes, "", "")
		if err != nil {
			t.Fatalf("IsCodeValid() error = %v", err)
		}
		if valid {
			t.Error("IsCodeValid() accepted an empty code")
		}
	})

	t.Run("code from a different user's secret is rejected", func(t *testing.T) {
		valid, _, err := svc.IsCodeValid(otherSecret, "", "", validTOTP)
		if err != nil {
			t.Fatalf("IsCodeValid() error = %v", err)
		}
		if valid {
			t.Error("IsCodeValid() accepted a TOTP code generated for a different user's secret")
		}
	})

	t.Run("wrong code with no backup codes configured is rejected", func(t *testing.T) {
		valid, _, err := svc.IsCodeValid(secret, "", "", "000000")
		if err != nil {
			t.Fatalf("IsCodeValid() error = %v", err)
		}
		if valid {
			t.Error("IsCodeValid() accepted an arbitrary wrong code")
		}
	})
}

func TestGenerateSessionToken(t *testing.T) {
	svc := NewService(&config.Config{})

	token1, err := svc.GenerateSessionToken()
	if err != nil {
		t.Fatalf("GenerateSessionToken() error = %v", err)
	}
	token2, err := svc.GenerateSessionToken()
	if err != nil {
		t.Fatalf("GenerateSessionToken() error = %v", err)
	}

	if token1 == "" {
		t.Fatal("GenerateSessionToken() returned an empty token")
	}
	if token1 == token2 {
		t.Error("GenerateSessionToken() returned the same token on consecutive calls")
	}

	decoded, err := base32.StdEncoding.DecodeString(token1)
	if err != nil {
		t.Fatalf("GenerateSessionToken() returned a non-base32 token: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("decoded token length = %d bytes, want 32", len(decoded))
	}
}

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return string(b)
}
