package webapp

import (
	"github.com/cybre/fingerbot-web/internal/tuyable/fingerbot"
)

type IndexData struct {
	BatteryStatus BatteryStatusData
	ID            string
}

func NewIndexData(device *fingerbot.Fingerbot) IndexData {
	return IndexData{
		BatteryStatus: NewBatteryStatusData(device),
		ID:            device.ID(),
	}
}

type ConfigurationData struct {
	ID               string `json:"id"`
	Mode             uint32 `json:"mode"`
	ClickSustainTime int32  `json:"clickSustainTime"`
	ControlBack      uint32 `json:"controlBack"`
	ArmDownPercent   int32  `json:"armDownPercent"`
	ArmUpPercent     int32  `json:"armUpPercent"`
}

func NewConfigurationData(device *fingerbot.Fingerbot) ConfigurationData {
	return ConfigurationData{
		ID:               device.ID(),
		Mode:             uint32(device.Mode()),
		ClickSustainTime: device.ClickSustainTime(),
		ControlBack:      uint32(device.ControlBack()),
		ArmDownPercent:   device.ArmDownPercent(),
		ArmUpPercent:     device.ArmUpPercent(),
	}
}

type BatteryStatusData struct {
	BatteryLevel int32 `json:"batteryLevel"`
	IsCharging   bool  `json:"isCharging"`
}

func NewBatteryStatusData(device *fingerbot.Fingerbot) BatteryStatusData {
	return BatteryStatusData{
		BatteryLevel: device.BatteryPercent(),
		IsCharging:   device.ChargeStatus() != fingerbot.ChargeStatusNone,
	}
}

type ConnectDeviceRequest struct {
	Address  string `form:"address"`
	DeviceID string `form:"deviceId"`
	Name     string `form:"name"`
	LocalKey string `form:"localKey"`
}
