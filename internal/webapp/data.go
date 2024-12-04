package webapp

import (
	"github.com/cybre/fingerbot-web/internal/tuyable/fingerbot"
	"github.com/cybre/fingerbot-web/internal/utils"
)

type DeviceDropdownItem struct {
	Address string
	Name    string
}

func NewDeviceDropdownItems(devs []*fingerbot.Fingerbot) []DeviceDropdownItem {
	return utils.Map(devs, func(d *fingerbot.Fingerbot) DeviceDropdownItem {
		return DeviceDropdownItem{
			Address: d.Address(),
			Name:    d.Name(),
		}
	})
}

type IndexData struct {
	BatteryStatus BatteryStatusData
	Name          string
	Address       string
	Devices       []DeviceDropdownItem
}

func NewIndexData(device *fingerbot.Fingerbot, allDevices []*fingerbot.Fingerbot) IndexData {
	currentDevice, _ := utils.Find(allDevices, func(d *fingerbot.Fingerbot) bool { return d.Address() == device.Address() })
	return IndexData{
		BatteryStatus: NewBatteryStatusData(device),
		Address:       currentDevice.Address(),
		Name:          currentDevice.Name(),
		Devices:       NewDeviceDropdownItems(utils.Filter(allDevices, func(d *fingerbot.Fingerbot) bool { return d.Address() != currentDevice.Address() })),
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
		ID:               device.Address(),
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
