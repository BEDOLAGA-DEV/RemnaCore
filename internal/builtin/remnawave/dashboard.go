package remnawave

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// GetDashboardOverview returns aggregate stats across all panels.
func (h *Handler) GetDashboardOverview(w http.ResponseWriter, r *http.Request) {
	clients, err := h.getAllPanelClients(r.Context())
	if err != nil || len(clients) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"panels_up":     0,
			"panels_total":  0,
			"nodes_healthy": 0,
			"nodes_total":   0,
			"active_users":  0,
			"bandwidth":     map[string]int64{"upload": 0, "download": 0, "total": 0},
		})
		return
	}

	totalPanels := len(clients)
	panelsUp := 0
	totalNodes := 0
	healthyNodes := 0
	totalActiveUsers := 0
	var totalUpload, totalDownload int64

	for panelID, client := range clients {
		stats, err := client.GetSystemStats(r.Context())
		if err != nil {
			h.logger.Warn("failed to get stats", slog.String("panel_id", panelID), slog.Any("error", err))
			continue
		}
		panelsUp++
		totalNodes += stats.TotalNodes
		healthyNodes += stats.ConnectedNodes
		totalActiveUsers += stats.ActiveUsers

		bwStats, err := client.GetBandwidthSystemStats(r.Context())
		if err == nil {
			totalUpload += bwStats.TotalUploadBytes
			totalDownload += bwStats.TotalDownloadBytes
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"panels_up":     panelsUp,
		"panels_total":  totalPanels,
		"nodes_healthy": healthyNodes,
		"nodes_total":   totalNodes,
		"active_users":  totalActiveUsers,
		"bandwidth": map[string]int64{
			"upload":   totalUpload,
			"download": totalDownload,
			"total":    totalUpload + totalDownload,
		},
	})
}

// GetDashboardRealtime returns live connections and bandwidth metrics.
func (h *Handler) GetDashboardRealtime(w http.ResponseWriter, r *http.Request) {
	clients, err := h.getAllPanelClients(r.Context())
	if err != nil || len(clients) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"panels": []any{}})
		return
	}

	type panelRealtime struct {
		PanelID string `json:"panel_id"`
		Slug    string `json:"slug"`
		Online  bool   `json:"online"`
		Metrics any    `json:"metrics"`
	}

	// Load panel slugs from collection
	slugMap := make(map[string]string)
	if docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionPanelConnections); err == nil {
		for _, doc := range docs {
			var p PanelConnectionInput
			if json.Unmarshal(doc.Data, &p) == nil {
				slugMap[doc.ID] = p.Slug
			}
		}
	}

	panels := make([]panelRealtime, 0)
	for panelID, client := range clients {
		slug := slugMap[panelID]
		if slug == "" {
			slug = panelID
		}

		entry := panelRealtime{PanelID: panelID, Slug: slug, Online: true}

		// Try realtime metrics (may not be supported by panel version)
		metrics, err := client.GetRealtimeNodeMetrics(r.Context())
		if err == nil {
			entry.Metrics = metrics
		}

		// Verify panel is actually reachable
		if _, healthErr := client.GetSystemHealth(r.Context()); healthErr != nil {
			entry.Online = false
		}

		panels = append(panels, entry)
	}

	writeJSON(w, http.StatusOK, map[string]any{"panels": panels})
}

// GetTrafficTopUsers returns top users by traffic consumption.
func (h *Handler) GetTrafficTopUsers(w http.ResponseWriter, r *http.Request) {
	clients, err := h.getAllPanelClients(r.Context())
	if err != nil || len(clients) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"users": []any{}})
		return
	}

	type userTraffic struct {
		PanelID  string `json:"panel_id"`
		UserData any    `json:"user_data"`
	}

	allUsers := make([]userTraffic, 0)
	for panelID, client := range clients {
		bwStats, err := client.GetNodesBandwidthStats(r.Context())
		if err != nil {
			h.logger.Warn("failed to get bandwidth stats", slog.String("panel_id", panelID), slog.Any("error", err))
			continue
		}
		for _, stat := range bwStats {
			allUsers = append(allUsers, userTraffic{PanelID: panelID, UserData: stat})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": allUsers})
}

// GetTrafficByNode returns traffic breakdown by node.
func (h *Handler) GetTrafficByNode(w http.ResponseWriter, r *http.Request) {
	clients, err := h.getAllPanelClients(r.Context())
	if err != nil || len(clients) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"nodes": []any{}})
		return
	}

	type nodeTraffic struct {
		PanelID string `json:"panel_id"`
		Stats   any    `json:"stats"`
	}

	all := make([]nodeTraffic, 0)
	for panelID, client := range clients {
		stats, err := client.GetNodesBandwidthStats(r.Context())
		if err != nil {
			continue
		}
		for _, s := range stats {
			all = append(all, nodeTraffic{PanelID: panelID, Stats: s})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"nodes": all})
}

// GetTrafficByCountry returns traffic grouped by node country.
func (h *Handler) GetTrafficByCountry(w http.ResponseWriter, r *http.Request) {
	clients, err := h.getAllPanelClients(r.Context())
	if err != nil || len(clients) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"countries": map[string]any{}})
		return
	}

	countryTraffic := make(map[string]int64)
	for _, client := range clients {
		// Use bandwidth stats grouped by node — country info requires
		// node metadata which the basic GetNodes doesn't return.
		stats, err := client.GetNodesBandwidthStats(r.Context())
		if err != nil {
			continue
		}
		for _, s := range stats {
			// Group by node name as proxy for country until metadata is available
			countryTraffic[s.Name] += s.TotalBytes
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"countries": countryTraffic})
}

// GetBandwidthTrends returns bandwidth trend data (placeholder for time-series data).
func (h *Handler) GetBandwidthTrends(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"period":      r.URL.Query().Get("period"),
		"granularity": r.URL.Query().Get("granularity"),
		"data_points": []any{},
		"note":        "requires traffic_sync background job for historical data",
	})
}

// GetUserRetention returns user retention cohort data (placeholder).
func (h *Handler) GetUserRetention(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"cohort":  r.URL.Query().Get("cohort"),
		"periods": []any{},
	})
}

// GetHWIDDistribution returns HWID device distribution across users.
func (h *Handler) GetHWIDDistribution(w http.ResponseWriter, r *http.Request) {
	clients, err := h.getAllPanelClients(r.Context())
	if err != nil || len(clients) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"distribution": map[string]any{}})
		return
	}

	type hwidData struct {
		PanelID string `json:"panel_id"`
		Stats   any    `json:"stats"`
		Top     any    `json:"top_users"`
	}

	all := make([]hwidData, 0)
	for panelID, client := range clients {
		stats, err := client.GetHWIDStats(r.Context())
		if err != nil {
			continue
		}
		top, _ := client.GetTopHWIDUsers(r.Context())
		all = append(all, hwidData{PanelID: panelID, Stats: stats, Top: top})
	}

	writeJSON(w, http.StatusOK, map[string]any{"distribution": all})
}

// GetClientAppDistribution returns client app usage stats (placeholder).
func (h *Handler) GetClientAppDistribution(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"apps": map[string]int{},
		"note": "requires SRH (Subscription Request Header) analysis",
	})
}
