package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 42.uz authentication: the browser carries a long-lived "refresh-token"
// cookie issued by the platform. FaangJobs exchanges it for a short-lived
// access-token JWT at the auth API (POST <authAPI>/api/auth/access-token/) and
// validates that JWT locally (HS256, user id in the "usr" claim). Exchanges
// are cached per refresh token until shortly before the JWT expires (bounded
// below), and concurrent requests carrying the same token share one in-flight
// exchange — a page load fires several API calls at once, and each must not
// pay (or rate-limit-trip) its own auth round-trip.
//
// Unauthenticated page loads are redirected to the login URL; API calls get
// 401 with the login URL in the body (the frontend follows it). Authenticated
// users outside the allowlist are sent to the enrollment page (403 on APIs).
//
// With no JWT secret configured (local dev), authentication is disabled and
// nothing redirects.
//
// The JWT (HS256) validation and the per-key singleflight are implemented on
// the standard library so the backend keeps its zero-dependency guarantee.

var allowedUsers = map[string]string{
	// Owner
	"248691799": "@azimjon42",
	// Group I
	"1933002694": "@yuldoshev_uz",
	"859020893":  "@AbdurahmonIbrohimov",
	"722703900":  "@ibrohim_ahatkulov",
	"1010537107": "@nomonzon",
	"1474525499": "@shahzod_rozzoqov",
	"449408303":  "@Khusanbek",
	"1687872138": "@nodir_yoqubov7",
	"2064726254": "@MeeeAbdulloh",
	"200310295":  "@MuzammilTohir",
	"603436474":  "@mhmd_0220",
	"631751797":  "@hardy1729",
	"933803582":  "@shoniyoz002",
	"1957718743": "@Hasan_Polatov",
	"5286693950": "@notabdurauf",
	"964470872":  "@behruz_ibragimovv",
	"678541731":  "@srdrbk",
	"924604756":  "@javokhirdjumanov",
	"649495362":  "@UmidjonYuldashev",
	"653172961":  "@AxmadovIlhomjon",
	"242200192":  "@khn77",
	"245686275":  "@iAbbosF",
	// Group II
	"7221579889": "@Mr_muhammadbilol",
	"904664945":  "@Ayxanov",
	"5987124979": "@Sobirjon_Abdumajidov",
	"2046965638": "@otabekhoshimxon",
	"5943933138": "@bayram_matchanov",
	"1048407637": "@MuxriddinNorqulov",
	"1377040674": "@javlon_jurabekov",
	"633442276":  "@Murodillayev",
	"8681100780": "@itsqodirjon",
	"105618575":  "@TheBugCreator",
	"978738956":  "@unknown00008",
	"8360590641": "@cdShahnoza",
	"1361082424": "@NetSoftSpace",
	"24568627":   "@iAbbosF",
	"1520814545": "@kabulov_cs",
	"318638500":  "@mamadaliyev_swe",
	"1682889286": "@abdulAlloh1",
	"8561156125": "@XudoyberganovOdilbek",
	"5613062658": "@nrx_xusan",
	"900604435":  "@Umidjon09_28",
	"777214417":  "@ilyosjon",
}

const (
	refreshCookie = "refresh-token"
	// accessTokenTTL is the cache floor for a successful exchange; when the
	// access token itself declares a later expiry, the entry lives until 30s
	// before that, capped at maxAuthCacheTTL.
	accessTokenTTL   = time.Minute
	maxAuthCacheTTL  = 10 * time.Minute
	authRejectTTL    = 15 * time.Second // definitive rejection: brief negative cache
	authTransientTTL = 3 * time.Second  // auth API down/5xx: retry quickly
	authCacheMax     = 4096             // opportunistic-cleanup threshold, bounds the cache
)

var (
	errNoRefreshToken = errors.New("authentication required (refresh-token cookie missing)")
	errBadToken       = errors.New("failed to authenticate token")
	// errNotAllowed: authenticated, but the user is not in the allowlist.
	errNotAllowed = errors.New("user is not enrolled in the course")
)

// User is an authenticated platform user.
type User struct {
	Id   string
	Name string
}

// isAllowed reports whether a user id is in the allowlist (allowedUsers).
func isAllowed(userID string) bool {
	_, ok := allowedUsers[userID]
	return ok
}

