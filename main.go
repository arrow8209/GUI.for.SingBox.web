package main

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"

	"guiforcores/bridge"
	"guiforcores/pkg/eventbus"
	"guiforcores/pkg/security"
)

//go:embed all:frontend/dist
var distFS embed.FS

var (
	hopHeaders     = []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade"}
	coreHTTPClient = &http.Client{Timeout: 30 * time.Second}
)

type Server struct {
	app             *bridge.App
	bus             *eventbus.Bus
	httpServer      *http.Server
	staticFS        http.FileSystem
	shutdown        chan struct{}
	auth            *AuthConfig
	sessions        map[string]time.Time
	csrfTokens      map[string]string // session token -> csrf token
	sessionTTL      time.Duration
	mu              sync.Mutex
	cfg             SecurityConfig
	loginRL         *security.RateLimiter
	originChk       *security.OriginChecker
	activeProfileID string
	profileMu       sync.RWMutex
}

type AuthConfig struct {
	Username           string    `yaml:"username"`
	PasswordHash       string    `yaml:"password_hash"`
	MustChangePassword bool      `yaml:"must_change_password"`
	CreatedAt          time.Time `yaml:"created_at"`
}

// SecurityConfig 公网部署安全相关 env 配置。
type SecurityConfig struct {
	BindAddr       string
	AllowedOrigins []string
	SecureCookie   bool
	SessionTTL     time.Duration
	AdminPassword  string
}

func loadSecurityConfig() SecurityConfig {
	bind := os.Getenv("BIND")
	if bind == "" {
		if p := os.Getenv("PORT"); p != "" {
			bind = "127.0.0.1:" + p
		} else if a := os.Getenv("SERVER_ADDR"); a != "" {
			bind = a
		} else {
			bind = "127.0.0.1:22345"
		}
	}
	origins := []string{"http://127.0.0.1:*", "http://localhost:*"}
	if env := os.Getenv("ALLOWED_ORIGINS"); env != "" {
		origins = nil
		for _, o := range strings.Split(env, ",") {
			origins = append(origins, strings.TrimSpace(o))
		}
	}
	secure := true
	if v := os.Getenv("SECURE_COOKIE"); v == "false" || v == "0" {
		secure = false
	}
	ttl := 24 * time.Hour
	if v := os.Getenv("SESSION_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			ttl = d
		}
	}
	return SecurityConfig{
		BindAddr:       bind,
		AllowedOrigins: origins,
		SecureCookie:   secure,
		SessionTTL:     ttl,
		AdminPassword:  os.Getenv("ADMIN_PASSWORD"),
	}
}

// securityHeaders 在所有响应上设置基础安全 header。
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; connect-src 'self' ws: wss:; "+
				"style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

