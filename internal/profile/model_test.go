package profile

import "testing"

func baseValidProfile() Profile {
	return Profile{
		Name:        "acme",
		ProjectDir:  "/tmp/acme",
		SafetyLevel: ReadOnly,
		AuthMode:    AuthSubscription,
	}
}

func TestValidateAcceptsKnownAuthModes(t *testing.T) {
	for _, mode := range []AuthMode{AuthSubscription, AuthAPIKey} {
		p := baseValidProfile()
		p.AuthMode = mode
		if err := Validate(p); err != nil {
			t.Fatalf("Validate(%q) = %v, want nil", mode, err)
		}
	}
}

func TestValidateRejectsUnknownAuthMode(t *testing.T) {
	p := baseValidProfile()
	p.AuthMode = "magic-link"
	if err := Validate(p); err == nil {
		t.Fatal("Validate accepted unknown auth_mode, want error")
	}
}

func TestValidateHoncho(t *testing.T) {
	tests := map[string]struct {
		honcho  HonchoConfig
		wantErr bool
	}{
		"valid enabled": {
			honcho: HonchoConfig{
				Enabled:     true,
				WorkspaceID: "workspace-123",
				UserName:    "naveed",
			},
		},
		"valid custom secret name": {
			honcho: HonchoConfig{
				Enabled:      true,
				WorkspaceID:  "workspace-123",
				UserName:     "naveed",
				APIKeySecret: " ACME_HONCHO_KEY ",
			},
		},
		"disabled": {
			honcho: HonchoConfig{},
		},
		"missing workspace": {
			honcho: HonchoConfig{
				Enabled:     true,
				WorkspaceID: "   ",
				UserName:    "naveed",
			},
			wantErr: true,
		},
		"missing user": {
			honcho: HonchoConfig{
				Enabled:     true,
				WorkspaceID: "workspace-123",
				UserName:    "   ",
			},
			wantErr: true,
		},
		"control character": {
			honcho: HonchoConfig{
				Enabled:      true,
				WorkspaceID:  "workspace-123",
				UserName:     "naveed",
				APIKeySecret: "key\u007fsecret",
			},
			wantErr: true,
		},
		"invalid api key secret": {
			honcho: HonchoConfig{
				Enabled:      true,
				WorkspaceID:  "workspace-123",
				UserName:     "naveed",
				APIKeySecret: "bad-name",
			},
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p := baseValidProfile()
			p.Honcho = tt.honcho
			err := Validate(p)
			if tt.wantErr && err == nil {
				t.Fatal("Validate returned nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}
		})
	}
}

func TestParseSecretRef(t *testing.T) {
	cases := map[string]struct {
		want string
		ok   bool
	}{
		"${secret:GITHUB_TOKEN}": {want: "GITHUB_TOKEN", ok: true},
		"${secret:_X1}":          {want: "_X1", ok: true},
		"literal":                {ok: false},
		"${secret:}":             {ok: false},
		"${secret:bad-name}":     {ok: false},
		"prefix${secret:X}":      {ok: false},
	}
	for input, want := range cases {
		got, ok := ParseSecretRef(input)
		if ok != want.ok || got != want.want {
			t.Fatalf("ParseSecretRef(%q) = (%q,%v), want (%q,%v)", input, got, ok, want.want, want.ok)
		}
	}
}

func TestValidateAcceptsMCPServers(t *testing.T) {
	p := baseValidProfile()
	p.MCP = map[string]MCPServer{
		"github": {Command: "npx", Args: []string{"-y", "srv"}, Env: map[string]string{"GH": "${secret:GITHUB_TOKEN}"}},
	}
	if err := Validate(p); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}