// cachedUser is one exchange outcome: a user (err == nil) or a cached failure
// (err != nil), so rejected tokens can't hammer the auth API either.
type cachedUser struct {
	user    User
	err     error
	expires time.Time
}

// Auth performs and caches 42.uz authentication for the server.
type Auth struct {
	jwtSecret string
	authAPI   string
	loginURL  string
	enrollURL string
	client    *http.Client
	log       func(format string, args ...any)

	mu    sync.Mutex
	cache map[string]cachedUser
	sf    authFlight
}

// NewAuth builds an Auth from the handler config. Returns nil (auth disabled)
// when no JWT secret is configured.
func NewAuth(cfg Config, log func(string, ...any)) *Auth {
	if cfg.JWTSecret == "" {
		return nil
	}
	return &Auth{
		jwtSecret: cfg.JWTSecret,
		authAPI:   cfg.AuthAPI,
		loginURL:  cfg.LoginURL,
		enrollURL: cfg.EnrollURL,
		client: &http.Client{
			Timeout: 15 * time.Second,
			// Never follow redirects: a misconfigured auth API base that 30x's
			// to a login page must surface as a clear error, not be chased
			// into an HTML page that fails token extraction confusingly.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		log:   log,
		cache: map[string]cachedUser{},
		sf:    authFlight{calls: map[string]*authCall{}},
	}
}

// Enabled reports whether authentication is configured.
func (a *Auth) Enabled() bool { return a != nil }

// ── request authentication ─────────────────────────────────────────────

// RequestUser authenticates the request: refresh-token cookie → (cached)
// access-token exchange → validated JWT → user.
func (a *Auth) RequestUser(r *http.Request) (User, error) {
	c, err := r.Cookie(refreshCookie)
	if err != nil || c.Value == "" {
		return User{}, errNoRefreshToken
	}
	rt := c.Value

	a.mu.Lock()
	if e, ok := a.cache[rt]; ok && time.Now().Before(e.expires) {
		a.mu.Unlock()
		return e.user, e.err
	}
	a.mu.Unlock()

	// Miss: exchange once per token, no matter how many requests race here.
	return a.sf.do(rt, func() (User, error) {
		// Detached context: the shared result must not die with whichever
		// request happened to trigger the exchange.
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		u, exp, ferr := a.exchangeUser(ctx, rt)
		a.cacheAuth(rt, u, exp, ferr)
		return u, ferr
	})
}

// exchangeUser performs the refresh-token → access-token → user exchange.
func (a *Auth) exchangeUser(ctx context.Context, rt string) (User, time.Time, error) {
	tok, err := a.fetchAccessToken(ctx, rt)
	if err != nil {
		return User{}, time.Time{}, err
	}
	return a.authenticateToken(tok)
}

// cacheAuth stores an exchange outcome. Successes live until shortly before
// the access token expires (bounded); rejections are cached briefly so a
// misbehaving client can't hammer the auth API; transient auth-API failures
// shorter still, so recovery is quick.
func (a *Auth) cacheAuth(rt string, u User, tokenExp time.Time, err error) {
	now := time.Now()
	var e cachedUser
	switch {
	case err == nil:
		ttl := accessTokenTTL
		if !tokenExp.IsZero() {
			if until := time.Until(tokenExp) - 30*time.Second; until > ttl {
				ttl = min(until, maxAuthCacheTTL)
			}
		}
		e = cachedUser{user: u, expires: now.Add(ttl)}
	case errors.Is(err, errBadToken):
		e = cachedUser{err: err, expires: now.Add(authRejectTTL)}
	default:
		e = cachedUser{err: err, expires: now.Add(authTransientTTL)}
	}
	a.mu.Lock()
	if len(a.cache) >= authCacheMax {
		for k, old := range a.cache {
			if now.After(old.expires) {
				delete(a.cache, k)
			}
		}
		// Still full — e.g. a flood of unique garbage cookies filling the
		// negative cache faster than entries expire. Evict arbitrary entries
		// (map order is effectively random) so the cache stays hard-bounded;
		// an evicted legitimate user just re-exchanges on their next request.
		for k := range a.cache {
			if len(a.cache) < authCacheMax {
				break
			}
			delete(a.cache, k)
		}
	}
	a.cache[rt] = e
	a.mu.Unlock()
}

// fetchAccessToken trades a refresh token for an access-token JWT at the auth API.
func (a *Auth) fetchAccessToken(ctx context.Context, refreshToken string) (string, error) {
	body, err := json.Marshal(map[string]string{"refreshToken": refreshToken})
	if err != nil {
		return "", err
	}
	url := strings.TrimRight(a.authAPI, "/") + "/api/auth/access-token/"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("auth API unreachable: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		// The refresh token was rejected — the user must log in again.
		return "", fmt.Errorf("%w: refresh rejected (HTTP %d)", errBadToken, resp.StatusCode)
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		// A redirecting "API" is a misconfigured base URL, not a brownout.
		a.log("auth API misconfigured: %s redirected (HTTP %d) to %q — check -auth-api / FAANGJOBS_AUTH_API", url, resp.StatusCode, resp.Header.Get("Location"))
		return "", fmt.Errorf("auth API misconfigured (HTTP %d redirect)", resp.StatusCode)
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed:
		// 404/405 means the exchange endpoint doesn't live at this base URL —
		// almost always a wrong -auth-api (e.g. the web host instead of the
		// API host). Log it loudly for the operator.
		a.log("auth API misconfigured: POST %s returned HTTP %d (body: %.120q) — check -auth-api / FAANGJOBS_AUTH_API (the API host is usually api.42.uz, not the website host)", url, resp.StatusCode, b)
		return "", fmt.Errorf("auth API misconfigured (HTTP %d at %s)", resp.StatusCode, url)
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		// The auth API itself is failing (5xx, rate limit, ...): surface it as
		// transient (502 to the client) rather than logging everyone out — an
		// auth-API brownout must not turn into a mass re-login stampede.
		a.log("auth API error: POST %s returned HTTP %d (body: %.120q)", url, resp.StatusCode, b)
		return "", fmt.Errorf("auth API error (HTTP %d)", resp.StatusCode)
	}
	tok, err := extractAccessToken(b)
	if err != nil {
		a.log("auth API returned HTTP %d but no recognizable token (body: %.120q)", resp.StatusCode, b)
	}
	return tok, err
}

// extractAccessToken pulls the JWT out of the auth API response.
func extractAccessToken(b []byte) (string, error) {
	var r struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(b, &r); err != nil || r.AccessToken == "" {
		return "", errors.New("unexpected auth API response")
	}
	return r.AccessToken, nil
}

// ── JWT validation (stdlib HS256) ──────────────────────────────────────

// authenticateToken validates an access-token JWT, returning the user and the
// token's expiry (zero if the token declares none). Only HS256 is accepted —
// the secret must never be used to "verify" a token that names a different
// algorithm.
func (a *Auth) authenticateToken(token string) (User, time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return User{}, time.Time{}, fmt.Errorf("%w: malformed JWT", errBadToken)
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return User{}, time.Time{}, fmt.Errorf("%w: bad header encoding", errBadToken)
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil || header.Alg != "HS256" {
		return User{}, time.Time{}, fmt.Errorf("%w: unexpected signing method %q", errBadToken, header.Alg)
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return User{}, time.Time{}, fmt.Errorf("%w: bad signature encoding", errBadToken)
	}
	mac := hmac.New(sha256.New, []byte(a.jwtSecret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return User{}, time.Time{}, fmt.Errorf("%w: signature mismatch", errBadToken)
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return User{}, time.Time{}, fmt.Errorf("%w: bad payload encoding", errBadToken)
	}
	var claims struct {
		Usr  int     `json:"usr"`
		Name string  `json:"name"`
		Exp  float64 `json:"exp"`
		Nbf  float64 `json:"nbf"`
	}
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return User{}, time.Time{}, fmt.Errorf("%w: bad claims", errBadToken)
	}
	now := time.Now()
	var exp time.Time
	if claims.Exp > 0 {
		exp = time.Unix(int64(claims.Exp), 0)
		if now.After(exp) {
			return User{}, time.Time{}, fmt.Errorf("%w: token expired", errBadToken)
		}
	}
	if claims.Nbf > 0 && now.Before(time.Unix(int64(claims.Nbf), 0)) {
		return User{}, time.Time{}, fmt.Errorf("%w: token not valid yet", errBadToken)
	}
	if claims.Usr == 0 {
		return User{}, time.Time{}, fmt.Errorf("%w: missing usr claim", errBadToken)
	}
	return User{Id: strconv.Itoa(claims.Usr), Name: claims.Name}, exp, nil
}

// ── middleware & handlers ──────────────────────────────────────────────

type userCtxKey struct{}

// Middleware gates every route behind authentication. Health endpoints stay
// open for monitoring. API calls receive structured JSON errors; page loads
// are redirected to the login (or enrollment) page.
func (a *Auth) Middleware(next http.Handler) http.Handler {
	if !a.Enabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "/healthz" || p == "/api/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		u, err := a.RequestUser(r)
		if err == nil && !isAllowed(u.Id) {
			// Authenticated, but not in the allowlist (defense in depth: the
			// page load also redirects; this blocks direct API use too).
			err = errNotAllowed
		}
		if err != nil {
			if strings.HasPrefix(p, "/api/") {
				a.writeAuthErr(w, err)
			} else {
				a.redirectAuthErr(w, r, err)
			}
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userCtxKey{}, u)))
	})
}

