package config

import (
	"os"
	"strings"
	"testing"
)

// TestLoadConfigMissingConfigPath tests that LoadConfig succeeds gracefully
// when the config file is missing and uses defaults + environment variables.
func TestLoadConfigMissingConfigPath(t *testing.T) {
	// Save current env vars
	oldEnv := os.Getenv("DT_ENV")
	oldJWTSecret := os.Getenv("DT_JWT_SECRET")
	oldName := os.Getenv("DT_NAME")
	defer func() {
		// Restore env vars
		if oldEnv != "" {
			os.Setenv("DT_ENV", oldEnv)
		} else {
			os.Unsetenv("DT_ENV")
		}
		if oldJWTSecret != "" {
			os.Setenv("DT_JWT_SECRET", oldJWTSecret)
		} else {
			os.Unsetenv("DT_JWT_SECRET")
		}
		if oldName != "" {
			os.Setenv("DT_NAME", oldName)
		} else {
			os.Unsetenv("DT_NAME")
		}
	}()

	// Set to local profile to skip JWT validation
	os.Setenv("DT_ENV", "local")
	os.Setenv("DT_NAME", "local")
	os.Unsetenv("DT_JWT_SECRET")

	// Create a temporary directory to simulate missing config
	tempDir := t.TempDir()
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(oldCwd)

	// Change to temp directory where no config file exists
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// This should not panic even though config file is missing
	cfg := LoadConfig()
	if cfg == nil {
		t.Fatal("LoadConfig returned nil, expected a Config struct")
	}

	// Verify that the profile name is correctly set from environment
	if cfg.Name != "local" {
		t.Errorf("Config.Name = %q, want %q", cfg.Name, "local")
	}

	// Verify some defaults that have explicit tags are applied
	if !cfg.Database.Migration {
		t.Errorf("Database.Migration = %v, want true", cfg.Database.Migration)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "info")
	}
}

// TestValidateJWTSecretWeakSecrets tests that validateJWTSecret panics
// for known weak secrets.
func TestValidateJWTSecretWeakSecrets(t *testing.T) {
	tests := []struct {
		name   string
		secret string
	}{
		{
			name:   "weak secret: secret",
			secret: "secret",
		},
		{
			name:   "weak secret: mysecret",
			secret: "mysecret",
		},
		{
			name:   "weak secret: jwt_secret",
			secret: "jwt_secret",
		},
		{
			name:   "weak secret: default",
			secret: "default",
		},
		{
			name:   "weak secret: password",
			secret: "password",
		},
		{
			name:   "weak secret: 123456",
			secret: "123456",
		},
		{
			name:   "weak secret: changeme",
			secret: "changeme",
		},
		{
			name:   "weak secret: donetick",
			secret: "donetick",
		},
		{
			name:   "weak secret: jwt",
			secret: "jwt",
		},
		{
			name:   "weak secret: token",
			secret: "token",
		},
		{
			name:   "weak secret: key",
			secret: "key",
		},
		{
			name:   "weak secret: secretkey",
			secret: "secretkey",
		},
		{
			name:   "weak secret: change_this_to_a_secure_random_string_32_characters_long",
			secret: "change_this_to_a_secure_random_string_32_characters_long",
		},
		{
			name:   "weak secret: case insensitive (SECRET)",
			secret: "SECRET",
		},
		{
			name:   "weak secret: case insensitive (MySecret)",
			secret: "MySecret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			didPanic := false
			func() {
				defer func() {
					if r := recover(); r != nil {
						didPanic = true
					}
				}()
				validateJWTSecret(tt.secret)
			}()

			if !didPanic {
				t.Errorf("validateJWTSecret(%q) did not panic, expected panic", tt.secret)
			}
		})
	}
}

// TestValidateJWTSecretTooShort tests that validateJWTSecret panics
// when the secret is shorter than 32 characters.
func TestValidateJWTSecretTooShort(t *testing.T) {
	tests := []struct {
		name   string
		secret string
	}{
		{
			name:   "empty secret",
			secret: "",
		},
		{
			name:   "1 character",
			secret: "a",
		},
		{
			name:   "10 characters",
			secret: strings.Repeat("a", 10),
		},
		{
			name:   "31 characters",
			secret: strings.Repeat("a", 31),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			didPanic := false
			func() {
				defer func() {
					if r := recover(); r != nil {
						didPanic = true
					}
				}()
				validateJWTSecret(tt.secret)
			}()

			if !didPanic {
				t.Errorf("validateJWTSecret with length %d did not panic, expected panic", len(tt.secret))
			}
		})
	}
}

