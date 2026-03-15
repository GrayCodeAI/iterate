package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"

	"github.com/yourusername/iterate/internal/agent"
	"github.com/yourusername/iterate/internal/session"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Server is the web dashboard.
type Server struct {
	store    *session.Store
	logger   *slog.Logger
	eventBus chan agent.Event // receives live events from the running agent
}

// New creates a new web server.
func New(store *session.Store, logger *slog.Logger) *Server {
	return &Server{
		store:    store,
		logger:   logger,
		eventBus: make(chan agent.Event, 128),
	}
}

// EventBus returns the channel to push live agent events.
func (s *Server) EventBus() chan<- agent.Event {
	return s.eventBus
}

// Handler builds and returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", s.handleDashboard)
	r.Get("/api/sessions", s.handleSessions)
	r.Get("/api/stats", s.handleStats)
	r.Get("/ws/live", s.handleWebSocket)

	return r
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(dashboardHTML))
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.store.List(50)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonResponse(w, sessions)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.Stats()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	dayCount := "?"
	if data, err := os.ReadFile("DAY_COUNT"); err == nil {
		dayCount = string(data)
	}

	jsonResponse(w, map[string]any{
		"by_status": stats,
		"day_count": dayCount,
	})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("ws upgrade failed", "err", err)
		return
	}
	defer conn.Close()

	s.logger.Info("ws client connected")

	for event := range s.eventBus {
		msg, _ := json.Marshal(event)
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			s.logger.Info("ws client disconnected")
			return
		}
	}
}

func jsonResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// dashboardHTML is the single-file web UI.
const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>iterate — self-evolving agent</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: system-ui, sans-serif; background: #0f0f0f; color: #e0e0e0; }
  header { padding: 1.5rem 2rem; border-bottom: 1px solid #222; display: flex; align-items: center; gap: 1rem; }
  header h1 { font-size: 1.25rem; font-weight: 600; color: #fff; }
  header .badge { background: #1a1a2e; color: #7c7cf0; padding: 0.25rem 0.75rem; border-radius: 999px; font-size: 0.75rem; border: 1px solid #7c7cf033; }
  .grid { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 1rem; padding: 1.5rem 2rem; }
  .card { background: #161616; border: 1px solid #222; border-radius: 12px; padding: 1.25rem; }
  .card h2 { font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.08em; color: #666; margin-bottom: 0.5rem; }
  .card .value { font-size: 2rem; font-weight: 700; color: #fff; }
  .card .sub { font-size: 0.8rem; color: #555; margin-top: 0.25rem; }
  .panel { margin: 0 2rem 2rem; background: #161616; border: 1px solid #222; border-radius: 12px; }
  .panel-header { padding: 1rem 1.25rem; border-bottom: 1px solid #222; font-size: 0.85rem; font-weight: 600; color: #999; display: flex; align-items: center; gap: 0.5rem; }
  .dot { width: 8px; height: 8px; border-radius: 50%; background: #666; }
  .dot.live { background: #22c55e; animation: pulse 2s infinite; }
  @keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.4} }
  #live-log { padding: 1rem 1.25rem; font-family: monospace; font-size: 0.8rem; line-height: 1.7; max-height: 400px; overflow-y: auto; }
  .event { padding: 0.2rem 0; border-bottom: 1px solid #1a1a1a; }
  .event .type { display: inline-block; width: 100px; color: #555; }
  .event.thought .type { color: #7c7cf0; }
  .event.tool_call .type { color: #f0a84b; }
  .event.tool_result .type { color: #4bade0; }
  .event.done .type { color: #22c55e; }
  .event.error .type { color: #ef4444; }
  table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
  th { text-align: left; padding: 0.75rem 1.25rem; color: #555; font-weight: 500; border-bottom: 1px solid #222; font-size: 0.75rem; text-transform: uppercase; }
  td { padding: 0.75rem 1.25rem; border-bottom: 1px solid #1a1a1a; color: #ccc; }
  .status { display: inline-block; padding: 0.15rem 0.6rem; border-radius: 999px; font-size: 0.7rem; font-weight: 600; }
  .status.committed { background: #14532d; color: #4ade80; }
  .status.reverted  { background: #7f1d1d; color: #f87171; }
  .status.running   { background: #1e1b4b; color: #a5b4fc; }
  .status.error     { background: #7f1d1d; color: #f87171; }
</style>
</head>
<body>
<header>
  <h1>iterate</h1>
  <span class="badge" id="provider">loading...</span>
  <span class="badge" id="day-badge">day ?</span>
</header>

<div class="grid">
  <div class="card">
    <h2>Total sessions</h2>
    <div class="value" id="stat-total">—</div>
    <div class="sub">all time</div>
  </div>
  <div class="card">
    <h2>Committed</h2>
    <div class="value" id="stat-committed">—</div>
    <div class="sub">successful improvements</div>
  </div>
  <div class="card">
    <h2>Reverted</h2>
    <div class="value" id="stat-reverted">—</div>
    <div class="sub">tests failed</div>
  </div>
</div>

<div class="panel">
  <div class="panel-header">
    <div class="dot live" id="live-dot"></div>
    Live agent stream
  </div>
  <div id="live-log"><span style="color:#444">Waiting for agent activity...</span></div>
</div>

<div class="panel">
  <div class="panel-header">Session history</div>
  <table>
    <thead><tr><th>Date</th><th>Status</th><th>Provider</th><th>Duration</th></tr></thead>
    <tbody id="sessions-table"></tbody>
  </table>
</div>

<script>
async function loadStats() {
  const r = await fetch('/api/stats');
  const d = await r.json();
  const s = d.by_status || {};
  const total = Object.values(s).reduce((a,b)=>a+b,0);
  document.getElementById('stat-total').textContent = total;
  document.getElementById('stat-committed').textContent = s.committed || 0;
  document.getElementById('stat-reverted').textContent = s.reverted || 0;
  document.getElementById('day-badge').textContent = 'day ' + (d.day_count || '?').trim();
}

async function loadSessions() {
  const r = await fetch('/api/sessions');
  const sessions = await r.json();
  const tbody = document.getElementById('sessions-table');
  tbody.innerHTML = (sessions || []).map(s => {
    const start = new Date(s.StartedAt);
    const end = new Date(s.FinishedAt);
    const dur = s.FinishedAt ? Math.round((end-start)/1000)+'s' : '—';
    return '<tr>' +
      '<td>' + start.toLocaleString() + '</td>' +
      '<td><span class="status ' + s.Status + '">' + s.Status + '</span></td>' +
      '<td>' + s.Provider + '</td>' +
      '<td>' + dur + '</td>' +
    '</tr>';
  }).join('');
}

function connectWS() {
  const ws = new WebSocket('ws://' + location.host + '/ws/live');
  const log = document.getElementById('live-log');
  const dot = document.getElementById('live-dot');
  
  ws.onmessage = (e) => {
    const event = JSON.parse(e.data);
    const div = document.createElement('div');
    div.className = 'event ' + event.Type;
    const content = event.Content.length > 200 ? event.Content.slice(0,200)+'...' : event.Content;
    div.innerHTML = '<span class="type">' + event.Type + '</span> ' + escapeHtml(content);
    if (log.children[0]?.textContent?.includes('Waiting')) log.innerHTML = '';
    log.appendChild(div);
    log.scrollTop = log.scrollHeight;
    if (event.Type === 'done' || event.Type === 'error') {
      loadStats(); loadSessions();
    }
  };
  
  ws.onclose = () => {
    dot.classList.remove('live');
    setTimeout(connectWS, 3000);
  };
  ws.onopen = () => dot.classList.add('live');
}

function escapeHtml(s) {
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

loadStats(); loadSessions(); connectWS();
setInterval(() => { loadStats(); loadSessions(); }, 30000);
</script>
</body>
</html>`
