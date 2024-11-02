package fingerbot

type Mode uint32

const (
	ModeClick     Mode = 0
	ModelongPress Mode = 1
)

func (m Mode) String() string {
	switch m {
	case ModeClick:
		return "click"
	case ModelongPress:
		return "long_press"
	default:
		return "unknown"
	}
}

func (m Mode) Valid() bool {
	return m >= ModeClick && m <= ModelongPress
}

type ControlBack uint32

const (
	ControlBackUp   ControlBack = 0
	ControlBackDown ControlBack = 1
)

func (c ControlBack) String() string {
	switch c {
	case ControlBackUp:
		return "up"
	case ControlBackDown:
		return "down"
	default:
		return "unknown"
	}
}

func (c ControlBack) Valid() bool {
	return c >= ControlBackUp && c <= ControlBackDown
}

type ChargeStatus uint32

const (
	ChargeStatusNone       ChargeStatus = 0
	ChargeStatusCharging   ChargeStatus = 1
	ChargeStatusChargeDone ChargeStatus = 2
)

func (s ChargeStatus) String() string {
	switch s {
	case ChargeStatusNone:
		return "none"
	case ChargeStatusCharging:
		return "charging"
	case ChargeStatusChargeDone:
		return "charge_done"
	default:
		return "unknown"
	}
}

func (s ChargeStatus) Valid() bool {
	return s >= ChargeStatusNone && s <= ChargeStatusChargeDone
}
