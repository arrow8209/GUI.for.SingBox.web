package main

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
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
)

//go:embed all:frontend/dist
var distFS embed.FS

var (
	hopHeaders     = []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade"}
	coreHTTPClient = &http.Client{Timeout: 30 * time.Second}
)

type Server struct {
	app        *bridge.App
	bus        *eventbus.Bus
	httpServer *http.Server
	staticFS   http.FileSystem
	shutdown   chan struct{}
	auth       *AuthConfig
	sessions   map[string]time.Time
	sessionTTL time.Duration
	mu         sync.Mutex
}

type AuthConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func loadAuthConfig() *AuthConfig {
	path := filepath.Join(bridge.Env.BasePath, "data", "auth.yaml")
	cfg := &AuthConfig{
		Username: "admin",
		Password: "admin123",
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		writeAuthConfig(path, cfg)
		return cfg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("failed to read auth config: %v", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		log.Fatalf("failed to parse auth config: %v", err)
	}
	return cfg
}

func writeAuthConfig(path string, cfg *AuthConfig) {
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		log.Printf("failed to create auth config directory: %v", err)
		return
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		log.Printf("failed to marshal auth config: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("failed to write auth config: %v", err)
	}
}

func NewServer(app *bridge.App, bus *eventbus.Bus) *Server {
	sub, err := fs.Sub(distFS, "frontend/dist")
	if err != nil {
		panic(err)
	}
	authCfg := loadAuthConfig()

	server := &Server{
		app:        app,
		bus:        bus,
		staticFS:   http.FS(sub),
		shutdown:   make(chan struct{}),
		auth:       authCfg,
		sessions:   make(map[string]time.Time),
		sessionTTL: 24 * time.Hour,
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
			})
			private.Post("/logout", s.handleLogout)
		})
	})

	router.HandleFunc("/ws", s.handleWebsocket)

	router.Handle("/*", s.spaHandler())
	router.Handle("/", s.spaHandler())

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: router,
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

	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "22345"
		}
		addr = ":" + port
	}

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

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	var body payload
	if err := decodeJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if body.Username != s.auth.Username || body.Password != s.auth.Password {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	token := s.generateToken()
	s.mu.Lock()
	s.sessions[token] = time.Now().Add(s.sessionTTL)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := getBearerToken(r.Header.Get("Authorization"))
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := getBearerToken(r.Header.Get("Authorization"))
		if token == "" && websocket.IsWebSocketUpgrade(r) {
			token = r.URL.Query().Get("token")
		}
		if token == "" || !s.validateToken(token) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) generateToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(buf)
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
		return false
	}
	return true
}

func (s *Server) handleWebsocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" || !s.validateToken(token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s.bus.ServeWS(w, r)
}

func getBearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
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

func (s *Server) handleCoreProxy(w http.ResponseWriter, r *http.Request) {
	coreBase := r.Header.Get("X-Core-Base")
	if coreBase == "" {
		coreBase = r.URL.Query().Get("coreBase")
	}
	if coreBase == "" {
		http.Error(w, "missing core base", http.StatusBadRequest)
		return
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
	query := r.URL.Query()
	query.Del("coreBase")
	query.Del("coreBearer")
	query.Del("token")
	rel := &url.URL{Path: pathParam, RawQuery: query.Encode()}
	targetURL := baseURL.ResolveReference(rel)
	bearer := r.Header.Get("X-Core-Bearer")
	if bearer == "" {
		bearer = r.URL.Query().Get("coreBearer")
	}
	if websocket.IsWebSocketUpgrade(r) {
		s.proxyCoreWebsocket(w, r, targetURL, bearer)
		return
	}
	s.proxyCoreHTTP(w, r, targetURL, bearer)
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
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
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
