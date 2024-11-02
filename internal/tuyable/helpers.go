package tuyable

import (
	"encoding/binary"
	"fmt"
)

// packInt packs an integer using variable-length encoding
func packInt(value int) []byte {
	var result []byte
	for {
		currByte := byte(value & 0x7F)
		value >>= 7
		if value != 0 {
			currByte |= 0x80
		}
		result = append(result, currByte)
		if value == 0 {
			break
		}
	}
	return result
}

// bytesToBool converts a byte slice to a boolean
func bytesToBool(data []byte) (bool, error) {
	if len(data) != 1 {
		return false, fmt.Errorf("invalid data length for bool: expected 1, got %d", len(data))
	}
	return data[0] != 0, nil
}

// bytesToInt32 converts a byte slice to int32 assuming big-endian order
func bytesToInt32(data []byte) (int32, error) {
	if len(data) != 4 {
		return 0, fmt.Errorf("invalid data length for int32: expected 4, got %d", len(data))
	}
	return int32(binary.BigEndian.Uint32(data)), nil
}

// bytesToUint32 converts a byte slice to uint32 assuming big-endian order
func bytesToUint32(data []byte) (uint32, error) {
	if len(data) != 1 && len(data) != 2 && len(data) != 4 {
		return 0, fmt.Errorf("invalid data length for uint32: expected 1, 2, or 4, got %d", len(data))
	}

	switch len(data) {
	case 1:
		return uint32(data[0]), nil
	case 2:
		return uint32(binary.BigEndian.Uint16(data)), nil
	case 4:
		return binary.BigEndian.Uint32(data), nil
	default:
		return 0, fmt.Errorf("unsupported data length for uint32")
	}
}
