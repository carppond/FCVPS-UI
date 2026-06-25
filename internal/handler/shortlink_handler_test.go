package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"shiguang-vps/internal/shortlink"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
)

// shortlinkTestStack wires the auth stack + ShortLinkHandler.
type shortlinkTestStack struct {
	*authTestStack
	repo    *storage.ShortLinkRepo
	service *shortlink.Service
}

func newShortLinkTestStack(t *testing.T) *shortlinkTestStack {
	t.Helper()
	base := newAuthTestStack(t)
	repo := storage.NewShortLinkRepo(base.dbRef, time.Now)
	svc := shortlink.New(repo, nil, time.Now)
	sh := NewShortLinkHandler(ShortLinkHandlerConfig{
		Service: svc,
		BaseURL: "http://localhost",
	})
	deps := &Deps{
		DB:               base.dbRef,
		AuthManager:      base.mgr,
		TokenStore:       base.tokens,
		UserRepo:         base.users,
		SessionRepo:      base.sessions,
		TOTPManager:      base.totp,
		ShortLinkHandler: sh,
	}
	base.mux = NewRouter(deps)
	return &shortlinkTestStack{authTestStack: base, repo: repo, service: svc}
}

func TestShortLinkCreateAndRedirect(t *testing.T) {
	s := newShortLinkTestStack(t)
	tok, _ := s.loginAs(t, "alice", "Hunter2-AAAA", types.RoleUser)

	create := s.do(http.MethodPost, "/api/shortlinks", types.CreateShortLinkRequest{
		TargetURL: "https://example.com/long/path",
	}, tok)
	if create.Code != http.StatusCreated {
		t.Fatalf("POST status=%d body=%s", create.Code, create.Body.String())
	}
	var env envelope[types.ShortLink]
	if err := json.Unmarshal(create.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.FileCode == "" || env.Data.UserCode == "" {
		t.Fatalf("expected codes populated, got %+v", env.Data)
	}
	combined := env.Data.FileCode + env.Data.UserCode

	// Redirect (public).
	rec := s.do(http.MethodGet, "/s/"+combined, nil, "")
	if rec.Code != http.StatusFound {
		t.Fatalf("GET /s/%s status=%d body=%s", combined, rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "https://example.com/long/path" {
		t.Fatalf("unexpected Location: %q", loc)
	}
}

func TestShortLinkRedirectUnknownReturns404(t *testing.T) {
	s := newShortLinkTestStack(t)
	rec := s.do(http.MethodGet, "/s/abcdef", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestShortLinkListScopesToCaller(t *testing.T) {
	s := newShortLinkTestStack(t)
	aliceTok, _ := s.loginAs(t, "alice", "Hunter2-AAAA", types.RoleUser)
	bobTok, _ := s.loginAs(t, "bob", "Hunter2-AAAA", types.RoleUser)

	for _, target := range []string{"https://a.com", "https://b.com"} {
		rec := s.do(http.MethodPost, "/api/shortlinks", types.CreateShortLinkRequest{TargetURL: target}, aliceTok)
		if rec.Code != http.StatusCreated {
			t.Fatalf("alice create: %d body=%s", rec.Code, rec.Body.String())
		}
	}
	rec := s.do(http.MethodPost, "/api/shortlinks", types.CreateShortLinkRequest{TargetURL: "https://c.com"}, bobTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("bob create: %d body=%s", rec.Code, rec.Body.String())
	}

	listRec := s.do(http.MethodGet, "/api/shortlinks", nil, aliceTok)
	if listRec.Code != http.StatusOK {
		t.Fatalf("alice list: %d body=%s", listRec.Code, listRec.Body.String())
	}
	var env envelope[[]types.ShortLink]
	if err := json.Unmarshal(listRec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data) != 2 {
		t.Fatalf("alice should see 2 links, got %d", len(env.Data))
	}
}

func TestShortLinkDelete(t *testing.T) {
	s := newShortLinkTestStack(t)
	tok, _ := s.loginAs(t, "alice", "Hunter2-AAAA", types.RoleUser)
	create := s.do(http.MethodPost, "/api/shortlinks", types.CreateShortLinkRequest{TargetURL: "https://x.com"}, tok)
	if create.Code != http.StatusCreated {
		t.Fatalf("create: %d body=%s", create.Code, create.Body.String())
	}
	var env envelope[types.ShortLink]
	_ = json.Unmarshal(create.Body.Bytes(), &env)

	del := s.do(http.MethodDelete, "/api/shortlinks/"+env.Data.FileCode+"/"+env.Data.UserCode, nil, tok)
	if del.Code != http.StatusOK {
		t.Fatalf("delete: %d body=%s", del.Code, del.Body.String())
	}

	rec := s.do(http.MethodGet, "/s/"+env.Data.FileCode+env.Data.UserCode, nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", rec.Code)
	}
}

func TestShortLinkCreateRejectsInvalidURL(t *testing.T) {
	s := newShortLinkTestStack(t)
	tok, _ := s.loginAs(t, "alice", "Hunter2-AAAA", types.RoleUser)
	rec := s.do(http.MethodPost, "/api/shortlinks", types.CreateShortLinkRequest{TargetURL: "javascript:alert(1)"}, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-http URL, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestShortLinkCreateRequiresAuth(t *testing.T) {
	s := newShortLinkTestStack(t)
	rec := s.do(http.MethodPost, "/api/shortlinks", types.CreateShortLinkRequest{TargetURL: "https://x.com"}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestComposeShortURL_PreservesPort(t *testing.T) {
	h := &ShortLinkHandler{}
	cases := []struct {
		name    string
		host    string
		headers map[string]string
		want    string
	}{
		{
			name:    "nginx strips port, X-Forwarded-Port recovers it",
			host:    "vpn.example.com",
			headers: map[string]string{"X-Forwarded-Proto": "https", "X-Forwarded-Port": "8443"},
			want:    "https://vpn.example.com:8443/s/abc",
		},
		{
			name:    "default https port is omitted",
			host:    "vpn.example.com",
			headers: map[string]string{"X-Forwarded-Proto": "https", "X-Forwarded-Port": "443"},
			want:    "https://vpn.example.com/s/abc",
		},
		{
			name:    "host already carries a port (local dev) is untouched",
			host:    "localhost:5173",
			headers: map[string]string{"X-Forwarded-Proto": "https", "X-Forwarded-Port": "8443"},
			want:    "https://localhost:5173/s/abc",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, _ := http.NewRequest(http.MethodGet, "/api/shortlinks", nil)
			r.Host = c.host
			for k, v := range c.headers {
				r.Header.Set(k, v)
			}
			if got := h.composeShortURL(r, "abc"); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}
