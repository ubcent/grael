package server

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"grael/demo/adapter"
	"grael/internal/api"
)

//go:embed web/*
var webFS embed.FS

type Server struct {
	svc     *api.Service
	adapter *adapter.Adapter
	mux     *http.ServeMux
}

func New(dataDir string) *Server {
	svc := api.New(dataDir)
	s := &Server{
		svc:     svc,
		adapter: adapter.New(svc),
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) Close() {
	s.svc.Close()
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	webRoot, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(webRoot))
	s.mux.Handle("GET /", files)
	s.mux.Handle("GET /demo/", http.StripPrefix("/demo/", files))
	s.mux.HandleFunc("GET /api/runs/{runID}/snapshot", s.handleSnapshot)
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("runID"))
	if runID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing run id"})
		return
	}

	afterSeq, err := parseAfterSeq(r.URL.Query().Get("after_seq"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid after_seq"})
		return
	}

	snapshot, err := s.adapter.Snapshot(runID, afterSeq)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func parseAfterSeq(value string) (uint64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	return strconv.ParseUint(value, 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
