package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testSecret = "test-secret"

// forgeToken builds an HS256 JWT the way the platform does.
func forgeToken(t *testing.T, secret, alg string, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	head := enc(map[string]string{"alg": alg, "typ": "JWT"})
	payload := enc(claims)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(head + "." + payload))
	return head + "." + payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func testAuth(t *testing.T, authAPI string) *Auth {
	t.Helper()
	a := NewAuth(Config{
		JWTSecret: testSecret,
		AuthAPI:   authAPI,
		LoginURL:  "https://42.uz/login",
		EnrollURL: "https://42.uz/course/devops",
	}, t.Logf)
	if !a.Enabled() {
		t.Fatal("auth should be enabled")
	}
	return a
}

func TestAuthenticateToken(t *testing.T) {
	a := testAuth(t, "http://unused")
	exp := time.Now().Add(time.Hour).Unix()

	// Valid token.
	u, tokenExp, err := a.authenticateToken(forgeToken(t, testSecret, "HS256",
		map[string]any{"usr": 248691799, "name": "Azimjon", "exp": exp}))
	if err != nil || u.Id != "248691799" || u.Name != "Azimjon" {
		t.Fatalf("valid token: %+v %v", u, err)
	}
	if tokenExp.Unix() != exp {
		t.Errorf("exp = %v, want %v", tokenExp.Unix(), exp)
	}

	// Wrong secret.
	if _, _, err := a.authenticateToken(forgeToken(t, "other-secret", "HS256",
		map[string]any{"usr": 1, "exp": exp})); err == nil {
		t.Error("wrong secret must fail")
	}
	// Algorithm confusion: "none" and non-HS256 must be rejected outright.
	for _, alg := range []string{"none", "RS256", "HS384"} {
		if _, _, err := a.authenticateToken(forgeToken(t, testSecret, alg,
			map[string]any{"usr": 1, "exp": exp})); err == nil {
			t.Errorf("alg %q must be rejected", alg)
		}
	}
	// Expired.
	if _, _, err := a.authenticateToken(forgeToken(t, testSecret, "HS256",
		map[string]any{"usr": 1, "exp": time.Now().Add(-time.Minute).Unix()})); err == nil {
		t.Error("expired token must fail")
	}
	// Missing usr claim.
	if _, _, err := a.authenticateToken(forgeToken(t, testSecret, "HS256",
		map[string]any{"exp": exp})); err == nil {
		t.Error("missing usr must fail")
	}
	// Garbage.
	if _, _, err := a.authenticateToken("not.a.jwt"); err == nil {
		t.Error("garbage must fail")
	}
}

