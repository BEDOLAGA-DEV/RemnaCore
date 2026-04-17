package remnawave

import "fmt"

// --- Collection name constants ---

const (
	CollectionPanelConnections     = "panel_connections"
	CollectionNodeHealthLog        = "node_health_log"
	CollectionUserTrafficSnapshots = "user_traffic_snapshots"
	CollectionDriftEvents          = "drift_events"
	CollectionSquadAssignments     = "squad_assignments"
	CollectionGeoRoutingRules      = "geo_routing_rules"
)

// --- Panel status constants ---

const (
	PanelStatusActive      = "active"
	PanelStatusReadonly     = "readonly"
	PanelStatusDisabled     = "disabled"
	PanelStatusMaintenance = "maintenance"
)

// --- PanelConnection — multi-panel support ---

// PanelConnectionInput is the set of fields accepted when registering or
// updating a panel connection.
type PanelConnectionInput struct {
	Slug          string `json:"slug"`           // human-readable: "eu-primary"
	URL           string `json:"url"`
	APIToken      string `json:"api_token"`
	Region        string `json:"region"`         // "eu" | "asia" | "us" | ""
	Priority      int    `json:"priority"`       // for failover ordering
	Status        string `json:"status"`         // "active" | "readonly" | "disabled" | "maintenance"
	TLSSkipVerify bool   `json:"tls_skip_verify"`
}

// PanelConnectionResponse wraps PanelConnectionInput with server-assigned
// identity, health status, and timestamps.
type PanelConnectionResponse struct {
	ID string `json:"id"`
	PanelConnectionInput
	HealthStatus PanelHealthStatus `json:"health_status"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
}

// PanelHealthStatus captures the latest health-check state for a panel.
type PanelHealthStatus struct {
	IsUp             bool   `json:"is_up"`
	LastCheckAt      string `json:"last_check_at,omitempty"`
	LastSuccessAt    string `json:"last_success_at,omitempty"`
	ConsecutiveFails int    `json:"consecutive_fails"`
	CircuitState     string `json:"circuit_state"` // "closed" | "half_open" | "open"
	ErrorMessage     string `json:"error_message,omitempty"`
}

// --- NodeHealthEntry — for health log collection ---

// NodeHealthEntry records a single health-check result for a node.
type NodeHealthEntry struct {
	PanelID   string `json:"panel_id"`
	NodeUUID  string `json:"node_uuid"`
	NodeName  string `json:"node_name"`
	IsUp      bool   `json:"is_up"`
	CheckedAt string `json:"checked_at"`
	Error     string `json:"error,omitempty"`
}

// --- UserTrafficSnapshot ---

// UserTrafficSnapshot captures a point-in-time traffic reading for a user.
type UserTrafficSnapshot struct {
	PanelID       string `json:"panel_id"`
	UserUUID      string `json:"user_uuid"`
	Username      string `json:"username"`
	UploadBytes   int64  `json:"upload_bytes"`
	DownloadBytes int64  `json:"download_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
	SnapshotAt    string `json:"snapshot_at"`
}

// --- DriftEvent ---

// DriftEventInput is the set of fields accepted when recording a drift event.
type DriftEventInput struct {
	PanelID      string `json:"panel_id"`
	Kind         string `json:"kind"`          // "missing_in_panel" | "missing_in_core" | "status_mismatch"
	ResourceType string `json:"resource_type"` // "user" | "node"
	ResourceID   string `json:"resource_id"`
	Details      string `json:"details"`
	Status       string `json:"status"` // "open" | "resolved" | "healed"
	DetectedAt   string `json:"detected_at"`
	ResolvedAt   string `json:"resolved_at,omitempty"`
}

// DriftEventResponse wraps DriftEventInput with server-assigned identity
// and timestamps.
type DriftEventResponse struct {
	ID string `json:"id"`
	DriftEventInput
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// --- GeoRoutingRule ---

// GeoRoutingRuleInput is the set of fields accepted when creating or
// updating a geo-routing rule.
type GeoRoutingRuleInput struct {
	CountryCode string `json:"country_code"` // ISO-3166 alpha-2
	PanelID     string `json:"panel_id"`
	Priority    int    `json:"priority"`
	IsActive    bool   `json:"is_active"`
}

// GeoRoutingRuleResponse wraps GeoRoutingRuleInput with server-assigned
// identity and timestamps.
type GeoRoutingRuleResponse struct {
	ID string `json:"id"`
	GeoRoutingRuleInput
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// --- SubscriptionURL ---

// SubscriptionURL holds the URLs needed to construct a subscription link.
type SubscriptionURL struct {
	BaseURL    string            `json:"base_url"`
	ClientURLs map[string]string `json:"client_urls"` // app_name -> deep link
	DeepLink   string            `json:"deep_link,omitempty"`
}

// --- Validation functions ---

func validatePanelConnectionInput(input *PanelConnectionInput) error {
	if input.Slug == "" {
		return fmt.Errorf("slug is required")
	}
	if input.URL == "" {
		return fmt.Errorf("url is required")
	}
	if input.APIToken == "" {
		return fmt.Errorf("api_token is required")
	}
	if input.Status == "" {
		input.Status = PanelStatusActive
	}
	validStatuses := map[string]bool{
		PanelStatusActive: true, PanelStatusReadonly: true,
		PanelStatusDisabled: true, PanelStatusMaintenance: true,
	}
	if !validStatuses[input.Status] {
		return fmt.Errorf("unsupported status: %s", input.Status)
	}
	return nil
}

func validateGeoRoutingRuleInput(input *GeoRoutingRuleInput) error {
	if input.CountryCode == "" {
		return fmt.Errorf("country_code is required")
	}
	if len(input.CountryCode) != 2 {
		return fmt.Errorf("country_code must be 2 characters (ISO-3166)")
	}
	if input.PanelID == "" {
		return fmt.Errorf("panel_id is required")
	}
	return nil
}
