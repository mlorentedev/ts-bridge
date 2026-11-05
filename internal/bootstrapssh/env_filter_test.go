package bootstrapssh

import (
	"reflect"
	"testing"
)

func TestWithoutSecrets(t *testing.T) {
	tests := []struct {
		name    string
		environ []string
		want    []string
	}{
		{
			name:    "auth key and headscale api key",
			environ: []string{"TS_AUTHKEY=tskey-auth-x", "TS_HEADSCALE_API_KEY=k", "TS_TARGET=host:3389"},
			want:    []string{"TS_TARGET=host:3389"},
		},
		{
			name:    "oauth, token and password under every namespace",
			environ: []string{"TS_OAUTH_CLIENT_SECRET=s", "TAILSCALE_API_KEY=k", "HEADSCALE_TOKEN=t", "TS_PASSWORD=p"},
			want:    []string{},
		},
		{
			name:    "ssh needs pass through",
			environ: []string{"HOME=/h", "PATH=/bin", "SSH_AUTH_SOCK=/s", "TS_AUTHKEY=k", "AWS_SECRET_ACCESS_KEY=other"},
			want:    []string{"HOME=/h", "PATH=/bin", "SSH_AUTH_SOCK=/s", "AWS_SECRET_ACCESS_KEY=other"},
		},
		{
			name:    "case-insensitive name, value keeps its equals signs",
			environ: []string{"ts_authkey=a=b", "KEEP=a=b"},
			want:    []string{"KEEP=a=b"},
		},
		{
			name:    "authkey file path is not a credential",
			environ: []string{"TS_AUTHKEY_FILE=/run/key"},
			want:    []string{"TS_AUTHKEY_FILE=/run/key"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := withoutSecrets(tt.environ); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("withoutSecrets() = %v, want %v", got, tt.want)
			}
		})
	}
}
