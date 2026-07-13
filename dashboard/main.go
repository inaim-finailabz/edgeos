package main

import (
	"bytes"
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

//go:embed static
var staticFS embed.FS

// The dashboard holds no secrets of its own: it's a static file server plus
// a transparent reverse proxy to the router under /api/, forwarding
// whatever Authorization header the browser sends. The router (and the
// agents behind it) are the actual source of truth for the management
// token — see docs/CAPABILITY_SCHEMA.md.
//
// -public additionally makes this safe to expose on the open internet: the
// proxy refuses anything but GET (so no captured/guessed token could ever
// reach a write route through this instance, regardless of what the router
// itself would accept), and the served page hides the token field and
// action buttons entirely.
func main() {
	defaultRouter := os.Getenv("EDGEOS_ROUTER_ADDR")
	if defaultRouter == "" {
		defaultRouter = "http://localhost:8081"
	}
	addr := flag.String("addr", ":8092", "address to serve the dashboard on")
	routerAddr := flag.String("router", defaultRouter, "router base URL (default: $EDGEOS_ROUTER_ADDR)")
	public := flag.Bool("public", false, "read-only mode for public/internet exposure: blocks all non-GET proxy requests and hides management UI")
	flag.Parse()

	target, err := url.Parse(*routerAddr)
	if err != nil {
		log.Fatalf("edgeos-dashboard: invalid -router URL: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	var apiHandler http.Handler = proxy
	if *public {
		apiHandler = readOnlyOnly(proxy)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", apiHandler))

	staticContent, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal(err)
	}
	mux.HandleFunc("GET /{$}", indexHandler(staticContent, *public))
	mux.Handle("/", http.FileServer(http.FS(staticContent)))

	if *public {
		log.Printf("edgeos-dashboard: PUBLIC (read-only) mode — no write route can reach the router through this instance")
	}
	log.Printf("edgeos-dashboard: serving on %s, proxying to router at %s", *addr, *routerAddr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

// readOnlyOnly rejects every method but GET before it ever reaches the
// proxy, so this is enforced here regardless of router-side auth.
func readOnlyOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "read-only public instance: only GET is allowed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// indexHandler serves static/index.html with a small injected flag so the
// page's own JS can hide the token field and action buttons in public mode.
func indexHandler(staticContent fs.FS, public bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := fs.ReadFile(staticContent, "index.html")
		if err != nil {
			http.Error(w, "index not found", http.StatusInternalServerError)
			return
		}
		flag := "<script>window.EDGEOS_PUBLIC = false;</script>"
		if public {
			flag = "<script>window.EDGEOS_PUBLIC = true;</script>"
		}
		page = bytes.Replace(page, []byte("</head>"), []byte(flag+"</head>"), 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(page)
	}
}
