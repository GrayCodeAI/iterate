package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"

	"github.com/GrayCodeAI/iterate/internal/agent"
	"github.com/GrayCodeAI/iterate/internal/session"
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
  :root {
    --bg: #0a0a0b;
    --bg-elevated: #111113;
    --bg-subtle: #18181b;
    --border: #27272a;
    --border-subtle: #1f1f23;
    --text: #fafafa;
    --text-secondary: #a1a1aa;
    --text-muted: #71717a;
    --accent: #22c55e;
    --accent-dim: #166534;
    --danger: #ef4444;
    --warning: #f59e0b;
    --info: #3b82f6;
    --purple: #8b5cf6;
    --font-sans: 'SF Pro Display', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    --font-mono: 'SF Mono', 'Fira Code', Monaco, monospace;
  }
  body { 
    font-family: var(--font-sans); 
    background: var(--bg); 
    color: var(--text); 
    line-height: 1.5;
    -webkit-font-smoothing: antialiased;
  }
  header { 
    padding: 1.5rem 2.5rem; 
    border-bottom: 1px solid var(--border); 
    display: flex; 
    align-items: center; 
    gap: 1.25rem; 
    background: var(--bg);
    position: sticky;
    top: 0;
    z-index: 100;
  }
  .logo { 
    font-size: 1.5rem; 
    font-weight: 700; 
    letter-spacing: -0.03em;
    color: var(--text);
    font-family: var(--font-mono);
  }
  .logo span { color: var(--accent); }
  .badge { 
    background: var(--bg-subtle); 
    color: var(--text-secondary); 
    padding: 0.35rem 0.85rem; 
    border-radius: 6px; 
    font-size: 0.75rem; 
    font-weight: 500;
    border: 1px solid var(--border);
    font-family: var(--font-mono);
  }
  .container { max-width: 1400px; margin: 0 auto; padding: 2rem 2.5rem; }
  .stats-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 1px; background: var(--border); border: 1px solid var(--border); border-radius: 12px; overflow: hidden; margin-bottom: 1.5rem; }
  .stat-card { background: var(--bg-elevated); padding: 1.75rem 2rem; }
  .stat-card h3 { font-size: 0.7rem; text-transform: uppercase; letter-spacing: 0.1em; color: var(--text-muted); font-weight: 600; margin-bottom: 0.75rem; }
  .stat-card .value { font-size: 2.5rem; font-weight: 700; color: var(--text); letter-spacing: -0.02em; }
  .stat-card .value.accent { color: var(--accent); }
  .stat-card .sub { font-size: 0.8rem; color: var(--text-muted); margin-top: 0.35rem; }
  
  .panel { 
    background: var(--bg-elevated); 
    border: 1px solid var(--border); 
    border-radius: 12px; 
    overflow: hidden;
    margin-bottom: 1.5rem;
  }
  .panel-header { 
    padding: 1rem 1.5rem; 
    border-bottom: 1px solid var(--border-subtle); 
    font-size: 0.8rem; 
    font-weight: 600; 
    color: var(--text-secondary); 
    display: flex; 
    align-items: center; 
    gap: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .status-dot { width: 8px; height: 8px; border-radius: 50%; background: var(--text-muted); }
  .status-dot.live { background: var(--accent); box-shadow: 0 0 8px var(--accent); }
  
  #live-log { 
    padding: 1rem 1.5rem; 
    font-family: var(--font-mono); 
    font-size: 0.75rem; 
    line-height: 1.8; 
    max-height: 380px; 
    overflow-y: auto; 
    background: var(--bg);
  }
  #live-log::-webkit-scrollbar { width: 6px; }
  #live-log::-webkit-scrollbar-track { background: var(--bg); }
  #live-log::-webkit-scrollbar-thumb { background: var(--border); border-radius: 3px; }
  
  .event { padding: 0.25rem 0; border-bottom: 1px solid var(--border-subtle); display: flex; gap: 1rem; }
  .event:last-child { border-bottom: none; }
  .event .type { 
    color: var(--text-muted); 
    min-width: 110px; 
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .event.thought .type { color: var(--purple); }
  .event.tool_call .type { color: var(--warning); }
  .event.tool_result .type { color: var(--info); }
  .event.done .type { color: var(--accent); }
  .event.error .type { color: var(--danger); }
  .event .content { color: var(--text-secondary); flex: 1; word-break: break-word; }
  
  table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
  th { text-align: left; padding: 1rem 1.5rem; color: var(--text-muted); font-weight: 600; border-bottom: 1px solid var(--border-subtle); font-size: 0.7rem; text-transform: uppercase; letter-spacing: 0.05em; }
  td { padding: 1rem 1.5rem; border-bottom: 1px solid var(--border-subtle); color: var(--text-secondary); }
  tr:last-child td { border-bottom: none; }
  tr:hover td { background: var(--bg-subtle); }
  
  .status-tag { 
    display: inline-block; 
    padding: 0.25rem 0.65rem; 
    border-radius: 4px; 
    font-size: 0.7rem; 
    font-weight: 600; 
    text-transform: uppercase;
    letter-spacing: 0.03em;
  }
  .status-tag.committed { background: var(--accent-dim); color: var(--accent); }
  .status-tag.reverted  { background: #450a0a; color: #f87171; }
  .status-tag.running   { background: #1e1b4b; color: #a5b4fc; }
  .status-tag.error     { background: #450a0a; color: #f87171; }
  .status-tag.commit_failed { background: #451a03; color: #fb923c; }
  
  .mono { font-family: var(--font-mono); }
  .text-muted { color: var(--text-muted); }
  
  .empty-state { 
    padding: 3rem; 
    text-align: center; 
    color: var(--text-muted);
    font-size: 0.9rem;
  }
</style>
</head>
<body>
<header>
  <div class="logo">iterate<span>_</span></div>
  <span class="badge" id="provider">loading...</span>
  <span class="badge" id="day-badge">day ?</span>
</header>

<div class="container">
  <div class="stats-grid">
    <div class="stat-card">
      <h3>Total Sessions</h3>
      <div class="value" id="stat-total">—</div>
      <div class="sub">all time</div>
    </div>
    <div class="stat-card">
      <h3>Successful</h3>
      <div class="value accent" id="stat-committed">—</div>
      <div class="sub">improvements committed</div>
    </div>
    <div class="stat-card">
      <h3>Reverted</h3>
      <div class="value" id="stat-reverted">—</div>
      <div class="sub">tests failed</div>
    </div>
  </div>

  <div class="panel">
    <div class="panel-header">
      <div class="status-dot live" id="live-dot"></div>
      Agent Activity
    </div>
    <div id="live-log"><span class="text-muted">Waiting for agent activity...</span></div>
  </div>

  <div class="panel">
    <div class="panel-header">Session History</div>
    <table>
      <thead><tr><th>Date</th><th>Status</th><th>Provider</th><th>Duration</th><th>Output</th></tr></thead>
      <tbody id="sessions-table">
        <tr><td colspan="5" class="empty-state">No sessions yet</td></tr>
      </tbody>
    </table>
  </div>
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
  if (!sessions || sessions.length === 0) {
    tbody.innerHTML = '<tr><td colspan="5" class="empty-state">No sessions yet</td></tr>';
    return;
  }
  tbody.innerHTML = sessions.map(s => {
    const start = new Date(s.StartedAt);
    const end = new Date(s.FinishedAt);
    const dur = s.FinishedAt ? Math.round((end-start)/1000)+'s' : '—';
    const output = s.RawOutput ? (s.RawOutput.slice(0, 60) + (s.RawOutput.length > 60 ? '...' : '')) : '—';
    return '<tr>' +
      '<td class="mono">' + start.toLocaleDateString() + ' <span class="text-muted">' + start.toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'}) + '</span></td>' +
      '<td><span class="status-tag ' + s.Status + '">' + s.Status + '</span></td>' +
      '<td class="mono">' + (s.Provider || '—') + '</td>' +
      '<td>' + dur + '</td>' +
      '<td class="text-muted" style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">' + escapeHtml(output) + '</td>' +
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
    let content = event.Content || '';
    if (typeof content === 'object') content = JSON.stringify(content);
    content = content.length > 300 ? content.slice(0,300)+'...' : content;
    div.innerHTML = '<span class="type">' + event.Type + '</span><span class="content">' + escapeHtml(content) + '</span>';
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
  if (!s) return '';
  return s.toString().replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

loadStats(); loadSessions(); connectWS();
setInterval(() => { loadStats(); loadSessions(); }, 30000);
</script>
</body>
</html>`
