package remnawave

// The struct shapes below mirror the Remnawave 2.8.0 system/metadata/ip-control
// contracts (identical in 2.7.0). The prior definitions used field names absent
// from the contract, so Go's tolerant json.Unmarshal decoded them to zero values
// — endpoints returned empty data without erroring. These match the real wire
// shapes so the data is actually populated.

// SystemHealth is the /system/health response: per-instance runtime metrics.
type SystemHealth struct {
	RuntimeMetrics []RuntimeMetric `json:"runtimeMetrics"`
}

// RuntimeMetric is one instance's runtime memory metrics.
type RuntimeMetric struct {
	InstanceID   string `json:"instanceId"`
	InstanceType string `json:"instanceType"`
	RSS          int64  `json:"rss"`
	HeapUsed     int64  `json:"heapUsed"`
	HeapTotal    int64  `json:"heapTotal"`
}

// SystemMetadata is the /system/metadata response.
type SystemMetadata struct {
	Version string          `json:"version"`
	Build   MetadataBuild   `json:"build"`
	Git     MetadataGitInfo `json:"git"`
}

// MetadataBuild carries build time and number.
type MetadataBuild struct {
	Time   string `json:"time"`
	Number string `json:"number"`
}

// MetadataGitInfo carries backend/frontend git provenance.
type MetadataGitInfo struct {
	Backend  MetadataGitRepo `json:"backend"`
	Frontend MetadataGitRepo `json:"frontend"`
}

// MetadataGitRepo is a single repo's git provenance.
type MetadataGitRepo struct {
	CommitSha string `json:"commitSha"`
	Branch    string `json:"branch"`
	CommitURL string `json:"commitUrl"`
}

// SystemStats is the /system/stats response.
type SystemStats struct {
	CPU         StatsCPU         `json:"cpu"`
	Memory      StatsMemory      `json:"memory"`
	Uptime      int64            `json:"uptime"`
	Timestamp   int64            `json:"timestamp"`
	Users       StatsUsers       `json:"users"`
	OnlineStats StatsOnline      `json:"onlineStats"`
	Nodes       StatsNodesSystem `json:"nodes"`
}

type StatsCPU struct {
	Cores int `json:"cores"`
}

type StatsMemory struct {
	Total int64 `json:"total"`
	Free  int64 `json:"free"`
	Used  int64 `json:"used"`
}

type StatsUsers struct {
	StatusCounts map[string]int `json:"statusCounts"`
	TotalUsers   int            `json:"totalUsers"`
}

type StatsOnline struct {
	LastDay     int `json:"lastDay"`
	LastWeek    int `json:"lastWeek"`
	NeverOnline int `json:"neverOnline"`
	OnlineNow   int `json:"onlineNow"`
}

type StatsNodesSystem struct {
	TotalOnline        int   `json:"totalOnline"`
	TotalBytesLifetime int64 `json:"totalBytesLifetime"`
}

// BandwidthPeriod is one period's bandwidth stat (BaseStat in the contract).
type BandwidthPeriod struct {
	Current    string `json:"current"`
	Previous   string `json:"previous"`
	Difference string `json:"difference"`
}

// BandwidthSystemStats is the /system/stats/bandwidth response: period deltas.
type BandwidthSystemStats struct {
	LastTwoDays   BandwidthPeriod `json:"bandwidthLastTwoDays"`
	LastSevenDays BandwidthPeriod `json:"bandwidthLastSevenDays"`
	Last30Days    BandwidthPeriod `json:"bandwidthLast30Days"`
	CalendarMonth BandwidthPeriod `json:"bandwidthCalendarMonth"`
	CurrentYear   BandwidthPeriod `json:"bandwidthCurrentYear"`
}

// NodesSystemStats is the /system/nodes/statistics response (last-7-days series).
type NodesSystemStats struct {
	LastSevenDays []NodeDailyUsage `json:"lastSevenDays"`
}

