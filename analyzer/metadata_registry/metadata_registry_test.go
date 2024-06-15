package metadata_registry

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseKeybaseResponse(t *testing.T) {
	tc := []struct {
		json      string
		shouldErr bool
		resp      string
	}{
		// https://keybase.io/_/api/1.0/user/lookup.json?fields=pictures&usernames=ptrs
		{
			json:      `{"status":{"code":0,"name":"OK"},"them":[{"id":"149815106745ab70d3e9cdc00fc96419","pictures":{"primary":{"url":"https://s3.amazonaws.com/keybase_processed_uploads/ef2c20c2c1e1a6584d91c49303fd8e05_360_360.jpg","source":null}}},{"id":"a19ba1453ec120ed157c6e85b245c119"}]}`,
			shouldErr: false,
			resp:      "https://s3.amazonaws.com/keybase_processed_uploads/ef2c20c2c1e1a6584d91c49303fd8e05_360_360.jpg",
		},
		// https://keybase.io/_/api/1.0/user/lookup.json?fields=pictures&usernames=test
		{
			json:      `{"status":{"code":0,"name":"OK"},"them":[{"id":"a19ba1453ec120ed157c6e85b245c119"}]}`,
			shouldErr: false,
			resp:      "",
		},
		// Invalid JSON.
		{
			json:      `{"status":123, "code}`,
			shouldErr: true,
			resp:      "",
		},
		// Empty response.
		{
			json:      "",
			shouldErr: true,
			resp:      "",
		},
	}

	for _, tt := range tc {
		url, err := parseKeybaseLogoUrl(strings.NewReader(tt.json))
		if tt.shouldErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tt.resp, url)
		}
	}
}

func TestFetchKeybase(t *testing.T) {
	logo, err := fetchKeybaseLogoUrl(context.Background(), "ptrs")
	require.NoError(t, err)
	require.NotEmpty(t, logo)
}
