package web

import (
	"embed"
	"net/http"
)

//go:embed frontend/index.html frontend/app.css frontend/app.js
var frontend embed.FS

func serveAsset(w http.ResponseWriter, r *http.Request, name, contentType string) {
	data, err := frontend.ReadFile(name)
	if err != nil {
		http.Error(w, "资源不存在", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) WorkbenchHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	serveAsset(w, r, "frontend/index.html", "text/html; charset=utf-8")
}

func (s *Server) CSSHandler(w http.ResponseWriter, r *http.Request) {
	serveAsset(w, r, "frontend/app.css", "text/css; charset=utf-8")
}

func (s *Server) JavaScriptHandler(w http.ResponseWriter, r *http.Request) {
	serveAsset(w, r, "frontend/app.js", "text/javascript; charset=utf-8")
}
