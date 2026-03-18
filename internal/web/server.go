// Package web provides the Argus HTTP dashboard.
package web

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/cortexark/argus/internal/db"
)

// Server is the embedded HTTP server.
type Server struct {
	store  *db.Store
	port   int
	server *http.Server
}

// New creates a web server.
func New(store *db.Store, port int) *Server {
	return &Server{store: store, port: port}
}

// Start starts the HTTP server on localhost only.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/summary", s.handleSummary)
	mux.HandleFunc("/api/alerts", s.handleAlerts)
	mux.HandleFunc("/api/network", s.handleNetwork)

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("Web dashboard unavailable (port %d in use)\n", s.port)
		return nil
	}

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		_ = s.server.Serve(ln)
	}()

	fmt.Printf("Dashboard: http://%s\n", addr)
	return nil
}

// Stop shuts down the HTTP server.
func (s *Server) Stop() {
	if s.server != nil {
		_ = s.server.Close()
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	fmt.Fprint(w, dashboardHTML)
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := s.store.GetDailySummary(time.Now())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, summary)
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := s.store.GetRecentFileAlerts(time.Time{}, 50)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, alerts)
}

func (s *Server) handleNetwork(w http.ResponseWriter, r *http.Request) {
	events, err := s.store.GetRecentNetworkEvents(50)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, events)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "http://127.0.0.1")
	_ = json.NewEncoder(w).Encode(v)
}

// dashboardHTML is the embedded single-page dashboard.
// All dynamic content is rendered via textContent (not innerHTML) to prevent XSS.
const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Argus — AI Privacy Monitor</title>
<meta http-equiv="Content-Security-Policy" content="default-src 'self'; script-src 'nonce-argus2025'; style-src 'unsafe-inline'">
<style>
  *{box-sizing:border-box;margin:0;padding:0}
  body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",monospace;background:#0d1117;color:#c9d1d9;min-height:100vh}
  header{background:#161b22;border-bottom:1px solid #30363d;padding:16px 24px;display:flex;align-items:center;gap:12px}
  header h1{font-size:20px;color:#58a6ff}
  .subtitle{color:#8b949e;font-size:13px}
  .badge{background:#238636;color:#fff;font-size:11px;padding:2px 8px;border-radius:12px}
  main{padding:24px;max-width:1200px;margin:0 auto}
  .grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(280px,1fr));gap:16px;margin-bottom:24px}
  .card{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:20px}
  .card h2{font-size:13px;color:#8b949e;text-transform:uppercase;letter-spacing:.5px;margin-bottom:16px}
  .stat{font-size:36px;font-weight:bold;color:#58a6ff}
  .stat-label{font-size:12px;color:#8b949e;margin-top:4px}
  table{width:100%;border-collapse:collapse;font-size:13px}
  th{text-align:left;color:#8b949e;padding:8px 0;border-bottom:1px solid #30363d;font-weight:normal}
  td{padding:10px 0;border-bottom:1px solid #21262d;max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .cred{color:#f85149;font-weight:bold}
  .browser{color:#d29922}
  .doc{color:#58a6ff}
  .refresh{background:none;border:1px solid #30363d;color:#8b949e;padding:6px 12px;border-radius:6px;cursor:pointer;font-size:12px;margin-left:auto}
  .refresh:hover{border-color:#58a6ff;color:#58a6ff}
  footer{text-align:center;color:#484f58;font-size:12px;padding:24px}
  .empty{color:#484f58}
</style>
</head>
<body>
<header>
  <h1>&#128065; Argus</h1>
  <div class="subtitle">AI Privacy Monitor</div>
  <span class="badge">LIVE</span>
  <button class="refresh" onclick="loadAll()">&#8635; Refresh</button>
</header>
<main>
  <div class="grid">
    <div class="card"><h2>AI Processes Today</h2><div class="stat" id="proc-count">&#8212;</div><div class="stat-label">active AI apps detected</div></div>
    <div class="card"><h2>File Alerts Today</h2><div class="stat" id="file-count">&#8212;</div><div class="stat-label">sensitive file accesses</div></div>
    <div class="card"><h2>Network Events</h2><div class="stat" id="net-count">&#8212;</div><div class="stat-label">connections to AI endpoints</div></div>
  </div>
  <div class="card" style="margin-bottom:16px">
    <h2>Recent File Alerts</h2>
    <table><thead><tr><th>Time</th><th>Process</th><th>File</th><th>Category</th></tr></thead>
    <tbody id="alerts-body"></tbody></table>
  </div>
  <div class="card">
    <h2>Network Activity</h2>
    <table><thead><tr><th>Time</th><th>Process</th><th>Endpoint</th><th>Port</th></tr></thead>
    <tbody id="network-body"></tbody></table>
  </div>
</main>
<footer>Argus &mdash; Watching your AI so you don&apos;t have to</footer>

<script nonce="argus2025">
// All data rendering uses textContent to prevent XSS
function esc(s) { return String(s == null ? '' : s); }

function makeRow(cells, classMap) {
  const tr = document.createElement('tr');
  cells.forEach(({text, cls}) => {
    const td = document.createElement('td');
    td.textContent = esc(text);
    if (cls) td.className = cls;
    tr.appendChild(td);
  });
  return tr;
}

function setEmpty(tbodyId, cols, msg) {
  const tbody = document.getElementById(tbodyId);
  tbody.innerHTML = '';
  const tr = document.createElement('tr');
  const td = document.createElement('td');
  td.setAttribute('colspan', cols);
  td.textContent = msg;
  td.className = 'empty';
  tr.appendChild(td);
  tbody.appendChild(tr);
}

async function loadAll() {
  await Promise.all([loadAlerts(), loadNetwork(), loadSummary()]);
}

async function loadSummary() {
  try {
    const d = await fetch('/api/summary').then(r => r.json());
    document.getElementById('proc-count').textContent = (d.processes||[]).length;
    document.getElementById('file-count').textContent = (d.fileAlerts||[]).length;
    document.getElementById('net-count').textContent = (d.networkEvents||[]).length;
  } catch(e) { console.error('summary', e); }
}

async function loadAlerts() {
  const tbody = document.getElementById('alerts-body');
  try {
    const items = await fetch('/api/alerts').then(r => r.json());
    tbody.innerHTML = '';
    if (!items || !items.length) { setEmpty('alerts-body', 4, 'No alerts'); return; }
    items.slice(0,20).forEach(a => {
      const ts = new Date(a.timestamp).toLocaleTimeString();
      const cls = a.sensitivity === 'credentials' ? 'cred' : a.sensitivity === 'browserData' ? 'browser' : 'doc';
      tbody.appendChild(makeRow([
        {text: ts}, {text: a.processName}, {text: a.filePath}, {text: a.sensitivity, cls}
      ]));
    });
  } catch(e) { setEmpty('alerts-body', 4, 'Error loading alerts'); console.error(e); }
}

async function loadNetwork() {
  try {
    const items = await fetch('/api/network').then(r => r.json());
    const tbody = document.getElementById('network-body');
    tbody.innerHTML = '';
    if (!items || !items.length) { setEmpty('network-body', 4, 'No events'); return; }
    items.slice(0,20).forEach(n => {
      const ts = new Date(n.timestamp).toLocaleTimeString();
      tbody.appendChild(makeRow([
        {text: ts}, {text: n.processName}, {text: n.label || n.remoteAddr}, {text: n.remotePort}
      ]));
    });
  } catch(e) { setEmpty('network-body', 4, 'Error loading events'); console.error(e); }
}

loadAll();
setInterval(loadAll, 10000);
</script>
</body>
</html>`