// loadAuthConfig 加载或初始化管理员认证配置。
// 优先级：env ADMIN_PASSWORD > 已存在 auth.yaml > 首次启动随机密码。
// 旧明文 password 字段会自动迁移为 password_hash；默认弱口令 admin123 会被删除重生。
func loadAuthConfig(adminPwdEnv string) *AuthConfig {
	authDir := filepath.Join(bridge.Env.BasePath, "data")
	authPath := filepath.Join(authDir, "auth.yaml")
	initialPwdPath := filepath.Join(authDir, ".cache", "initial-password.txt")

	if adminPwdEnv != "" {
		hash, err := security.HashPassword(adminPwdEnv)
		if err != nil {
			log.Fatalf("hash ADMIN_PASSWORD: %v", err)
		}
		cfg := &AuthConfig{
			Username:           "admin",
			PasswordHash:       hash,
			MustChangePassword: false,
			CreatedAt:          time.Now().UTC(),
		}
		writeAuthConfig(authPath, cfg)
		_ = os.Remove(initialPwdPath)
		return cfg
	}

	if _, err := os.Stat(authPath); errors.Is(err, os.ErrNotExist) {
		pwd, err := security.GenerateRandomPassword()
		if err != nil {
			log.Fatalf("generate password: %v", err)
		}
		hash, err := security.HashPassword(pwd)
		if err != nil {
			log.Fatalf("hash password: %v", err)
		}
		cfg := &AuthConfig{
			Username:           "admin",
			PasswordHash:       hash,
			MustChangePassword: true,
			CreatedAt:          time.Now().UTC(),
		}
		writeAuthConfig(authPath, cfg)
		_ = os.MkdirAll(filepath.Dir(initialPwdPath), 0700)
		_ = os.WriteFile(initialPwdPath, []byte(pwd), 0600)
		fmt.Fprintf(os.Stderr, "\n========================================\n")
		fmt.Fprintf(os.Stderr, "Initial admin password: %s\n", pwd)
		fmt.Fprintf(os.Stderr, "Username: admin\n")
		fmt.Fprintf(os.Stderr, "Stored in: %s\n", initialPwdPath)
		fmt.Fprintf(os.Stderr, "Login and change immediately.\n")
		fmt.Fprintf(os.Stderr, "========================================\n\n")
		return cfg
	}

	data, err := os.ReadFile(authPath)
	if err != nil {
		log.Fatalf("read auth config: %v", err)
	}
	cfg := &AuthConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		log.Fatalf("parse auth config: %v", err)
	}
	if cfg.PasswordHash != "" {
		return cfg
	}
	// 旧格式（明文 password）→ 迁移
	var legacy struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	}
	if err := yaml.Unmarshal(data, &legacy); err != nil || legacy.Password == "" {
		log.Fatalf("auth.yaml format invalid; delete it and restart")
	}
	if legacy.Password == "admin123" {
		log.Println("WARNING: detected default password 'admin123'; deleting and regenerating")
		_ = os.Remove(authPath)
		return loadAuthConfig(adminPwdEnv)
	}
	hash, err := security.HashPassword(legacy.Password)
	if err != nil {
		log.Fatalf("migrate hash: %v", err)
	}
	cfg.Username = legacy.Username
	cfg.PasswordHash = hash
	cfg.MustChangePassword = false
	cfg.CreatedAt = time.Now().UTC()
	writeAuthConfig(authPath, cfg)
	log.Println("auth.yaml migrated from plain password to argon2id hash")
	return cfg
}

func writeAuthConfig(path string, cfg *AuthConfig) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		log.Printf("create auth dir: %v", err)
		return
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		log.Printf("marshal auth: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		log.Printf("write auth: %v", err)
	}
}

func NewServer(app *bridge.App, bus *eventbus.Bus) *Server {
	sub, err := fs.Sub(distFS, "frontend/dist")
	if err != nil {
		panic(err)
	}
	cfg := loadSecurityConfig()
	authCfg := loadAuthConfig(cfg.AdminPassword)

	server := &Server{
		app:        app,
		bus:        bus,
		staticFS:   http.FS(sub),
		shutdown:   make(chan struct{}),
		auth:       authCfg,
		sessions:   make(map[string]time.Time),
		csrfTokens: make(map[string]string),
		sessionTTL: cfg.SessionTTL,
		cfg:        cfg,
		loginRL:    security.NewRateLimiter(5, time.Minute, 5*time.Minute),
		originChk:  security.NewOriginChecker(cfg.AllowedOrigins),
	}
	app.Exit = server.Shutdown
	return server
}

func (s *Server) Run(addr string) error {
	router := chi.NewRouter()
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	router.Route("/api", func(api chi.Router) {
		api.Post("/login", s.handleLogin)
		api.Group(func(private chi.Router) {
			private.Use(s.authMiddleware)
			private.Use(s.csrfMiddleware)
			s.registerAppRoutes(private)
			private.Route("/files", func(files chi.Router) {
				s.registerFileRoutes(files)
			})
			private.Route("/exec", func(exec chi.Router) {
				s.registerExecRoutes(exec)
			})
			private.Route("/http", func(httpRouter chi.Router) {
				s.registerHTTPRoutes(httpRouter)
			})
			private.Route("/mmdb", func(mmdb chi.Router) {
				s.registerMMDBRoutes(mmdb)
			})
			private.Route("/core", func(core chi.Router) {
				core.HandleFunc("/*", s.handleCoreProxy)
				core.Post("/select-profile", s.handleSelectProfile)
			})
			private.Post("/change-password", s.handleChangePassword)
			private.Post("/logout", s.handleLogout)
		})
	})

	router.HandleFunc("/ws", s.handleWebsocket)

	router.Handle("/*", s.spaHandler())
	router.Handle("/", s.spaHandler())

	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           securityHeaders(router),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		<-s.shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.httpServer.Shutdown(ctx)
	}()

	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown() {
	select {
	case <-s.shutdown:
		return
	default:
		close(s.shutdown)
	}
}