// TestValidateJWTSecretValidSecrets tests that validateJWTSecret does NOT panic
// for strong, adequately-long secrets that are not in the blocklist.
func TestValidateJWTSecretValidSecrets(t *testing.T) {
	tests := []struct {
		name   string
		secret string
	}{
		{
			name:   "exactly 32 characters",
			secret: strings.Repeat("a", 32),
		},
		{
			name:   "more than 32 characters",
			secret: strings.Repeat("a", 64),
		},
		{
			name:   "base64-like string",
			secret: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcd01",
		},
		{
			name:   "alphanumeric with special chars",
			secret: "aB1!@#$%^&*()-_=+[]{}|;:,.<>?xyzw",
		},
		{
			name:   "long strong secret with mixed case",
			secret: "MyVeryStrongJWTSecretWithManyCharactersAndNumbers123456789",
		},
		{
			name:   "not in blocklist: secrets (different from 'secret')",
			secret: "secretsNotInBlocklist12345678901234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			didPanic := false
			func() {
				defer func() {
					if r := recover(); r != nil {
						didPanic = true
					}
				}()
				err := validateJWTSecret(tt.secret)
				// Also verify no error is returned
				if err != nil {
					t.Errorf("validateJWTSecret(%q) returned error %v, expected nil", tt.secret, err)
				}
			}()

			if didPanic {
				t.Errorf("validateJWTSecret(%q) panicked unexpectedly", tt.secret)
			}
		})
	}
}

// TestLoadConfigLocalProfileSkipsJWTValidation tests that when the profile is "local",
// LoadConfig does not call validateJWTSecret even if the JWT secret is weak.
func TestLoadConfigLocalProfileSkipsJWTValidation(t *testing.T) {
	// Save current env vars
	oldEnv := os.Getenv("DT_ENV")
	oldJWTSecret := os.Getenv("DT_JWT_SECRET")
	oldName := os.Getenv("DT_NAME")
	defer func() {
		if oldEnv != "" {
			os.Setenv("DT_ENV", oldEnv)
		} else {
			os.Unsetenv("DT_ENV")
		}
		if oldJWTSecret != "" {
			os.Setenv("DT_JWT_SECRET", oldJWTSecret)
		} else {
			os.Unsetenv("DT_JWT_SECRET")
		}
		if oldName != "" {
			os.Setenv("DT_NAME", oldName)
		} else {
			os.Unsetenv("DT_NAME")
		}
	}()

	// Create a temporary directory to simulate missing config
	tempDir := t.TempDir()
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(oldCwd)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Set to local profile
	os.Setenv("DT_ENV", "local")
	os.Setenv("DT_NAME", "local")
	// Set a weak JWT secret - should be allowed in local profile
	os.Setenv("DT_JWT_SECRET", "weak_secret_but_ok_for_local")

	// This should NOT panic because local profile skips JWT validation
	cfg := LoadConfig()
	if cfg == nil {
		t.Fatal("LoadConfig returned nil, expected a Config struct")
	}

	if cfg.Name != "local" {
		t.Errorf("Config.Name = %q, want %q", cfg.Name, "local")
	}
}

// TestLoadConfigNonLocalProfileValidatesJWT tests that non-local profiles
// enforce JWT secret validation by panicking on weak secrets.
func TestLoadConfigNonLocalProfileValidatesJWT(t *testing.T) {
	// Save current env vars
	oldEnv := os.Getenv("DT_ENV")
	oldJWTSecret := os.Getenv("DT_JWT_SECRET")
	oldName := os.Getenv("DT_NAME")
	defer func() {
		if oldEnv != "" {
			os.Setenv("DT_ENV", oldEnv)
		} else {
			os.Unsetenv("DT_ENV")
		}
		if oldJWTSecret != "" {
			os.Setenv("DT_JWT_SECRET", oldJWTSecret)
		} else {
			os.Unsetenv("DT_JWT_SECRET")
		}
		if oldName != "" {
			os.Setenv("DT_NAME", oldName)
		} else {
			os.Unsetenv("DT_NAME")
		}
	}()

	// Create a temporary directory to simulate missing config
	tempDir := t.TempDir()
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(oldCwd)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Set to prod profile (non-local)
	os.Setenv("DT_ENV", "prod")
	os.Setenv("DT_NAME", "prod")
	// Set a weak JWT secret - should cause panic in prod
	os.Setenv("DT_JWT_SECRET", "weak")

	didPanic := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
			}
		}()
		LoadConfig()
	}()

	if !didPanic {
		t.Error("LoadConfig with weak JWT secret in non-local profile did not panic, expected panic")
	}
}

// TestValidateJWTSecretReturnValue tests that validateJWTSecret returns nil
// on success (when no panic occurs).
func TestValidateJWTSecretReturnValue(t *testing.T) {
	validSecret := "this_is_a_valid_secret_with_32_or_more_characters_long_enough"
	err := validateJWTSecret(validSecret)
	if err != nil {
		t.Errorf("validateJWTSecret(%q) returned error %v, expected nil", validSecret, err)
	}
}
