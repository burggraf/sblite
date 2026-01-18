package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserInfoValidation(t *testing.T) {
	tests := []struct {
		name    string
		info    *UserInfo
		wantErr bool
	}{
		{
			name: "valid user info",
			info: &UserInfo{
				ProviderID:    "123456",
				Email:         "user@example.com",
				Name:          "Test User",
				EmailVerified: true,
			},
			wantErr: false,
		},
		{
			name: "missing provider ID",
			info: &UserInfo{
				Email: "user@example.com",
			},
			wantErr: true,
		},
		{
			name: "missing email",
			info: &UserInfo{
				ProviderID: "123456",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.info.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
