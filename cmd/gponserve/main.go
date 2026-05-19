// gponserve: a minimal static file server for the GPON dashboard container.
//
// Why this exists: the lipanski/docker-static-website busybox httpd in the
// RouterOS container is compiled without IPv6 ("httpd: bad address
// '[::]:3000'"), so it cannot be exposed over the Route64 v6 WAN. This is a
// ~single-file, stdlib-only, GET/HEAD-only replacement that binds dual-stack
// and is delivered as a statically-linked linux/arm64 binary mounted
// read-only into the scratch image and run as the container entrypoint.
//
//	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath \
//	  -ldflags "-s -w" -o gponserve .
//
// Listening on ":3000" gives a dual-stack socket on Linux (net.ipv6.
// bindv6only defaults to 0), so it can serve both IPv4 and IPv6 container
// addresses when RouterOS assigns them.
package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func main() {
	addr := envOr("GPON_ADDR", ":3000")
	staticRoot := envOr("GPON_STATIC_ROOT", envOr("GPON_ROOT", "/opt/gpontelemetry/www"))
	logRoot := envOr("GPON_LOG_ROOT", envOr("GPON_ROOT", "/var/lib/gpontelemetry"))

	staticAbs, err := filepath.Abs(staticRoot)
	if err != nil {
		log.Fatalf("gponserve: bad static root %q: %v", staticRoot, err)
	}
	logAbs, err := filepath.Abs(logRoot)
	if err != nil {
		log.Fatalf("gponserve: bad log root %q: %v", logRoot, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// GET/HEAD only — this is a read-only dashboard.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Map "/" (or any dir path) to index.html. Files are served by
		// explicit open + copy — never http.FileServer/ServeFile, which
		// canonical-redirect "/index.html" -> "/" and loop.
		clean := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
		if clean == "/" || strings.HasSuffix(clean, "/") {
			clean = path.Join(clean, "index.html")
		}
		// Whitelist: HTML comes from the image; logs come from the mounted
		// writable data directory. Refuse anything else.
		var ctype, base string
		switch strings.ToLower(path.Ext(clean)) {
		case ".html":
			ctype = "text/html; charset=utf-8"
			base = staticAbs
		case ".txt":
			ctype = "text/plain; charset=utf-8"
			base = logAbs
		default:
			http.NotFound(w, r)
			return
		}
		// path.Clean above already removed any ".." traversal.
		full := filepath.Join(base, filepath.FromSlash(clean))
		f, err := os.Open(full)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		if info, err := f.Stat(); err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		// The log changes every poll; never let anything cache it.
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", ctype)
		if r.Method == http.MethodHead {
			return
		}
		io.Copy(w, f)
	})

	srv := &http.Server{Addr: addr, Handler: mux}
	log.Printf("gponserve: serving static=%s logs=%s on %s (dual-stack)", staticAbs, logAbs, addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("gponserve: %v", err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