// NodeDailyUsage is one node's total for a day.
type NodeDailyUsage struct {
	NodeName  string `json:"nodeName"`
	Date      string `json:"date"`
	TotalGiB  string `json:"totalGib"`
	TotalDown string `json:"totalDownload"`
	TotalUp   string `json:"totalUpload"`
}

// NodeMetrics is one node's realtime metrics (/system/nodes/metrics nodes[]).
type NodeMetrics struct {
	NodeUUID      string       `json:"nodeUuid"`
	NodeName      string       `json:"nodeName"`
	CountryEmoji  string       `json:"countryEmoji"`
	ProviderName  string       `json:"providerName"`
	UsersOnline   int          `json:"usersOnline"`
	InboundsStats []NodeIOStat `json:"inboundsStats"`
	OutboundsStat []NodeIOStat `json:"outboundsStats"`
}

// NodeIOStat is one inbound/outbound tag's upload/download.
type NodeIOStat struct {
	Tag      string `json:"tag"`
	Upload   string `json:"upload"`
	Download string `json:"download"`
}

// StatsRecap is the /system/stats/recap response.
type StatsRecap struct {
	UsersByStatus map[string]int `json:"usersByStatus"`
	TotalUsers    int            `json:"totalUsers"`
}

// IPFetchJob is the fetch-ips response: just a job id.
type IPFetchJob struct {
	JobID string `json:"jobId"`
}

// IPFetchProgress tracks an async IP-fetch job's progress.
type IPFetchProgress struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Percent   int `json:"percent"`
}

// IPFetchNodeResult is one node's IPs for a user in a fetch result.
type IPFetchNodeResult struct {
	NodeUUID    string      `json:"nodeUuid"`
	NodeName    string      `json:"nodeName"`
	CountryCode string      `json:"countryCode"`
	IPs         []IPFetchIP `json:"ips"`
}

// IPFetchIP is a single observed IP with its last-seen time.
type IPFetchIP struct {
	IP       string `json:"ip"`
	LastSeen string `json:"lastSeen"`
}

// IPFetchInner is the nested "result" object of a completed fetch.
type IPFetchInner struct {
	Success  bool                `json:"success"`
	UserUUID string              `json:"userUuid"`
	UserID   int64               `json:"userId"`
	Nodes    []IPFetchNodeResult `json:"nodes"`
}

// IPFetchResult is the fetch-ips-result response.
type IPFetchResult struct {
	IsCompleted bool            `json:"isCompleted"`
	IsFailed    bool            `json:"isFailed"`
	Progress    IPFetchProgress `json:"progress"`
	Result      *IPFetchInner   `json:"result"`
}

// DropConnectionsBy selects WHICH connections to drop (discriminated union:
// by = "userUuids" or "ipAddresses").
type DropConnectionsBy struct {
	By          string   `json:"by"`
	UserUuids   []string `json:"userUuids,omitempty"`
	IpAddresses []string `json:"ipAddresses,omitempty"`
}

// DropConnectionsTarget selects on WHICH nodes to drop (discriminated union:
// target = "allNodes" or "specificNodes").
type DropConnectionsTarget struct {
	Target    string   `json:"target"`
	NodeUuids []string `json:"nodeUuids,omitempty"`
}

// DropConnectionsRequest is the payload to drop active connections. The 2.8.0
// (and 2.7.0) contract uses two discriminated unions; the previous flat
// {userUuid, nodeUuid} shape failed Zod validation.
type DropConnectionsRequest struct {
	DropBy      DropConnectionsBy     `json:"dropBy"`
	TargetNodes DropConnectionsTarget `json:"targetNodes"`
}

// NewDropUserConnections builds a request that drops all of a single user's
// connections across every node.
func NewDropUserConnections(userUUID string) DropConnectionsRequest {
	return DropConnectionsRequest{
		DropBy:      DropConnectionsBy{By: "userUuids", UserUuids: []string{userUUID}},
		TargetNodes: DropConnectionsTarget{Target: "allNodes"},
	}
}
