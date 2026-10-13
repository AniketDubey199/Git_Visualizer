package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// DayStat represents commits for a specific calendar day in JSON and web responses.
type DayStat struct {
	DayOffset int       `json:"day_offset"`
	Date      string    `json:"date"`
	DayOfWeek string    `json:"day_of_week"`
	Commits   int       `json:"commits"`
	Level     int       `json:"level"` // 0-4 GitHub activity level
}

// StatsResponse represents the JSON payload returned by the CLI --json flag and /api/stats.
type StatsResponse struct {
	Emails       []string  `json:"emails"`
	TotalCommits int       `json:"total_commits"`
	DaysAnalyzed int       `json:"days_analyzed"`
	GeneratedAt  time.Time `json:"generated_at"`
	RepoCount    int       `json:"repo_count"`
	Repositories []string  `json:"repositories"`
	Days         []DayStat `json:"days"`
}

// buildStatsResponse converts commits map into structured JSON-ready representation.
func buildStatsResponse(emailStr string, repos []string, commits map[int]int, maxDays int, now time.Time) StatsResponse {
	var emails []string
	for e := range parseEmails(emailStr) {
		emails = append(emails, e)
	}
	sort.Strings(emails)

	offset := calcOffset(now)
	nowNorm := normalizeToMidnightUTC(now)

	var days []DayStat
	total := 0

	for d := 0; d < maxDays; d++ {
		targetDate := nowNorm.AddDate(0, 0, -d)
		gridIdx := d + offset
		count := commits[gridIdx]
		total += count

		level := 0
		switch {
		case count >= 10:
			level = 4
		case count >= 5:
			level = 3
		case count >= 2:
			level = 2
		case count >= 1:
			level = 1
		}

		days = append(days, DayStat{
			DayOffset: d,
			Date:      targetDate.Format("2006-01-02"),
			DayOfWeek: targetDate.Weekday().String(),
			Commits:   count,
			Level:     level,
		})
	}

	return StatsResponse{
		Emails:       emails,
		TotalCommits: total,
		DaysAnalyzed: maxDays,
		GeneratedAt:  now,
		RepoCount:    len(repos),
		Repositories: repos,
		Days:         days,
	}
}

