package remnawave

import "time"

// HWIDDevice represents a hardware-identified device registered to a user.
type HWIDDevice struct {
	UUID      string    `json:"uuid"`
	DeviceID  string    `json:"deviceId"`
	UserUUID  string    `json:"userUuid"`
	CreatedAt time.Time `json:"createdAt"`
}

// CreateHWIDDeviceRequest is the payload to register a new HWID device.
type CreateHWIDDeviceRequest struct {
	DeviceID string `json:"deviceId"`
	UserUUID string `json:"userUuid"`
}

// DeleteHWIDDeviceRequest is the payload to remove a specific HWID device.
type DeleteHWIDDeviceRequest struct {
	UUID     string `json:"uuid"`
	UserUUID string `json:"userUuid"`
}

// HWIDStats contains aggregate HWID statistics.
type HWIDStats struct {
	TotalDevices int `json:"totalDevices"`
	TotalUsers   int `json:"totalUsers"`
}

// TopHWIDUser represents a user with the most registered HWID devices.
type TopHWIDUser struct {
	UserUUID    string `json:"userUuid"`
	Username    string `json:"username"`
	DeviceCount int    `json:"deviceCount"`
}