// authFailStatus maps an authentication error to an HTTP status: 403 when the
// user is authenticated but not enrolled, 401 when they must (re)log in, 502
// when the auth API itself is unavailable.
func authFailStatus(err error) int {
	switch {
	case errors.Is(err, errNotAllowed):
		return http.StatusForbidden
	case errors.Is(err, errNoRefreshToken) || errors.Is(err, errBadToken):
		return http.StatusUnauthorized
	default:
		return http.StatusBadGateway
	}
}

// writeAuthErr writes an auth failure for an API request. A 403 (not
// enrolled) carries the enroll URL and a 401 carries the login URL, so the
// frontend can send the user there.
func (a *Auth) writeAuthErr(w http.ResponseWriter, err error) {
	if errors.Is(err, errNotAllowed) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error(), "redirect": a.enrollURL}, 0)
		return
	}
	status := authFailStatus(err)
	if status == http.StatusUnauthorized && a.loginURL != "" {
		writeJSON(w, status, map[string]string{"error": err.Error(), "login": a.loginURL}, 0)
		return
	}
	writeError(w, status, err.Error())
}

// redirectAuthErr handles an auth failure on a page load: send the browser to
// the login page (or the enrollment page for authenticated non-enrollees).
// Auth-API outages surface as 502 rather than redirect loops.
func (a *Auth) redirectAuthErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errNotAllowed):
		http.Redirect(w, r, a.enrollURL, http.StatusFound)
	case errors.Is(err, errNoRefreshToken) || errors.Is(err, errBadToken):
		http.Redirect(w, r, a.loginURL, http.StatusFound)
	default:
		a.log("auth API failure: %v", err)
		http.Error(w, "authentication service unavailable, please retry", http.StatusBadGateway)
	}
}

