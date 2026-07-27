package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigValidateWebAppURL(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		wantError bool
	}{
		{name: "disabled app does not require URL", wantError: false},
		{name: "valid public HTTPS URL", publicURL: "https://app.example.com", wantError: false},
		{name: "HTTP URL", publicURL: "http://app.example.com", wantError: true},
		{name: "localhost HTTPS URL", publicURL: "https://localhost:8080", wantError: false},
		{name: "missing URL", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Telegram: TelegramConfig{Token: "test-token"},
				WebApp: WebAppConfig{
					Enabled:   tt.name != "disabled app does not require URL",
					PublicURL: tt.publicURL,
				},
			}

			err := cfg.Validate()
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