func main() {
	bus := eventbus.New()
	app := bridge.NewApp(bus)
	server := NewServer(app, bus)

	addr := server.cfg.BindAddr

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(addr)
	}()

	log.Printf("Server listening on %s", addr)

	select {
	case <-ctx.Done():
		server.Shutdown()
		if err := <-errCh; err != nil {
			log.Fatalf("server error: %v", err)
		}
	case err := <-errCh:
		if err != nil {
			log.Fatalf("server error: %v", err)
		}
	}
}

// ---- Routing helpers ----

func (s *Server) registerAppRoutes(r chi.Router) {
	r.Get("/env", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, s.app.GetEnv())
	})

	r.Get("/startup", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"startup": s.app.IsStartup()})
	})

	r.Get("/interfaces", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, s.app.GetInterfaces())
	})

	r.Post("/restart", func(w http.ResponseWriter, _ *http.Request) {
		result := s.app.RestartApp()
		writeJSON(w, http.StatusOK, result)
		if result.Flag {
			go func() {
				time.Sleep(500 * time.Millisecond)
				s.Shutdown()
			}()
		}
	})

	r.Post("/exit", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		go s.Shutdown()
	})

	r.Post("/notify", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Title   string               `json:"title"`
			Message string               `json:"message"`
			Icon    string               `json:"icon"`
			Options bridge.NotifyOptions `json:"options"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.Notify(payload.Title, payload.Message, payload.Icon, payload.Options)
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/reality/public-key", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			PrivateKey string `json:"private_key"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		privateKeyBytes, err := decodeRealityPrivateKey(payload.PrivateKey)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		curve := ecdh.X25519()
		privateKey, err := curve.NewPrivateKey(privateKeyBytes)
		if err != nil {
			writeJSONError(w, err)
			return
		}
		publicKey := privateKey.PublicKey().Bytes()
		writeJSON(w, http.StatusOK, map[string]string{
			"public_key": base64.RawStdEncoding.EncodeToString(publicKey),
		})
	})
}

// extractClientIP 从 X-Forwarded-For（反代场景）或 RemoteAddr 提取客户端 IP。
func extractClientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		if i := strings.Index(xf, ","); i > 0 {
			return strings.TrimSpace(xf[:i])
		}
		return strings.TrimSpace(xf)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.SecureCookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(s.cfg.SessionTTL.Seconds()),
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.SecureCookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   s.cfg.SecureCookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