// testUser is the identity every visitor gets when auth is disabled (no JWT
// secret configured — local dev / self-hosting), so the signed-in UI is fully
// exercisable without the platform.
var testUser = User{Id: "0", Name: "Test User"}

// handleMe reports the authenticated user for the frontend ("signed in as").
// With auth disabled it signs the visitor in as the test user.
func (a *Auth) handleMe(w http.ResponseWriter, r *http.Request) {
	if !a.Enabled() {
		writeJSON(w, http.StatusOK, map[string]any{
			"id":       testUser.Id,
			"name":     testUser.Name,
			"username": "@testuser",
			"test":     true,
		}, 0)
		return
	}
	u, ok := r.Context().Value(userCtxKey{}).(User)
	if !ok {
		// Reached without passing the middleware (shouldn't happen).
		writeError(w, http.StatusUnauthorized, errNoRefreshToken.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":       u.Id,
		"name":     u.Name,
		"username": allowedUsers[u.Id],
	}, 0)
}

// ── minimal per-key singleflight (stdlib) ──────────────────────────────

type authCall struct {
	done chan struct{}
	user User
	err  error
}

type authFlight struct {
	mu    sync.Mutex
	calls map[string]*authCall
}

// do runs fn once per concurrent key; racing callers wait for and share the
// first caller's result.
func (f *authFlight) do(key string, fn func() (User, error)) (User, error) {
	f.mu.Lock()
	if c, ok := f.calls[key]; ok {
		f.mu.Unlock()
		<-c.done
		return c.user, c.err
	}
	c := &authCall{done: make(chan struct{})}
	f.calls[key] = c
	f.mu.Unlock()

	c.user, c.err = fn()
	close(c.done)

	f.mu.Lock()
	delete(f.calls, key)
	f.mu.Unlock()
	return c.user, c.err
}
