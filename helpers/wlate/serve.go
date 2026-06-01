//go:build ignore

package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/luisfurquim/goose"
	"github.com/luisfurquim/wings/wi18n"
)

// G is this binary's goose alert. wings.json may set the level via SetConfig.
var G goose.Alert = goose.Alert(2)

type serverConfig struct {
	cert         string
	listen       string
	root         string
	dictStateDir string
	oauth2       oauth2Config
	hasAuth      bool
}

type oauth2Config struct {
	issuer       string
	clientID     string
	clientSecret string
	redirectURL  string
	allowedFile  string
}

var (
	reKV      = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*?)\s*$`)
	reComment = regexp.MustCompile(`^\s*(#.*)?$`)
)

func loadConfig(path string) (*serverConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	cfg := &serverConfig{}
	for i, line := range strings.Split(string(data), "\n") {
		if reComment.MatchString(line) {
			continue
		}
		m := reKV.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("%s:%d: invalid line: %q", path, i+1, line)
		}
		val := strings.Trim(m[2], `"'`)
		switch m[1] {
		case "cert":
			cfg.cert = val
		case "listen":
			cfg.listen = val
		case "root":
			cfg.root = val
		case "dict_state_dir":
			cfg.dictStateDir = val
		case "oauth2_issuer":
			cfg.oauth2.issuer = val
			cfg.hasAuth = true
		case "oauth2_client_id":
			cfg.oauth2.clientID = val
			cfg.hasAuth = true
		case "oauth2_client_secret":
			cfg.oauth2.clientSecret = val
			cfg.hasAuth = true
		case "oauth2_redirect_url":
			cfg.oauth2.redirectURL = val
			cfg.hasAuth = true
		case "oauth2_allowed":
			cfg.oauth2.allowedFile = val
			cfg.hasAuth = true
		default:
			return nil, fmt.Errorf("%s:%d: unknown key %q", path, i+1, m[1])
		}
	}
	if cfg.hasAuth {
		missing := []string{}
		if cfg.oauth2.issuer == "" {
			missing = append(missing, "oauth2_issuer")
		}
		if cfg.oauth2.clientID == "" {
			missing = append(missing, "oauth2_client_id")
		}
		if cfg.oauth2.clientSecret == "" {
			missing = append(missing, "oauth2_client_secret")
		}
		if cfg.oauth2.redirectURL == "" {
			missing = append(missing, "oauth2_redirect_url")
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("%s: OAuth2 requires: %s", path, strings.Join(missing, ", "))
		}
	}
	return cfg, nil
}

func loadTLSCert(dir, name string) (tls.Certificate, error) {
	certPath := filepath.Join(dir, name+".crt")
	if _, err := os.Stat(certPath); err != nil {
		if !os.IsNotExist(err) {
			return tls.Certificate{}, err
		}
		certPath = filepath.Join(dir, name+".pem")
		if _, err := os.Stat(certPath); err != nil {
			return tls.Certificate{}, fmt.Errorf("cert file not found: %s.crt nor %s.pem in %s", name, name, dir)
		}
	}

	if cert, err := tls.LoadX509KeyPair(certPath, certPath); err == nil {
		return cert, nil
	}

	keyPath := filepath.Join(dir, name+".key")
	return tls.LoadX509KeyPair(certPath, keyPath)
}

// ----------------- OAuth2 / OIDC -----------------

type oidcMetadata struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
}

type authManager struct {
	cfg         oauth2Config
	meta        oidcMetadata
	allowedPath string

	mu       sync.RWMutex
	sessions map[string]session
	pending  map[string]pending
	allowed  map[string]bool
}

type session struct {
	email   string
	expires time.Time
}

type pending struct {
	redirect string
	expires  time.Time
}

const (
	sessionCookie = "wlate_session"
	sessionTTL    = 12 * time.Hour
	pendingTTL    = 10 * time.Minute
)

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func newAuthManager(cfg oauth2Config) (*authManager, error) {
	resp, err := http.Get(strings.TrimRight(cfg.issuer, "/") + "/.well-known/openid-configuration")
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC discovery: HTTP %d", resp.StatusCode)
	}
	var meta oidcMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("OIDC discovery decode: %w", err)
	}
	if meta.AuthorizationEndpoint == "" || meta.TokenEndpoint == "" || meta.UserinfoEndpoint == "" {
		return nil, fmt.Errorf("OIDC discovery: missing endpoints (provider must expose userinfo_endpoint)")
	}

	am := &authManager{
		cfg:         cfg,
		meta:        meta,
		allowedPath: cfg.allowedFile,
		sessions:    map[string]session{},
		pending:     map[string]pending{},
	}
	if err := am.reloadAllowed(); err != nil {
		return nil, err
	}
	return am, nil
}

