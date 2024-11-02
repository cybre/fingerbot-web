package fingerbot

import (
	"fmt"

	"github.com/cybre/fingerbot-web/internal/tuyable"
)

type FingerbotTransaction struct {
	unconmmited map[byte]tuyable.DataPoint
	parent      *Fingerbot
	errors      []error
}

func (c *FingerbotTransaction) Switch() bool {
	if dp, ok := c.unconmmited[SwitchDP]; ok {
		return dp.Value.(bool)
	}

	return c.parent.Switch()
}

func (c *FingerbotTransaction) SetSwitch(open bool) {
	c.unconmmited[SwitchDP] = tuyable.NewDataPoint(SwitchDP, tuyable.DPTypeBool, open)
}

func (c *FingerbotTransaction) Mode() Mode {
	if dp, ok := c.unconmmited[ModeDP]; ok {
		return Mode(dp.Value.(uint32))
	}

	return c.parent.Mode()
}

func (c *FingerbotTransaction) SetMode(mode Mode) {
	if !mode.Valid() {
		c.errors = append(c.errors, fmt.Errorf("invalid mode: %d", mode))
		return
	}

	c.unconmmited[ModeDP] = tuyable.NewDataPoint(ModeDP, tuyable.DPTypeEnum, uint32(mode))
}

func (c *FingerbotTransaction) ClickSustainTime() int32 {
	if dp, ok := c.unconmmited[ClickSustainTimeDP]; ok {
		return dp.Value.(int32)
	}

	return c.parent.ClickSustainTime()
}

func (c *FingerbotTransaction) SetClickSustainTime(seconds int32) {
	if seconds < MinClickSustainTime || seconds > MaxClickSustainTime {
		c.errors = append(c.errors, fmt.Errorf("invalid click sustain time: %d", seconds))
		return
	}

	c.unconmmited[ClickSustainTimeDP] = tuyable.NewDataPoint(ClickSustainTimeDP, tuyable.DPTypeValue, seconds)
}

func (c *FingerbotTransaction) ControlBack() ControlBack {
	if dp, ok := c.unconmmited[ControlBackDP]; ok {
		return ControlBack(dp.Value.(uint32))
	}

	return c.parent.ControlBack()
}

func (c *FingerbotTransaction) SetControlBack(back ControlBack) {
	if !back.Valid() {
		c.errors = append(c.errors, fmt.Errorf("invalid control back: %d", back))
		return
	}

	c.unconmmited[ControlBackDP] = tuyable.NewDataPoint(ControlBackDP, tuyable.DPTypeEnum, uint32(back))
}

func (c *FingerbotTransaction) ArmDownPercent() int32 {
	if dp, ok := c.unconmmited[ArmDownPercentDP]; ok {
		return dp.Value.(int32)
	}

	return c.parent.ArmDownPercent()
}

func (c *FingerbotTransaction) ArmUpPercent() int32 {
	if dp, ok := c.unconmmited[ArmUpPercentDP]; ok {
		return dp.Value.(int32)
	}

	return c.parent.ArmUpPercent()
}

func (c *FingerbotTransaction) SetArmPercent(armUpPercent, armDownPercent int32) {
	if armDownPercent < 0 || armDownPercent > 100 {
		c.errors = append(c.errors, fmt.Errorf("invalid arm down percent: %d", armDownPercent))
	}

	if armUpPercent < 0 || armUpPercent > 100 {
		c.errors = append(c.errors, fmt.Errorf("invalid arm up percent: %d", armUpPercent))
	}

	if armDownPercent < armUpPercent {
		c.errors = append(c.errors, fmt.Errorf("arm down percent cannot be less than arm up percent"))
	}

	if armUpPercent > armDownPercent {
		c.errors = append(c.errors, fmt.Errorf("arm up percent cannot be greater than arm down percent"))
	}

	c.unconmmited[ArmUpPercentDP] = tuyable.NewDataPoint(ArmUpPercentDP, tuyable.DPTypeValue, armUpPercent)
	c.unconmmited[ArmDownPercentDP] = tuyable.NewDataPoint(ArmDownPercentDP, tuyable.DPTypeValue, armDownPercent)
}
