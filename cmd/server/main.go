// Command server is the ai-chattermax platform: a multi-tenant chat server for
// humans and AI personas. See .spec/proposals/platform-design.md.
package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	static "github.com/cpuchip/ai-chattermax"
	"github.com/cpuchip/ai-chattermax/internal/auth"
	"github.com/cpuchip/ai-chattermax/internal/config"
	"github.com/cpuchip/ai-chattermax/internal/db"
	"github.com/cpuchip/ai-chattermax/internal/dnd"
	"github.com/cpuchip/ai-chattermax/internal/gateway"
	"github.com/cpuchip/ai-chattermax/internal/httpapi"
	"github.com/cpuchip/ai-chattermax/internal/seed"
	"github.com/cpuchip/ai-chattermax/internal/store"
)

// buildTag is surfaced at GET /api/version so a deploy's freshness is verifiable
// (Dokploy has silently served stale binaries). Bump it with backend changes.
const buildTag = "persona-profile-sync-2026-06-11"

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("chattermax: ")
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool, static.Migrations); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Printf("schema ready")

	st := store.New(pool)

	demo, err := seed.EnsureDemo(ctx, st, cfg.DevMode())
	if err != nil {
		log.Fatalf("seed: %v", err)
	}
	log.Printf("demo server ready (id=%s)", demo.ServerID)
	if cfg.DevMode() {
		for hostRef, key := range demo.PersonaKeys {
			log.Printf("dev persona key  %-14s = %s", hostRef, key)
		}
	}

	authService := auth.NewService(st, auth.NewAuthenticator(cfg.AuthMode, cfg.IbecoBaseURL), cfg.CookieDomain, cfg.CookieSecure)
	// A brand-new user (member of no servers) gets their own server so they land
	// somewhere they own. Collaboration is via invite links, not auto-join.
	authService.OnLogin = func(ctx context.Context, user store.User) {
		servers, err := st.ListServersForUser(ctx, user.ID)
		if err != nil {
			return
		}
		for _, sv := range servers {
			if sv.OwnerUserID == user.ID {
				return // already owns a server
			}
		}
		sv, err := st.CreateServer(ctx, personalSlug(user.DisplayName), user.DisplayName+"'s Server", user.ID)
		if err != nil {
			log.Printf("onboard %s: create server: %v", user.ID, err)
			return
		}
		if _, err := st.CreateRoom(ctx, sv.ID, "general", "general", "public", "Welcome aboard.", user.ID); err != nil {
			log.Printf("onboard %s: create room: %v", user.ID, err)
		}
	}

	dndClient := dnd.New(cfg.DNDURL, cfg.DNDAPIKey)
	if dndClient.Enabled() {
		log.Printf("dnd-tools integration enabled: %s", cfg.DNDURL)
	}
	api := httpapi.New(st, cfg.AuthMode, dndClient)
	hub := gateway.NewHub()
	gw := gateway.NewHandler(hub, st, authService.UserFromRequest, dndClient)

	// Protected API routes (the user is required + placed in context).
	protected := http.NewServeMux()
	api.Register(protected)
	protected.HandleFunc("GET /api/me", authService.MeHandler)
	protectedHandler := authService.Required(protected)

	staticFS, err := static.FS()
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}

	root := http.NewServeMux()
	root.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	root.HandleFunc("GET /api/config", api.ConfigHandler)
	root.HandleFunc("GET /api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"build":"` + buildTag + `"}`))
	})
	root.HandleFunc("GET /api/persona/rooms", api.PersonaRoomsHandler)      // persona-key auth
	root.HandleFunc("GET /api/persona/dms", api.PersonaDMsHandler)          // persona-key auth
	root.HandleFunc("PATCH /api/persona/profile", api.PersonaProfileHandler) // persona-key auth
	root.HandleFunc("POST /api/auth/login", authService.LoginHandler)
	root.HandleFunc("POST /api/auth/logout", authService.LogoutHandler)
	root.Handle("GET /gateway", gw)
	// Everything else under /api/ requires auth.
	root.Handle("/api/", protectedHandler)
	// SPA + static assets.
	root.Handle("/", spaHandler(staticFS))

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: root}
	go func() {
		log.Printf("listening on :%s (auth=%s)", cfg.Port, cfg.AuthMode)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Printf("stopped")
}

// personalSlug builds a unique, URL-safe server slug from a display name.
func personalSlug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	base := strings.Trim(b.String(), "-")
	if base == "" {
		base = "crew"
	}
	return fmt.Sprintf("%s-%x", base, time.Now().UnixNano()%0xffffff)
}

// spaHandler serves embedded static files, falling back to index.html for SPA
// client-side routes.
func spaHandler(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := strings.TrimPrefix(r.URL.Path, "/")
		if clean == "" {
			clean = "index.html"
		}
		if f, err := fsys.Open(clean); err == nil {
			defer f.Close()
			if stat, err := f.Stat(); err == nil && !stat.IsDir() {
				http.ServeContent(w, r, clean, stat.ModTime(), f.(io.ReadSeeker))
				return
			}
		}
		f, err := fsys.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		stat, _ := f.Stat()
		http.ServeContent(w, r, "index.html", stat.ModTime(), f.(io.ReadSeeker))
	})
}