func (s *Server) setCSRFCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    token,
		Path:     "/",
		HttpOnly: false, // 前端 JS 需要读
		Secure:   s.cfg.SecureCookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(s.cfg.SessionTTL.Seconds()),
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	clientIP := extractClientIP(r)
	if !s.loginRL.Allow("ip:" + clientIP) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts, try later"})
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !s.loginRL.Allow("user:" + body.Username) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts, try later"})
		return
	}
	if body.Username != s.auth.Username || !security.VerifyPassword(body.Password, s.auth.PasswordHash) {
		s.loginRL.RecordFailure("ip:" + clientIP)
		s.loginRL.RecordFailure("user:" + body.Username)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	s.loginRL.RecordSuccess("ip:" + clientIP)
	s.loginRL.RecordSuccess("user:" + body.Username)

	sessionToken, err := security.NewCSRFToken()
	if err != nil {
		writeJSONError(w, err)
		return
	}
	csrfToken, err := security.NewCSRFToken()
	if err != nil {
		writeJSONError(w, err)
		return
	}
	s.mu.Lock()
	s.sessions[sessionToken] = time.Now().Add(s.cfg.SessionTTL)
	s.csrfTokens[sessionToken] = csrfToken
	s.mu.Unlock()

	s.setSessionCookie(w, sessionToken)
	s.setCSRFCookie(w, csrfToken)

	writeJSON(w, http.StatusOK, map[string]any{
		"csrfToken":          csrfToken,
		"mustChangePassword": s.auth.MustChangePassword,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session"); err == nil {
		s.mu.Lock()
		delete(s.sessions, cookie.Value)
		delete(s.csrfTokens, cookie.Value)
		s.mu.Unlock()
	}
	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeJSONError(w, err)
		return
	}
	if !security.VerifyPassword(body.OldPassword, s.auth.PasswordHash) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "old password incorrect"})
		return
	}
	if len(body.NewPassword) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "new password too short (min 8)"})
		return
	}
	hash, err := security.HashPassword(body.NewPassword)
	if err != nil {
		writeJSONError(w, err)
		return
	}
	s.auth.PasswordHash = hash
	s.auth.MustChangePassword = false
	authPath := filepath.Join(bridge.Env.BasePath, "data", "auth.yaml")
	writeAuthConfig(authPath, s.auth)
	_ = os.Remove(filepath.Join(bridge.Env.BasePath, "data", ".cache", "initial-password.txt"))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSelectProfile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProfileID string `json:"profileId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeJSONError(w, err)
		return
	}
	if body.ProfileID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profileId required"})
		return
	}
	s.profileMu.Lock()
	s.activeProfileID = body.ProfileID
	s.profileMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string
		if cookie, err := r.Cookie("session"); err == nil {
			token = cookie.Value
		}
		if token == "" || !s.validateToken(token) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// csrfMiddleware 校验 CSRF 双提交：状态变更方法必须带 X-CSRF-Token 与 csrf_token cookie 一致。
func (s *Server) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		// WebSocket upgrade 走 GET，但已在 authMiddleware 后；不需要 CSRF。
		if websocket.IsWebSocketUpgrade(r) {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie("csrf_token")
		if err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "missing csrf cookie"})
			return
		}
		header := r.Header.Get("X-CSRF-Token")
		if !security.CompareCSRF(cookie.Value, header) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "csrf mismatch"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) validateToken(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	expiry, ok := s.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		delete(s.sessions, token)
		delete(s.csrfTokens, token)
		return false
	}
	return true
}

func (s *Server) handleWebsocket(w http.ResponseWriter, r *http.Request) {
	// Cookie 自动带 session；WS Origin 检查在 bus.ServeWS 内部 upgrader 处理。
	var token string
	if cookie, err := r.Cookie("session"); err == nil {
		token = cookie.Value
	}
	if token == "" || !s.validateToken(token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !s.originChk.Allow(r.Header.Get("Origin")) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}
	s.bus.ServeWS(w, r)
}

func (s *Server) registerFileRoutes(r chi.Router) {
	type pathPayload struct {
		Path string `json:"path"`
	}
	type pathModePayload struct {
		Path string `json:"path"`
		Mode string `json:"mode"`
	}
	type writePayload struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Mode    string `json:"mode"`
	}
	type movePayload struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}
	type unzipPayload struct {
		Path   string `json:"path"`
		Output string `json:"output"`
	}

	r.Post("/read", func(w http.ResponseWriter, r *http.Request) {
		var payload pathModePayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.ReadFile(payload.Path, bridge.IOOptions{Mode: payload.Mode})
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/write", func(w http.ResponseWriter, r *http.Request) {
		var payload writePayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.WriteFile(payload.Path, payload.Content, bridge.IOOptions{Mode: payload.Mode})
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/move", func(w http.ResponseWriter, r *http.Request) {
		var payload movePayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.MoveFile(payload.Source, payload.Target)
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/remove", func(w http.ResponseWriter, r *http.Request) {
		var payload pathPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.RemoveFile(payload.Path)
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/copy", func(w http.ResponseWriter, r *http.Request) {
		var payload movePayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.CopyFile(payload.Source, payload.Target)
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/mkdir", func(w http.ResponseWriter, r *http.Request) {
		var payload pathPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.MakeDir(payload.Path)
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/list", func(w http.ResponseWriter, r *http.Request) {
		var payload pathPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.ReadDir(payload.Path)
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/absolute", func(w http.ResponseWriter, r *http.Request) {
		var payload pathPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.AbsolutePath(payload.Path)
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/exists", func(w http.ResponseWriter, r *http.Request) {
		var payload pathPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.FileExists(payload.Path)
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/unzip/zip", func(w http.ResponseWriter, r *http.Request) {
		var payload unzipPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.UnzipZIPFile(payload.Path, payload.Output)
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/unzip/gz", func(w http.ResponseWriter, r *http.Request) {
		var payload unzipPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.UnzipGZFile(payload.Path, payload.Output)
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/unzip/targz", func(w http.ResponseWriter, r *http.Request) {
		var payload unzipPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.UnzipTarGZFile(payload.Path, payload.Output)
		writeJSON(w, http.StatusOK, resp)
	})
}

func (s *Server) registerExecRoutes(r chi.Router) {
	type execPayload struct {
		Path    string             `json:"path"`
		Args    []string           `json:"args"`
		Options bridge.ExecOptions `json:"options"`
	}
	type execBgPayload struct {
		Path     string             `json:"path"`
		Args     []string           `json:"args"`
		OutEvent string             `json:"outEvent"`
		EndEvent string             `json:"endEvent"`
		Options  bridge.ExecOptions `json:"options"`
	}
	type pidPayload struct {
		PID int `json:"pid"`
	}
	type killPayload struct {
		PID     int `json:"pid"`
		Timeout int `json:"timeout"`
	}

	r.Post("/run", func(w http.ResponseWriter, r *http.Request) {
		var payload execPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.Exec(payload.Path, payload.Args, payload.Options)
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/background", func(w http.ResponseWriter, r *http.Request) {
		var payload execBgPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.ExecBackground(payload.Path, payload.Args, payload.OutEvent, payload.EndEvent, payload.Options)
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/process-info", func(w http.ResponseWriter, r *http.Request) {
		var payload pidPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.ProcessInfo(int32(payload.PID))
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/process-memory", func(w http.ResponseWriter, r *http.Request) {
		var payload pidPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.ProcessMemory(int32(payload.PID))
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/kill", func(w http.ResponseWriter, r *http.Request) {
		var payload killPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.KillProcess(payload.PID, payload.Timeout)
		writeJSON(w, http.StatusOK, resp)
	})
}

func (s *Server) registerHTTPRoutes(r chi.Router) {
	type reqPayload struct {
		Method  string                `json:"method"`
		URL     string                `json:"url"`
		Headers map[string]string     `json:"headers"`
		Body    string                `json:"body"`
		Options bridge.RequestOptions `json:"options"`
	}
	type downloadPayload struct {
		Method  string                `json:"method"`
		URL     string                `json:"url"`
		Path    string                `json:"path"`
		Event   string                `json:"event"`
		Headers map[string]string     `json:"headers"`
		Options bridge.RequestOptions `json:"options"`
	}

	r.Post("/request", func(w http.ResponseWriter, r *http.Request) {
		var payload reqPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.Requests(payload.Method, payload.URL, payload.Headers, payload.Body, payload.Options)
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/download", func(w http.ResponseWriter, r *http.Request) {
		var payload downloadPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.Download(payload.Method, payload.URL, payload.Path, payload.Headers, payload.Event, payload.Options)
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/upload", func(w http.ResponseWriter, r *http.Request) {
		var payload downloadPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.Upload(payload.Method, payload.URL, payload.Path, payload.Headers, payload.Event, payload.Options)
		writeJSON(w, http.StatusOK, resp)
	})
}
func (s *Server) registerMMDBRoutes(r chi.Router) {
	type openPayload struct {
		Path string `json:"path"`
		ID   string `json:"id"`
	}
	type queryPayload struct {
		Path string `json:"path"`
		IP   string `json:"ip"`
		Type string `json:"type"`
	}

	r.Post("/open", func(w http.ResponseWriter, r *http.Request) {
		var payload openPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.OpenMMDB(payload.Path, payload.ID)
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/close", func(w http.ResponseWriter, r *http.Request) {
		var payload openPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.CloseMMDB(payload.Path, payload.ID)
		writeJSON(w, http.StatusOK, resp)
	})

	r.Post("/query", func(w http.ResponseWriter, r *http.Request) {
		var payload queryPayload
		if err := decodeJSON(r, &payload); err != nil {
			writeJSONError(w, err)
			return
		}
		resp := s.app.QueryMMDB(payload.Path, payload.IP, payload.Type)
		writeJSON(w, http.StatusOK, resp)
	})
}

// handleCoreProxy 把 /api/core/* 请求代理到激活 profile 配置的 sing-box clash_api。
// bearer 与 base 来自服务端读 data/profiles.yaml，前端不再透传。
func (s *Server) handleCoreProxy(w http.ResponseWriter, r *http.Request) {
	profile, err := s.readActiveProfile()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	coreBase := profile.Experimental.ClashAPI.ExternalController
	bearer := profile.Experimental.ClashAPI.Secret
	if coreBase == "" {
		http.Error(w, "core base not configured in active profile", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(coreBase, "http") {
		coreBase = "http://" + coreBase
	}
	baseURL, err := url.Parse(coreBase)
	if err != nil {
		http.Error(w, "invalid core base", http.StatusBadRequest)
		return
	}
	if !isLoopbackHost(baseURL.Hostname()) {
		http.Error(w, "core base must be loopback", http.StatusForbidden)
		return
	}
	pathParam := chi.URLParam(r, "*")
	if !strings.HasPrefix(pathParam, "/") {
		pathParam = "/" + pathParam
	}
	rel := &url.URL{Path: pathParam, RawQuery: r.URL.RawQuery}
	targetURL := baseURL.ResolveReference(rel)
	if websocket.IsWebSocketUpgrade(r) {
		s.proxyCoreWebsocket(w, r, targetURL, bearer)
		return
	}
	s.proxyCoreHTTP(w, r, targetURL, bearer)
}

// miniProfile 仅解析 profiles.yaml 中需要的字段。
type miniProfile struct {
	ID           string `yaml:"id"`
	Experimental struct {
		ClashAPI struct {
			ExternalController string `yaml:"external_controller"`
			Secret             string `yaml:"secret"`
		} `yaml:"clash_api"`
	} `yaml:"experimental"`
}

func (s *Server) readActiveProfile() (*miniProfile, error) {
	s.profileMu.RLock()
	id := s.activeProfileID
	s.profileMu.RUnlock()
	if id == "" {
		return nil, errors.New("no active profile selected; call /api/core/select-profile first")
	}
	path := filepath.Join(bridge.Env.BasePath, "data", "profiles.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var profiles []miniProfile
	if err := yaml.Unmarshal(data, &profiles); err != nil {
		return nil, err
	}
	for i := range profiles {
		if profiles[i].ID == id {
			return &profiles[i], nil
		}
	}
	return nil, fmt.Errorf("profile %s not found", id)
}

func (s *Server) proxyCoreHTTP(w http.ResponseWriter, r *http.Request, target *url.URL, bearer string) {
	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, _ = io.ReadAll(r.Body)
		r.Body.Close()
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	copyHeaders(req.Header, r.Header)
	req.Header.Del("Host")
	req.Header.Del("Content-Length")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := coreHTTPClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (s *Server) proxyCoreWebsocket(w http.ResponseWriter, r *http.Request, target *url.URL, bearer string) {
	wsURL := *target
	switch wsURL.Scheme {
	case "http":
		wsURL.Scheme = "ws"
	case "https":
		wsURL.Scheme = "wss"
	}
	header := http.Header{}
	if bearer != "" {
		header.Set("Authorization", "Bearer "+bearer)
	}
	backendConn, resp, err := websocket.DefaultDialer.Dial(wsURL.String(), header)
	if err != nil {
		status := http.StatusBadGateway
		message := err.Error()
		if resp != nil {
			status = resp.StatusCode
			message = resp.Status
		}
		http.Error(w, message, status)
		return
	}
	upgrader := websocket.Upgrader{CheckOrigin: func(req *http.Request) bool {
		return s.originChk.Allow(req.Header.Get("Origin"))
	}}
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		backendConn.Close()
		return
	}
	errCh := make(chan error, 2)
	go proxyWebsocketPump(clientConn, backendConn, errCh)
	go proxyWebsocketPump(backendConn, clientConn, errCh)
	<-errCh
	backendConn.Close()
	clientConn.Close()
}

func proxyWebsocketPump(src, dst *websocket.Conn, errCh chan<- error) {
	for {
		msgType, msg, err := src.ReadMessage()
		if err != nil {
			errCh <- err
			return
		}
		if err := dst.WriteMessage(msgType, msg); err != nil {
			errCh <- err
			return
		}
	}
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		ignore := false
		for _, hop := range hopHeaders {
			if strings.EqualFold(key, hop) {
				ignore = true
				break
			}
		}
		if ignore {
			continue
		}
		for _, v := range values {
			dst.Add(key, v)
		}
	}
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ---- Utilities ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return errors.New("empty body")
	}
	return json.Unmarshal(body, v)
}

func decodeRealityPrivateKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("empty private key")
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(padBase64(value)); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.URLEncoding.DecodeString(padBase64(value)); err == nil {
		return decoded, nil
	}
	value = strings.TrimPrefix(value, "0x")
	if decoded, err := hex.DecodeString(value); err == nil {
		return decoded, nil
	}
	return nil, errors.New("invalid private key encoding")
}

func padBase64(value string) string {
	if value == "" {
		return value
	}
	padding := len(value) % 4
	if padding == 0 {
		return value
	}
	return value + strings.Repeat("=", 4-padding)
}

func (s *Server) spaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file := strings.TrimPrefix(r.URL.Path, "/")
		if file == "" || strings.HasPrefix(file, "api/") || file == "ws" {
			file = "index.html"
		}

		f, err := s.staticFS.Open(file)
		if err != nil {
			if !os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			f, err = s.staticFS.Open("index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}

		http.ServeContent(w, r, path.Base(file), info.ModTime(), f)
	}
}