func (a *authManager) reloadAllowed() error {
	if a.allowedPath == "" {
		a.mu.Lock()
		a.allowed = nil
		a.mu.Unlock()
		return nil
	}
	data, err := os.ReadFile(a.allowedPath)
	if err != nil {
		return fmt.Errorf("oauth2_allowed file: %w", err)
	}
	set := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		e := strings.ToLower(strings.TrimSpace(line))
		if e == "" || strings.HasPrefix(e, "#") {
			continue
		}
		set[e] = true
	}
	a.mu.Lock()
	a.allowed = set
	a.mu.Unlock()
	return nil
}

func (a *authManager) isAllowed(email string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.allowed == nil {
		return true
	}
	return a.allowed[strings.ToLower(email)]
}

func (a *authManager) sessionEmail(r *http.Request) (string, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return "", false
	}
	a.mu.RLock()
	s, ok := a.sessions[c.Value]
	a.mu.RUnlock()
	if !ok || time.Now().After(s.expires) {
		return "", false
	}
	return s.email, true
}

func (a *authManager) startLogin(w http.ResponseWriter, r *http.Request, redirect string) {
	state := randomToken(16)
	a.mu.Lock()
	a.pending[state] = pending{redirect: redirect, expires: time.Now().Add(pendingTTL)}
	a.mu.Unlock()

	q := url.Values{}
	q.Set("client_id", a.cfg.clientID)
	q.Set("redirect_uri", a.cfg.redirectURL)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	http.Redirect(w, r, a.meta.AuthorizationEndpoint+"?"+q.Encode(), http.StatusFound)
}

func (a *authManager) handleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		http.Error(w, "missing state/code", http.StatusBadRequest)
		return
	}
	a.mu.Lock()
	p, ok := a.pending[state]
	delete(a.pending, state)
	a.mu.Unlock()
	if !ok || time.Now().After(p.expires) {
		http.Error(w, "invalid or expired state", http.StatusBadRequest)
		return
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", a.cfg.redirectURL)
	form.Set("client_id", a.cfg.clientID)
	form.Set("client_secret", a.cfg.clientSecret)

	req, _ := http.NewRequest(http.MethodPost, a.meta.TokenEndpoint, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "token request: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf("token endpoint HTTP %d: %s", resp.StatusCode, body), http.StatusBadGateway)
		return
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		http.Error(w, "token decode: "+err.Error(), http.StatusBadGateway)
		return
	}

	uiReq, _ := http.NewRequest(http.MethodGet, a.meta.UserinfoEndpoint, nil)
	uiReq.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	uiResp, err := http.DefaultClient.Do(uiReq)
	if err != nil {
		http.Error(w, "userinfo request: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer uiResp.Body.Close()
	if uiResp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("userinfo HTTP %d", uiResp.StatusCode), http.StatusBadGateway)
		return
	}
	var ui struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := json.NewDecoder(uiResp.Body).Decode(&ui); err != nil {
		http.Error(w, "userinfo decode: "+err.Error(), http.StatusBadGateway)
		return
	}
	if ui.Email == "" {
		http.Error(w, "userinfo: no email claim", http.StatusForbidden)
		return
	}
	if !a.isAllowed(ui.Email) {
		http.Error(w, "access denied for "+ui.Email, http.StatusForbidden)
		return
	}

	sid := randomToken(32)
	a.mu.Lock()
	a.sessions[sid] = session{email: ui.Email, expires: time.Now().Add(sessionTTL)}
	a.mu.Unlock()

	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionTTL),
	})

	dest := p.redirect
	if dest == "" {
		dest = "/"
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

func (a *authManager) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.mu.Lock()
		delete(a.sessions, c.Value)
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *authManager) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/oauth2/") {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := a.sessionEmail(r); ok {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		a.startLogin(w, r, r.URL.RequestURI())
	})
}

// defaultDictStateDir returns the same default cache directory as dictbuild's
// -state-dir flag, so serve.go finds provider avatars without configuration.
func defaultDictStateDir() string {
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "wings", "dictbuild")
	}
	return filepath.Join(os.Getenv("HOME"), ".cache", "wings", "dictbuild")
}

