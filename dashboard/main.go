package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

//go:embed static
var staticFS embed.FS

// The dashboard holds no secrets of its own: it's a static file server plus
// a transparent reverse proxy to the router under /api/, forwarding
// whatever Authorization header the browser sends. The router (and the
// agents behind it) are the actual source of truth for the management
// token — see docs/CAPABILITY_SCHEMA.md.
func main() {
	addr := flag.String("addr", ":8092", "address to serve the dashboard on")
	routerAddr := flag.String("router", "http://localhost:8081", "router base URL")
	flag.Parse()

	target, err := url.Parse(*routerAddr)
	if err != nil {
		log.Fatalf("edgeos-dashboard: invalid -router URL: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", httputil.NewSingleHostReverseProxy(target)))

	staticContent, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticContent)))

	log.Printf("edgeos-dashboard: serving on %s, proxying to router at %s", *addr, *routerAddr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