// fakeAuthAPI serves the access-token exchange: refresh token "good-<usr>"
// yields a valid JWT for that user; "reject" yields 401; anything else 500.
func fakeAuthAPI(t *testing.T, calls *atomic.Int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/access-token/" {
			http.NotFound(w, r)
			return
		}
		calls.Add(1)
		var body struct {
			RefreshToken string `json:"refreshToken"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch {
		case strings.HasPrefix(body.RefreshToken, "good-"):
			var usr int
			fmt.Sscanf(body.RefreshToken, "good-%d", &usr)
			tok := forgeToken(t, testSecret, "HS256", map[string]any{
				"usr": usr, "name": "User " + body.RefreshToken,
				"exp": time.Now().Add(5 * time.Minute).Unix(),
			})
			_ = json.NewEncoder(w).Encode(map[string]string{"accessToken": tok})
		case body.RefreshToken == "reject":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
}

func middlewareTarget(a *Auth) http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return a.Middleware(inner)
}

func doReq(h http.Handler, path, refreshToken string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", path, nil)
	if refreshToken != "" {
		r.AddCookie(&http.Cookie{Name: refreshCookie, Value: refreshToken})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestMiddleware(t *testing.T) {
	var calls atomic.Int64
	api := fakeAuthAPI(t, &calls)
	defer api.Close()
	a := testAuth(t, api.URL)
	h := middlewareTarget(a)

	// No cookie: API gets 401 JSON with login URL; page gets 302 to login.
	w := doReq(h, "/api/jobs", "")
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "42.uz/login") {
		t.Fatalf("api no-cookie: %d %s", w.Code, w.Body.String())
	}
	w = doReq(h, "/", "")
	if w.Code != http.StatusFound || w.Header().Get("Location") != "https://42.uz/login" {
		t.Fatalf("page no-cookie: %d -> %q", w.Code, w.Header().Get("Location"))
	}

	// Health endpoints stay open.
	if w = doReq(h, "/healthz", ""); w.Code != http.StatusOK {
		t.Fatalf("healthz gated: %d", w.Code)
	}

	// Allowlisted user (owner id from the allowlist) passes.
	if w = doReq(h, "/api/jobs", "good-248691799"); w.Code != http.StatusOK {
		t.Fatalf("allowlisted user blocked: %d %s", w.Code, w.Body.String())
	}

	// Authenticated but NOT allowlisted: API 403 with enroll redirect; page 302 to enroll.
	w = doReq(h, "/api/jobs", "good-999")
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "course/devops") {
		t.Fatalf("non-enrollee api: %d %s", w.Code, w.Body.String())
	}
	w = doReq(h, "/", "good-999")
	if w.Code != http.StatusFound || w.Header().Get("Location") != "https://42.uz/course/devops" {
		t.Fatalf("non-enrollee page: %d -> %q", w.Code, w.Header().Get("Location"))
	}

	// Rejected refresh token: 401.
	if w = doReq(h, "/api/jobs", "reject"); w.Code != http.StatusUnauthorized {
		t.Fatalf("rejected token: %d", w.Code)
	}
	// Auth API failure (5xx): 502 for API calls.
	if w = doReq(h, "/api/jobs", "boom"); w.Code != http.StatusBadGateway {
		t.Fatalf("transient failure: %d", w.Code)
	}
}

func TestAuthCaching(t *testing.T) {
	var calls atomic.Int64
	api := fakeAuthAPI(t, &calls)
	defer api.Close()
	a := testAuth(t, api.URL)
	h := middlewareTarget(a)

	// Three sequential requests with the same token: one exchange.
	for i := 0; i < 3; i++ {
		if w := doReq(h, "/api/jobs", "good-248691799"); w.Code != http.StatusOK {
			t.Fatalf("req %d: %d", i, w.Code)
		}
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("expected 1 auth API call, got %d", n)
	}

	// Rejections are negative-cached too.
	calls.Store(0)
	for i := 0; i < 3; i++ {
		doReq(h, "/api/jobs", "reject")
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("expected 1 auth API call for cached rejection, got %d", n)
	}
}

func TestAuthSingleflight(t *testing.T) {
	var calls atomic.Int64
	api := fakeAuthAPI(t, &calls)
	defer api.Close()
	a := testAuth(t, api.URL)
	h := middlewareTarget(a)

	// A burst of concurrent requests with the same cold token shares one exchange.
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			doReq(h, "/api/jobs", "good-248691799")
		}()
	}
	wg.Wait()
	if n := calls.Load(); n != 1 {
		t.Fatalf("expected 1 auth API call under concurrency, got %d", n)
	}
}

func TestMeEndpoint(t *testing.T) {
	// Disabled auth: the fixed test user is signed in.
	var disabled *Auth
	w := httptest.NewRecorder()
	disabled.handleMe(w, httptest.NewRequest("GET", "/api/me", nil))
	if !strings.Contains(w.Body.String(), "@testuser") || !strings.Contains(w.Body.String(), "Test User") {
		t.Fatalf("disabled /api/me should sign in the test user: %s", w.Body.String())
	}

	// Enabled: flows through the middleware and reports the user.
	var calls atomic.Int64
	api := fakeAuthAPI(t, &calls)
	defer api.Close()
	a := testAuth(t, api.URL)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/me", a.handleMe)
	h := a.Middleware(mux)
	resp := doReq(h, "/api/me", "good-248691799")
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "@azimjon42") {
		t.Fatalf("/api/me: %d %s", resp.Code, resp.Body.String())
	}
}
