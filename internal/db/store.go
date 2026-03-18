package db

import (
	"time"
)

// ─── Domain types ────────────────────────────────────────────────────────────

// ProcessSnapshot captures one observation of an AI process.
type ProcessSnapshot struct {
	PID        int
	Name       string
	AppName    string
	Cmdline    string
	User       string
	CPUPercent float64
	MemMB      float64
	Confidence string
	Score      int
	Timestamp  time.Time
}

// FileAccessEvent records that an AI process opened a sensitive file.
type FileAccessEvent struct {
	PID         int
	ProcessName string
	FilePath    string
	Sensitivity string
	Timestamp   time.Time
}

// NetworkEvent records a network connection by an AI process.
type NetworkEvent struct {
	PID         int
	ProcessName string
	Protocol    string
	LocalPort   int
	RemoteAddr  string
	RemotePort  int
	Endpoint    string
	Label       string
	State       string
	Timestamp   time.Time
}

// InjectionAlert records a detected prompt injection attempt.
type InjectionAlert struct {
	Source    string
	Pattern   string
	Snippet   string
	Severity  string
	Timestamp time.Time
}

// DailySummary is the aggregate view used by reports and the web dashboard.
type DailySummary struct {
	Processes       []ProcessRow     `json:"processes"`
	FileAlerts      []FileAccessEvent `json:"fileAlerts"`
	NetworkEvents   []NetworkEvent   `json:"networkEvents"`
	InjectionAlerts []InjectionAlert `json:"injectionAlerts"`
}

// ProcessRow is one aggregated row in the daily process summary.
type ProcessRow struct {
	AppName      string `json:"appName"`
	Confidence   string `json:"confidence"`
	NetworkCount int    `json:"networkCount"`
	FileCount    int    `json:"fileCount"`
}

// ─── Insert methods ──────────────────────────────────────────────────────────

// InsertProcessSnapshot stores a process observation.
func (s *Store) InsertProcessSnapshot(snap ProcessSnapshot) error {
	ts := snap.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	_, err := s.db.Exec(
		`INSERT INTO process_snapshots (pid,name,app_name,cmdline,user,cpu_pct,mem_mb,confidence,score,timestamp)
         VALUES (?,?,?,?,?,?,?,?,?,?)`,
		snap.PID, snap.Name, snap.AppName, snap.Cmdline, snap.User,
		snap.CPUPercent, snap.MemMB, snap.Confidence, snap.Score,
		ts.UTC().Format(time.RFC3339),
	)
	return err
}

// InsertFileAccessEvent stores a file access event.
func (s *Store) InsertFileAccessEvent(e FileAccessEvent) error {
	ts := e.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	_, err := s.db.Exec(
		`INSERT INTO file_access_events (pid,process_name,file_path,sensitivity,timestamp) VALUES (?,?,?,?,?)`,
		e.PID, e.ProcessName, e.FilePath, e.Sensitivity, ts.UTC().Format(time.RFC3339),
	)
	return err
}

// InsertNetworkEvent stores a network connection event.
func (s *Store) InsertNetworkEvent(e NetworkEvent) error {
	ts := e.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	_, err := s.db.Exec(
		`INSERT INTO network_events (pid,process_name,protocol,local_port,remote_addr,remote_port,endpoint,label,state,timestamp)
         VALUES (?,?,?,?,?,?,?,?,?,?)`,
		e.PID, e.ProcessName, e.Protocol, e.LocalPort, e.RemoteAddr,
		e.RemotePort, e.Endpoint, e.Label, e.State, ts.UTC().Format(time.RFC3339),
	)
	return err
}

// InsertInjectionAlert stores an injection detection alert.
func (s *Store) InsertInjectionAlert(a InjectionAlert) error {
	ts := a.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	_, err := s.db.Exec(
		`INSERT INTO injection_alerts (source,pattern,snippet,severity,timestamp) VALUES (?,?,?,?,?)`,
		a.Source, a.Pattern, a.Snippet, a.Severity, ts.UTC().Format(time.RFC3339),
	)
	return err
}

// ─── Query methods ───────────────────────────────────────────────────────────