// reName restricts avatar file names to alphanumerics, hyphens, and
// underscores — no slashes, no dots, no path traversal possible.
var reName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// llmAvatarSVG is the fallback badge served for any /avatar/llm-* request
// when no per-model PNG file exists in the avatars directory.
const llmAvatarSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20">` +
	`<circle cx="10" cy="10" r="9" fill="#7c3aed"/>` +
	`<text x="10" y="14" text-anchor="middle" font-family="system-ui,sans-serif" ` +
	`font-size="8" font-weight="700" fill="#fff">AI</text>` +
	`</svg>`

// requiredAppFiles are the front-end bundle artifacts build.sh produces in the
// app directory (dist/). Their absence means the server was started before a
// build, or from the wrong working directory.
var requiredAppFiles = []string{"index.html", "main.wasm", "prana_helper.js", "wasm_exec.js"}

// validateLayout fails fast when the directories the server depends on are not
// laid out as expected. appDir holds the front-end bundle served as the web
// root; projectRoot holds the translation project (wings.json + the i18n/
// catalogs). The most common mistake — launching without the project-root
// argument, so it defaults to "." — leaves wings.json and i18n/ unreachable and
// the UI silently empty; this turns that into one clear error instead of a wall
// of runtime 404s. Returns nil when both directories are conformant.
func validateLayout(appDir, projectRoot string) error {
	var problems []string

	if fi, err := os.Stat(appDir); err != nil || !fi.IsDir() {
		problems = append(problems, fmt.Sprintf("app bundle directory %q not found — run ./build.sh and launch from helpers/wlate/", appDir))
	} else {
		for _, name := range requiredAppFiles {
			if _, err := os.Stat(filepath.Join(appDir, name)); err != nil {
				problems = append(problems, fmt.Sprintf("%s missing from app bundle %q — run ./build.sh", name, appDir))
			}
		}
	}

	if fi, err := os.Stat(projectRoot); err != nil || !fi.IsDir() {
		problems = append(problems, fmt.Sprintf("project root %q is not a directory", projectRoot))
	} else {
		if _, err := os.Stat(filepath.Join(projectRoot, "wings.json")); err != nil {
			problems = append(problems, fmt.Sprintf("wings.json missing from project root %q", projectRoot))
		}
		if fi, err := os.Stat(filepath.Join(projectRoot, "i18n")); err != nil || !fi.IsDir() {
			problems = append(problems, fmt.Sprintf("i18n/ directory missing from project root %q", projectRoot))
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("startup layout check failed:\n  - %s\n\n"+
		"Usage: serve <project-root>   (e.g. run `go run serve.go ./dist` from helpers/wlate/)\n"+
		"The project root holds wings.json and i18n/; the app bundle is served from %q.",
		strings.Join(problems, "\n  - "), appDir)
}

// ----------------- main -----------------

func main() {
	exe, err := os.Executable()
	if err != nil {
		G.Fatalf(1, "cannot determine executable path: %v", err)
	}
	exeDir := filepath.Dir(exe)

	cfg, err := loadConfig(filepath.Join(exeDir, "server.conf"))
	if err != nil {
		G.Fatalf(1, "server.conf error: %v", err)
	}
	if cfg == nil {
		cfg = &serverConfig{}
	}

	projectRoot := "."
	if len(os.Args) > 1 {
		projectRoot = os.Args[1]
	}
	if cfg.root != "" {
		projectRoot = cfg.root
	}

	// appDir holds the wlate front-end bundle (served as the web root); it is
	// relative to the working directory, so the server must be launched from
	// helpers/wlate/. Fail fast with an actionable message when the bundle or
	// the project layout is wrong — the alternative is a silently empty UI and
	// a wall of 404s (the project root defaulting to "." is the usual cause).
	appDir := "dist"
	if err := validateLayout(appDir, projectRoot); err != nil {
		G.Fatalf(1, "%v", err)
	}

	// Apply project-wide debug settings from wings.json (if present).
	if data, err := os.ReadFile(filepath.Join(projectRoot, "wings.json")); err == nil {
		if err := wi18n.SetConfig(data); err != nil {
			G.Logf(1, "wings.json: %v", err)
		}
		wi18n.ConfigureGoose(&G)
	}

	listenAddr := ":8080"
	if cfg.listen != "" {
		listenAddr = cfg.listen
	}

	dictStateDir := defaultDictStateDir()
	if cfg.dictStateDir != "" {
		dictStateDir = cfg.dictStateDir
	}

	fs := http.FileServer(http.Dir(appDir))

	mux := http.NewServeMux()

	var am *authManager
	if cfg.hasAuth {
		am, err = newAuthManager(cfg.oauth2)
		if err != nil {
			G.Logf(1, "OAuth2 init: %v", err)
			os.Exit(1)
		}
		mux.HandleFunc("/oauth2/callback", am.handleCallback)
		mux.HandleFunc("/oauth2/logout", am.handleLogout)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// POST /save?file=i18n/en-US.json — write JSON to project directory
		if r.Method == http.MethodPost && r.URL.Path == "/save" {
			file := r.URL.Query().Get("file")
			if file == "" {
				http.Error(w, "missing file parameter", http.StatusBadRequest)
				return
			}
			clean := filepath.Clean(file)
			if !strings.HasPrefix(clean, "i18n/") || !strings.HasSuffix(clean, ".json") {
				http.Error(w, "file must be under i18n/ and end with .json", http.StatusBadRequest)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			target := filepath.Join(projectRoot, clean)
			abs, _ := filepath.Abs(target)
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := os.WriteFile(target, body, 0644); err != nil {
				G.Logf(1, "save: WriteFile %s failed: %v", abs, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			st, statErr := os.Stat(target)
			if statErr != nil {
				G.Logf(1, "save: stat after write %s: %v", abs, statErr)
			} else {
				G.Logf(2, "save: wrote %s (%d bytes; mtime=%s; on-disk size=%d)",
					abs, len(body), st.ModTime().Format(time.RFC3339Nano), st.Size())
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "saved %s (%d bytes)\n", clean, len(body))
			return
		}

		if strings.HasPrefix(r.URL.Path, "/avatar/") {
			name := strings.TrimPrefix(r.URL.Path, "/avatar/")
			if !reName.MatchString(name) {
				http.Error(w, "invalid avatar name", http.StatusBadRequest)
				return
			}
			avatarPath := filepath.Join(dictStateDir, "avatars", name+".png")
			if _, err := os.Stat(avatarPath); err != nil {
				if strings.HasPrefix(name, "llm-") {
					w.Header().Set("Content-Type", "image/svg+xml")
					w.Header().Set("Cache-Control", "public, max-age=86400")
					fmt.Fprint(w, llmAvatarSVG)
					return
				}
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "public, max-age=86400")
			http.ServeFile(w, r, avatarPath)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/i18n/") {
			target := filepath.Join(projectRoot, r.URL.Path)
			abs, _ := filepath.Abs(target)
			if st, err := os.Stat(target); err == nil {
				G.Logf(3, "serve i18n: %s -> %s (size=%d, mtime=%s)",
					r.URL.Path, abs, st.Size(), st.ModTime().Format(time.RFC3339Nano))
			} else {
				G.Logf(2, "serve i18n: %s -> %s (stat error: %v)", r.URL.Path, abs, err)
			}
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFile(w, r, target)
			return
		}

		if r.URL.Path == "/wings.json" {
			target := filepath.Join(projectRoot, "wings.json")
			http.ServeFile(w, r, target)
			return
		}

		if strings.HasSuffix(r.URL.Path, ".wasm") {
			w.Header().Set("Content-Type", "application/wasm")
		}
		fs.ServeHTTP(w, r)
	})

	var handler http.Handler = mux
	if am != nil {
		handler = am.middleware(mux)
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		G.Fatalf(1, "listen error: %v", err)
	}
	defer ln.Close()

	scheme := "http"
	if cfg.cert != "" {
		cert, err := loadTLSCert(exeDir, cfg.cert)
		if err != nil {
			G.Fatalf(1, "TLS cert error: %v", err)
		}
		ln = tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{cert}})
		scheme = "https"
	}

	G.Logf(2, "wlate server listening on %s://%s", scheme, ln.Addr())
	G.Logf(2, "Project root: %s", projectRoot)
	if am != nil {
		G.Logf(2, "OAuth2 enabled (issuer=%s)", cfg.oauth2.issuer)
		if cfg.oauth2.allowedFile != "" {
			G.Logf(2, "Allowed emails from: %s", cfg.oauth2.allowedFile)
		}
	}

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return context.Background() },
	}
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		G.Logf(1, "serve error: %v", err)
		os.Exit(1)
	}
}
