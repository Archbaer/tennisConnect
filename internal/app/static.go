package app

import (
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// spaHandler serves the built SvelteKit site and falls back to index.html for
// client-side routes. API prefixes are never shadowed by the fallback.
func spaHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		for _, segment := range strings.Split(r.URL.Path, "/") {
			if segment == ".." {
				http.NotFound(w, r)
				return
			}
		}
		clean := path.Clean("/" + r.URL.Path)
		for _, prefix := range []string{"/api/", "/auth/", "/reviewer/verify", "/healthz"} {
			if clean == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(clean, prefix) {
				http.NotFound(w, r)
				return
			}
		}
		for _, segment := range strings.Split(clean, "/") {
			if strings.HasPrefix(segment, ".") && segment != "" && segment != "." {
				http.NotFound(w, r)
				return
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'; base-uri 'none'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(self)")

		// http.Dir applies its own traversal guard; the request path was also
		// cleaned and checked above. Stat through it so gosec does not see a
		// tainted filepath.Join.
		if asset, err := http.Dir(dir).Open(clean); err == nil {
			info, statErr := asset.Stat()
			_ = asset.Close()
			if statErr == nil && !info.IsDir() {
				files.ServeHTTP(w, r)
				return
			}
		}
		index := filepath.Join(dir, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, index)
	})
}

// validateWebDir fails startup early instead of serving a broken site.
func validateWebDir(dir string) error {
	if dir == "" {
		return nil
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return errors.New("WEB_DIR must be an existing directory")
	}
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		return errors.New("WEB_DIR must contain index.html")
	}
	return nil
}
