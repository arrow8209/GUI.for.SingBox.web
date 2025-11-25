package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"guiforcores/bridge"
	"guiforcores/pkg/eventbus"
)

//go:embed all:frontend/dist
var distFS embed.FS

type Server struct {
	app        *bridge.App
	bus        *eventbus.Bus
	httpServer *http.Server
	staticFS   http.FileSystem
	shutdown   chan struct{}
}

func NewServer(app *bridge.App, bus *eventbus.Bus) *Server {
	sub, err := fs.Sub(distFS, "frontend/dist")
	if err != nil {
		panic(err)
	}
	server := &Server{
		app:      app,
		bus:      bus,
		staticFS: http.FS(sub),
		shutdown: make(chan struct{}),
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
		s.registerAppRoutes(api)
		api.Route("/files", func(files chi.Router) {
			s.registerFileRoutes(files)
		})
		api.Route("/exec", func(exec chi.Router) {
			s.registerExecRoutes(exec)
		})
		api.Route("/http", func(httpRouter chi.Router) {
			s.registerHTTPRoutes(httpRouter)
		})
		api.Route("/mmdb", func(mmdb chi.Router) {
			s.registerMMDBRoutes(mmdb)
		})
	})

	router.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		s.bus.ServeWS(w, r)
	})

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
