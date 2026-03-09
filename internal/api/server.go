package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
)

// SnapshotProvider 提供只读运行时快照访问。
type SnapshotProvider interface {
	StatusPayload() any
	SourcesPayload() any
	NodesPayload() any
	CandidatesPayload() any
	HealthPayload() any
}

// Server 提供最小只读管理 API 与嵌入式前端静态页面。
type Server struct {
	provider      SnapshotProvider
	authHeader    string
	token         string
	staticHandler http.Handler
}

func NewServer(provider SnapshotProvider, authHeader, token string) *Server {
	return &Server{
		provider:      provider,
		authHeader:    strings.TrimSpace(authHeader),
		token:         strings.TrimSpace(token),
		staticHandler: newStaticHandler(),
	}
}

func (s *Server) Handler() http.Handler {
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/v1/status", s.handleStatus)
	apiMux.HandleFunc("/api/v1/sources", s.handleSources)
	apiMux.HandleFunc("/api/v1/nodes", s.handleNodes)
	apiMux.HandleFunc("/api/v1/candidates", s.handleCandidates)
	apiMux.HandleFunc("/api/v1/health", s.handleHealth)
	apiMux.HandleFunc("/api/v1/", s.handleAPINotFound)

	apiHandler := http.Handler(apiMux)
	if s.token != "" {
		apiHandler = s.authMiddleware(apiHandler)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/v1/", apiHandler)
	mux.Handle("/", s.staticHandler)
	return mux
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, s.provider.StatusPayload())
}

func (s *Server) handleSources(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, s.provider.SourcesPayload())
}

func (s *Server) handleNodes(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, s.provider.NodesPayload())
}

func (s *Server) handleCandidates(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, s.provider.CandidatesPayload())
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, s.provider.HealthPayload())
}

func (s *Server) handleAPINotFound(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get(s.authHeader)), []byte(s.token)) != 1 {
			s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
