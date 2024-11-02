package fingerbot

import (
	"fmt"

	"github.com/cybre/fingerbot-web/internal/tuyable"
	"github.com/cybre/fingerbot-web/internal/utils"
)

const (
	SwitchDP           = 1
	ModeDP             = 2
	ClickSustainTimeDP = 3
	ControlBackDP      = 4
	ArmDownPercentDP   = 5
	ArmUpPercentDP     = 6
	ChargeStatusDP     = 7
	BatteryPercentDP   = 8
)

const (
	MinClickSustainTime = 0
	MaxClickSustainTime = 10
)

type Fingerbot struct {
	*tuyable.Device
}

func NewFingerbot(device *tuyable.Device) *Fingerbot {
	return &Fingerbot{
		Device: device,
	}
}

func (c *Fingerbot) Transaction(callback func(*FingerbotTransaction) error) error {
	transaction := &FingerbotTransaction{
		unconmmited: make(map[byte]tuyable.DataPoint),
		parent:      c,
	}

	if err := callback(transaction); err != nil {
		return fmt.Errorf("error in transaction: %w", err)
	}

	if len(transaction.errors) > 0 {
		return fmt.Errorf("transaction errors: %v", transaction.errors)
	}

	return c.SetDatapoints(utils.MapValues(transaction.unconmmited))
}

func (c *Fingerbot) Switch() bool {
	dp, ok := c.GetDatapoint(SwitchDP)
	if !ok {
		return false
	}

	return dp.Value.(bool)
}

func (c *Fingerbot) SetSwitch(open bool) error {
	return c.SetDatapoint(tuyable.NewDataPoint(SwitchDP, tuyable.DPTypeBool, open))
}

func (c *Fingerbot) Mode() Mode {
	dp, ok := c.GetDatapoint(ModeDP)
	if !ok {
		return ModeClick
	}

	return Mode(dp.Value.(uint32))
}

func (c *Fingerbot) SetMode(mode Mode) error {
	if !mode.Valid() {
		return fmt.Errorf("invalid mode: %d", mode)
	}

	return c.SetDatapoint(tuyable.NewDataPoint(ModeDP, tuyable.DPTypeEnum, uint32(mode)))
}

func (c *Fingerbot) ClickSustainTime() int32 {
	dp, ok := c.GetDatapoint(ClickSustainTimeDP)
	if !ok {
		return 0
	}

	return dp.Value.(int32)
}

func (c *Fingerbot) SetClickSustainTime(seconds int32) error {
	if seconds < MinClickSustainTime || seconds > MaxClickSustainTime {
		return fmt.Errorf("invalid click sustain time: %d", seconds)
	}

	return c.SetDatapoint(tuyable.NewDataPoint(ClickSustainTimeDP, tuyable.DPTypeValue, seconds))
}

func (c *Fingerbot) ControlBack() ControlBack {
	dp, ok := c.GetDatapoint(ControlBackDP)
	if !ok {
		return ControlBackUp
	}

	return ControlBack(dp.Value.(uint32))
}

func (c *Fingerbot) SetControlBack(back ControlBack) error {
	if !back.Valid() {
		return fmt.Errorf("invalid control back: %d", back)
	}

	return c.SetDatapoint(tuyable.NewDataPoint(ControlBackDP, tuyable.DPTypeEnum, uint32(back)))
}

func (c *Fingerbot) ArmDownPercent() int32 {
	dp, ok := c.GetDatapoint(ArmDownPercentDP)
	if !ok {
		return 100
	}

	return dp.Value.(int32)
}

func (c *Fingerbot) SetArmDownPercent(percent int32) error {
	if percent < 0 || percent > 100 {
		return fmt.Errorf("invalid arm down percent: %d", percent)
	}
	if percent < c.ArmUpPercent() {
		return fmt.Errorf("arm down percent cannot be less than arm up percent")
	}

	return c.SetDatapoint(tuyable.NewDataPoint(ArmDownPercentDP, tuyable.DPTypeValue, percent))
}

func (c *Fingerbot) ArmUpPercent() int32 {
	dp, ok := c.GetDatapoint(ArmUpPercentDP)
	if !ok {
		return 20
	}

	return dp.Value.(int32)
}

func (c *Fingerbot) SetArmUpPercent(percent int32) error {
	if percent < 0 || percent > 100 {
		return fmt.Errorf("invalid arm up percent: %d", percent)
	}
	if percent < c.ArmDownPercent() {
		return fmt.Errorf("arm up percent cannot be greater than arm down percent")
	}

	return c.SetDatapoint(tuyable.NewDataPoint(ArmUpPercentDP, tuyable.DPTypeValue, percent))
}

func (c *Fingerbot) ChargeStatus() ChargeStatus {
	dp, ok := c.GetDatapoint(ChargeStatusDP)
	if !ok {
		return ChargeStatusNone
	}

	return ChargeStatus(dp.Value.(uint32))
}

func (c *Fingerbot) BatteryPercent() int32 {
	dp, ok := c.GetDatapoint(BatteryPercentDP)
	if !ok {
		return 0
	}

	return dp.Value.(int32)
}