// outputJSON prints stats as formatted JSON to stdout.
func outputJSON(emailStr string, workers int) {
	repos := parseFileToSlice(getDotFilePath())
	commits := processRepositoriesConcurrent(emailStr, repos, daysInLastSixMonths, workers, time.Now())
	resp := buildStatsResponse(emailStr, repos, commits, daysInLastSixMonths, time.Now())

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		fmt.Printf(`{"error": %q}`+"\n", err.Error())
		return
	}
	fmt.Println(string(data))
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Git Visualizer - Commit Heatmap</title>
  <style>
    :root {
      --bg: #0d1117;
      --card-bg: #161b22;
      --border: #30363d;
      --text: #c9d1d9;
      --text-muted: #8b949e;
      --cell-0: #161b22;
      --cell-1: #0e4429;
      --cell-2: #006d32;
      --cell-3: #26a641;
      --cell-4: #39d353;
      --accent: #58a6ff;
    }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
      background: var(--bg);
      color: var(--text);
      margin: 0;
      padding: 32px 24px;
      display: flex;
      flex-direction: column;
      align-items: center;
    }
    .container {
      max-width: 1040px;
      width: 100%;
    }
    header {
      margin-bottom: 24px;
      display: flex;
      justify-content: space-between;
      align-items: baseline;
      border-bottom: 1px solid var(--border);
      padding-bottom: 16px;
    }
    h1 { margin: 0; font-size: 24px; color: #f0f6fc; }
    .badge {
      background: #21262d;
      border: 1px solid var(--border);
      padding: 4px 10px;
      border-radius: 12px;
      font-size: 13px;
      color: var(--accent);
    }
    .stats-card {
      background: var(--card-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 24px;
      margin-bottom: 24px;
      box-shadow: 0 4px 12px rgba(0,0,0,0.3);
    }
    .metrics {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
      gap: 16px;
      margin-bottom: 24px;
    }
    .metric {
      background: #0d1117;
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 14px 18px;
    }
    .metric-value { font-size: 26px; font-weight: 700; color: #58a6ff; }
    .metric-label { font-size: 12px; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.5px; }
    
    .heatmap-wrapper {
      overflow-x: auto;
      padding: 12px 0;
    }
    .heatmap {
      display: inline-grid;
      grid-template-rows: repeat(7, 14px);
      grid-auto-flow: column;
      grid-auto-columns: 14px;
      gap: 4px;
    }
    .cell {
      width: 14px;
      height: 14px;
      border-radius: 2px;
      border: 1px solid rgba(27,31,35,0.06);
      cursor: pointer;
      position: relative;
    }
    .cell[data-level="0"] { background: var(--cell-0); border-color: rgba(240,246,252,0.05); }
    .cell[data-level="1"] { background: var(--cell-1); }
    .cell[data-level="2"] { background: var(--cell-2); }
    .cell[data-level="3"] { background: var(--cell-3); }
    .cell[data-level="4"] { background: var(--cell-4); }

    .tooltip {
      position: fixed;
      pointer-events: none;
      background: #1f242c;
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 6px 12px;
      font-size: 12px;
      color: #fff;
      display: none;
      z-index: 1000;
      box-shadow: 0 8px 24px rgba(0,0,0,0.5);
    }
    .legend {
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: 6px;
      margin-top: 16px;
      font-size: 12px;
      color: var(--text-muted);
    }
    .legend-cell {
      width: 12px;
      height: 12px;
      border-radius: 2px;
    }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <div>
        <h1>Git Activity Visualizer</h1>
        <div style="font-size: 13px; color: var(--text-muted); margin-top: 4px;">Local Git repositories commit heatmap</div>
      </div>
      <div id="email-badge" class="badge">Loading...</div>
    </header>

    <div class="stats-card">
      <div class="metrics">
        <div class="metric">
          <div class="metric-value" id="total-commits">-</div>
          <div class="metric-label">Commits (Last 6 Months)</div>
        </div>
        <div class="metric">
          <div class="metric-value" id="repo-count">-</div>
          <div class="metric-label">Repositories Tracked</div>
        </div>
        <div class="metric">
          <div class="metric-value" id="active-days">-</div>
          <div class="metric-label">Active Commit Days</div>
        </div>
      </div>

      <div class="heatmap-wrapper">
        <div id="heatmap" class="heatmap"></div>
      </div>

      <div class="legend">
        <span>Less</span>
        <div class="legend-cell" style="background: var(--cell-0); border: 1px solid var(--border)"></div>
        <div class="legend-cell" style="background: var(--cell-1)"></div>
        <div class="legend-cell" style="background: var(--cell-2)"></div>
        <div class="legend-cell" style="background: var(--cell-3)"></div>
        <div class="legend-cell" style="background: var(--cell-4)"></div>
        <span>More</span>
      </div>
    </div>
  </div>

  <div id="tooltip" class="tooltip"></div>

  <script>
    const tooltip = document.getElementById('tooltip');

    fetch('/api/stats')
      .then(res => res.json())
      .then(data => {
        document.getElementById('email-badge').textContent = data.emails.join(', ');
        document.getElementById('total-commits').textContent = data.total_commits;
        document.getElementById('repo-count').textContent = data.repo_count;

        const activeDays = data.days.filter(d => d.commits > 0).length;
        document.getElementById('active-days').textContent = activeDays;

        const heatmap = document.getElementById('heatmap');
        // Render cells chronologically from left to right (reverse of offset)
        const reversed = [...data.days].reverse();
        reversed.forEach(day => {
          const cell = document.createElement('div');
          cell.className = 'cell';
          cell.setAttribute('data-level', day.level);
          cell.setAttribute('data-date', day.date);
          cell.setAttribute('data-commits', day.commits);

          cell.addEventListener('mouseenter', (e) => {
            tooltip.innerHTML = '<strong>' + (day.commits === 1 ? '1 commit' : day.commits + ' commits') + '</strong> on ' + day.date + ' (' + day.day_of_week + ')';
            tooltip.style.display = 'block';
            tooltip.style.left = (e.clientX + 12) + 'px';
            tooltip.style.top = (e.clientY + 12) + 'px';
          });
          cell.addEventListener('mousemove', (e) => {
            tooltip.style.left = (e.clientX + 12) + 'px';
            tooltip.style.top = (e.clientY + 12) + 'px';
          });
          cell.addEventListener('mouseleave', () => {
            tooltip.style.display = 'none';
          });

          heatmap.appendChild(cell);
        });
      })
      .catch(err => {
        console.error('Failed to load stats:', err);
      });
  </script>
</body>
</html>
`

// startServer starts a local HTTP server serving the interactive web dashboard and JSON API.
func startServer(emailStr string, workers int, port int) {
	mux := http.NewServeMux()

	// API endpoint returning structured commit JSON
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		repos := parseFileToSlice(getDotFilePath())
		commits := processRepositoriesConcurrent(emailStr, repos, daysInLastSixMonths, workers, time.Now())
		resp := buildStatsResponse(emailStr, repos, commits, daysInLastSixMonths, time.Now())

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// Web UI
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(htmlTemplate))
	})

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Git Visualizer Web Server listening on http://localhost:%d\n", port)
	fmt.Printf("JSON API available at http://localhost:%d/api/stats\n", port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Printf("Server stopped: %v\n", err)
	}
}