// GetRecentFileAlerts returns file access events since `since`, newest first.
// Pass zero time to get all recent events up to limit.
func (s *Store) GetRecentFileAlerts(since time.Time, limit int) ([]FileAccessEvent, error) {
	var rows interface {
		Next() bool
		Scan(...any) error
		Close() error
	}
	var err error
	if since.IsZero() {
		rows, err = s.db.Query(
			`SELECT pid,process_name,file_path,sensitivity,timestamp
             FROM file_access_events ORDER BY timestamp DESC LIMIT ?`, limit)
	} else {
		rows, err = s.db.Query(
			`SELECT pid,process_name,file_path,sensitivity,timestamp
             FROM file_access_events WHERE timestamp >= ?
             ORDER BY timestamp DESC LIMIT ?`,
			since.UTC().Format(time.RFC3339), limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []FileAccessEvent
	for rows.Next() {
		var e FileAccessEvent
		var ts string
		if err := rows.Scan(&e.PID, &e.ProcessName, &e.FilePath, &e.Sensitivity, &ts); err != nil {
			continue
		}
		e.Timestamp, _ = time.Parse(time.RFC3339, ts)
		events = append(events, e)
	}
	return events, nil
}

// GetRecentNetworkEvents returns recent network connection events since `since`.
func (s *Store) GetRecentNetworkEvents(limit int) ([]NetworkEvent, error) {
	rows, err := s.db.Query(
		`SELECT pid,process_name,protocol,local_port,remote_addr,remote_port,endpoint,label,state,timestamp
         FROM network_events ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []NetworkEvent
	for rows.Next() {
		var e NetworkEvent
		var ts string
		if err := rows.Scan(&e.PID, &e.ProcessName, &e.Protocol, &e.LocalPort,
			&e.RemoteAddr, &e.RemotePort, &e.Endpoint, &e.Label, &e.State, &ts); err != nil {
			continue
		}
		e.Timestamp, _ = time.Parse(time.RFC3339, ts)
		events = append(events, e)
	}
	return events, nil
}

// GetDailySummary returns a full daily summary for the given date.
func (s *Store) GetDailySummary(date time.Time) (*DailySummary, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	end := start.Add(24 * time.Hour)
	startStr := start.UTC().Format(time.RFC3339)
	endStr := end.UTC().Format(time.RFC3339)

	summary := &DailySummary{}

	// Aggregated process rows
	rows, err := s.db.Query(
		`SELECT app_name, confidence,
                COUNT(DISTINCT n.id) as net_cnt,
                COUNT(DISTINCT f.id) as file_cnt
         FROM process_snapshots ps
         LEFT JOIN network_events n   ON n.process_name = ps.name   AND n.timestamp   BETWEEN ? AND ?
         LEFT JOIN file_access_events f ON f.process_name = ps.name AND f.timestamp BETWEEN ? AND ?
         WHERE ps.timestamp BETWEEN ? AND ?
         GROUP BY ps.app_name, ps.confidence
         ORDER BY ps.app_name`,
		startStr, endStr, startStr, endStr, startStr, endStr)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var r ProcessRow
			_ = rows.Scan(&r.AppName, &r.Confidence, &r.NetworkCount, &r.FileCount)
			summary.Processes = append(summary.Processes, r)
		}
	}

	// File alerts for the day
	fileRows, err := s.db.Query(
		`SELECT pid,process_name,file_path,sensitivity,timestamp
         FROM file_access_events WHERE timestamp BETWEEN ? AND ?
         ORDER BY timestamp DESC LIMIT 100`, startStr, endStr)
	if err == nil {
		defer fileRows.Close()
		for fileRows.Next() {
			var e FileAccessEvent
			var ts string
			_ = fileRows.Scan(&e.PID, &e.ProcessName, &e.FilePath, &e.Sensitivity, &ts)
			e.Timestamp, _ = time.Parse(time.RFC3339, ts)
			summary.FileAlerts = append(summary.FileAlerts, e)
		}
	}

	// Network events
	netRows, err := s.db.Query(
		`SELECT pid,process_name,protocol,local_port,remote_addr,remote_port,endpoint,label,state,timestamp
         FROM network_events WHERE timestamp BETWEEN ? AND ?
         ORDER BY timestamp DESC LIMIT 100`, startStr, endStr)
	if err == nil {
		defer netRows.Close()
		for netRows.Next() {
			var e NetworkEvent
			var ts string
			_ = netRows.Scan(&e.PID, &e.ProcessName, &e.Protocol, &e.LocalPort,
				&e.RemoteAddr, &e.RemotePort, &e.Endpoint, &e.Label, &e.State, &ts)
			e.Timestamp, _ = time.Parse(time.RFC3339, ts)
			summary.NetworkEvents = append(summary.NetworkEvents, e)
		}
	}

	// Injection alerts
	injRows, err := s.db.Query(
		`SELECT source,pattern,snippet,severity,timestamp
         FROM injection_alerts WHERE timestamp BETWEEN ? AND ?
         ORDER BY timestamp DESC`, startStr, endStr)
	if err == nil {
		defer injRows.Close()
		for injRows.Next() {
			var a InjectionAlert
			var ts string
			_ = injRows.Scan(&a.Source, &a.Pattern, &a.Snippet, &a.Severity, &ts)
			a.Timestamp, _ = time.Parse(time.RFC3339, ts)
			summary.InjectionAlerts = append(summary.InjectionAlerts, a)
		}
	}

	return summary, nil
}
