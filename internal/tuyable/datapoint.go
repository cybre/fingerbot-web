package tuyable

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// DPType represents a type of a data point in the Tuya BLE protocol
type DPType byte

const (
	DPTypeRaw    DPType = 0x00
	DPTypeBool   DPType = 0x01
	DPTypeValue  DPType = 0x02
	DPTypeString DPType = 0x03
	DPTypeEnum   DPType = 0x04
	DPTypeBitmap DPType = 0x05
)

func (t DPType) String() string {
	switch t {
	case DPTypeRaw:
		return "raw"
	case DPTypeBool:
		return "bool"
	case DPTypeValue:
		return "value"
	case DPTypeString:
		return "string"
	case DPTypeEnum:
		return "enum"
	case DPTypeBitmap:
		return "bitmap"
	default:
		return "unknown"
	}
}

func (t DPType) Valid() bool {
	return t >= DPTypeRaw && t <= DPTypeBitmap
}

// DataPoint represents a device data point
type DataPoint struct {
	ID    byte
	Type  DPType
	Value interface{}
}

// NewDataPoint creates a new DataPoint instance
func NewDataPoint(id byte, t DPType, v interface{}) DataPoint {
	return DataPoint{
		ID:    id,
		Type:  t,
		Value: v,
	}
}

func ParseDataPoint(id byte, t DPType, rawValue []byte) (DataPoint, error) {
	var value interface{}
	var err error
	switch t {
	case DPTypeRaw, DPTypeBitmap:
		value = rawValue
	case DPTypeBool:
		value, err = bytesToBool(rawValue)
		if err != nil {
			return DataPoint{}, fmt.Errorf("error parsing DPTypeBool: %w", err)
		}
	case DPTypeValue:
		value, err = bytesToInt32(rawValue)
		if err != nil {
			return DataPoint{}, fmt.Errorf("error parsing DPTypeValue: %w", err)
		}
	case DPTypeEnum:
		value, err = bytesToUint32(rawValue)
		if err != nil {
			return DataPoint{}, fmt.Errorf("error parsing DPTypeEnum: %w", err)
		}
	case DPTypeString:
		value = string(rawValue)
	default:
		return DataPoint{}, fmt.Errorf("unknown data point type: %v", t)
	}

	return NewDataPoint(id, t, value), nil
}

// Validate validates the DataPoint
func (d *DataPoint) Validate() error {
	if !d.Type.Valid() {
		return fmt.Errorf("invalid type: %v", d.Type)
	}
	if d.ID == 0 {
		return fmt.Errorf("invalid ID: %d", d.ID)
	}
	if d.Value == nil {
		return fmt.Errorf("invalid value: %v", d.Value)
	}

	return nil
}

// Payload constructs a FUN_SENDER_DP command payload out of the DataPoint
func (d *DataPoint) Payload() ([]byte, error) {
	var data []byte
	switch d.Type {
	case DPTypeRaw, DPTypeBitmap:
		rawValue, ok := d.Value.([]byte)
		if !ok {
			return nil, fmt.Errorf("expected []byte for DPTypeRaw or DPTypeBitmap")
		}
		data = rawValue
	case DPTypeBool:
		boolValue, ok := d.Value.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool for DPTypeBool")
		}
		var byteValue byte
		if boolValue {
			byteValue = 1
		} else {
			byteValue = 0
		}
		data = []byte{byteValue}
	case DPTypeValue:
		intValue, ok := d.Value.(int32)
		if !ok {
			return nil, fmt.Errorf("expected int32 for DPTypeValue")
		}
		buf := new(bytes.Buffer)
		if err := binary.Write(buf, binary.BigEndian, intValue); err != nil {
			return nil, fmt.Errorf("error writing int32: %w", err)
		}
		data = buf.Bytes()
	case DPTypeEnum:
		uintValue, ok := d.Value.(uint32)
		if !ok {
			return nil, fmt.Errorf("expected uint32 for DPTypeEnum")
		}
		var buf []byte
		switch {
		case uintValue > 0xFFFF:
			buf = make([]byte, 4)
			binary.BigEndian.PutUint32(buf, uintValue)
		case uintValue > 0xFF:
			buf = make([]byte, 2)
			binary.BigEndian.PutUint16(buf, uint16(uintValue))
		default:
			buf = []byte{byte(uintValue)}
		}
		data = buf
	case DPTypeString:
		strValue, ok := d.Value.(string)
		if !ok {
			return nil, fmt.Errorf("expected string for DPTypeString")
		}
		data = []byte(strValue)
	default:
		return nil, fmt.Errorf("unknown data point type: %v", d.Type)
	}

	return append([]byte{d.ID, byte(d.Type), byte(len(data))}, data...), nil
}
