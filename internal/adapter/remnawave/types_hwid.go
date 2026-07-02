package remnawave

import "time"

// HWIDDevice represents a hardware-identified device registered to a user.
// Remnawave 2.8.0 keys the device identity on `hwid` (not deviceId) and
// references the owning user by numeric userId (not a userUuid string).
type HWIDDevice struct {
	UUID      string    `json:"uuid"`
	Hwid      string    `json:"hwid"`
	UserID    int64     `json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
}

// HWIDDeviceList is the {total, devices[]} envelope the list/create/get-by-user
// endpoints return in 2.8.0.
type HWIDDeviceList struct {
	Total   int          `json:"total"`
	Devices []HWIDDevice `json:"devices"`
}

// CreateHWIDDeviceRequest is the payload to register a new HWID device. The
// device identifier field is `hwid` in the contract.
type CreateHWIDDeviceRequest struct {
	Hwid     string `json:"hwid"`
	UserUUID string `json:"userUuid"`
}

// DeleteHWIDDeviceRequest is the POST body to remove a specific HWID device
// (POST /api/hwid/devices/delete, body {hwid, userUuid}).
type DeleteHWIDDeviceRequest struct {
	Hwid     string `json:"hwid"`
	UserUUID string `json:"userUuid"`
}

// DeleteAllHWIDDevicesRequest is the POST body to wipe all of a user's devices
// (POST /api/hwid/devices/delete-all, body {userUuid}).
type DeleteAllHWIDDevicesRequest struct {
	UserUUID string `json:"userUuid"`
}

// HWIDStats is the aggregate HWID statistics response.
type HWIDStats struct {
	Stats      HWIDStatsSummary   `json:"stats"`
	ByPlatform []HWIDPlatformStat `json:"byPlatform"`
}

// HWIDStatsSummary holds the top-level device totals.
type HWIDStatsSummary struct {
	TotalUniqueDevices        int     `json:"totalUniqueDevices"`
	TotalHwidDevices          int     `json:"totalHwidDevices"`
	AverageHwidDevicesPerUser float64 `json:"averageHwidDevicesPerUser"`
}

// HWIDPlatformStat is per-platform device counts with a per-app breakdown.
type HWIDPlatformStat struct {
	Platform string        `json:"platform"`
	Count    int           `json:"count"`
	ByApp    []HWIDAppStat `json:"byApp"`
}

// HWIDAppStat is per-app device counts within a platform.
type HWIDAppStat struct {
	App   string `json:"app"`
	Count int    `json:"count"`
}

// TopHWIDUsersResponse is the {users[], total} envelope for the top-users endpoint.
type TopHWIDUsersResponse struct {
	Users []TopHWIDUser `json:"users"`
	Total int           `json:"total"`
}

// TopHWIDUser represents a user with the most registered HWID devices.
type TopHWIDUser struct {
	UserUUID     string `json:"userUuid"`
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	DevicesCount int    `json:"devicesCount"`
}
