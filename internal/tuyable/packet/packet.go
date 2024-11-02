package packet

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

// CommandType represents a command in the Tuya BLE protocol
type CommandType uint16

const (
	FUN_SENDER_DEVICE_INFO   CommandType = 0x0000
	FUN_SENDER_PAIR          CommandType = 0x0001
	FUN_SENDER_DPS           CommandType = 0x0002
	FUN_SENDER_DEVICE_STATUS CommandType = 0x0003

	FUN_RECEIVE_DP        CommandType = 0x8001
	FUN_RECEIVE_TIME1_REQ CommandType = 0x8011
	FUN_RECEIVE_TIME2_REQ CommandType = 0x8012
)

// SecurityFlag represents a security flag in the Tuya BLE protocol
type SecurityFlag byte

const (
	SecurityFlagAuth    SecurityFlag = 0x01
	SecurityFlagLogin   SecurityFlag = 0x04
	SecurityFlagSession SecurityFlag = 0x05
)

// Packet represents a Tuya BLE packet
type Packet struct {
	SeqNum       uint32
	ResponseTo   uint32
	CommandType  CommandType
	Payload      []byte
	SecurityFlag SecurityFlag
	IV           []byte
	Data         []byte // Encrypted data
}

// NewPacket creates a new Packet instance
func NewPacket(seqNum, responseTo uint32, commandType CommandType, payload []byte, securityFlag SecurityFlag) *Packet {
	return &Packet{
		SeqNum:       seqNum,
		ResponseTo:   responseTo,
		CommandType:  commandType,
		Payload:      payload,
		SecurityFlag: securityFlag,
	}
}

// BuildAndEncryptPacket constructs and encrypts a packet
func (p *Packet) BuildAndEncryptPacket(key []byte) ([]byte, error) {
	raw := new(bytes.Buffer)

	if err := binary.Write(raw, binary.BigEndian, p.SeqNum); err != nil {
		return nil, fmt.Errorf("error writing seqNum: %w", err)
	}
	if err := binary.Write(raw, binary.BigEndian, p.ResponseTo); err != nil {
		return nil, fmt.Errorf("error writing responseTo: %w", err)
	}
	if err := binary.Write(raw, binary.BigEndian, p.CommandType); err != nil {
		return nil, fmt.Errorf("error writing commandType: %w", err)
	}
	if err := binary.Write(raw, binary.BigEndian, uint16(len(p.Payload))); err != nil {
		return nil, fmt.Errorf("error writing payload length: %w", err)
	}
	if _, err := raw.Write(p.Payload); err != nil {
		return nil, fmt.Errorf("error writing payload: %w", err)
	}
	crc := calcCRC16(raw.Bytes())
	if err := binary.Write(raw, binary.BigEndian, crc); err != nil {
		return nil, fmt.Errorf("error writing CRC: %w", err)
	}
	for raw.Len()%16 != 0 {
		if err := raw.WriteByte(0x00); err != nil {
			return nil, fmt.Errorf("error padding packet: %w", err)
		}
	}

	iv := make([]byte, 16)
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("error generating IV: %w", err)
	}
	p.IV = iv

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("error creating cipher: %w", err)
	}
	mode := cipher.NewCBCEncrypter(block, iv)
	encrypted := make([]byte, raw.Len())
	mode.CryptBlocks(encrypted, raw.Bytes())
	p.Data = append([]byte{byte(p.SecurityFlag)}, iv...)
	p.Data = append(p.Data, encrypted...)

	return p.Data, nil
}

// DecryptAndParsePacket decrypts and parses a packet from the given data
func DecryptAndParsePacket(data []byte, key []byte) (*Packet, error) {
	p := &Packet{}

	if len(data) < 17 {
		return nil, fmt.Errorf("invalid packet length: %d", len(data))
	}

	p.SecurityFlag = SecurityFlag(data[0])
	p.IV = data[1:17]
	encrypted := data[17:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("error creating cipher: %w", err)
	}

	if len(encrypted)%16 != 0 {
		return nil, fmt.Errorf("encrypted data is not a multiple of block size")
	}

	mode := cipher.NewCBCDecrypter(block, p.IV)
	decrypted := make([]byte, len(encrypted))
	mode.CryptBlocks(decrypted, encrypted)
	decrypted = bytes.TrimRight(decrypted, "\x00")

	buf := bytes.NewBuffer(decrypted)
	if err := binary.Read(buf, binary.BigEndian, &p.SeqNum); err != nil {
		return nil, fmt.Errorf("error reading seqNum: %w", err)
	}
	if err := binary.Read(buf, binary.BigEndian, &p.ResponseTo); err != nil {
		return nil, fmt.Errorf("error reading responseTo: %w", err)
	}
	if err := binary.Read(buf, binary.BigEndian, &p.CommandType); err != nil {
		return nil, fmt.Errorf("error reading commandType: %w", err)
	}
	var payloadLen uint16
	if err := binary.Read(buf, binary.BigEndian, &payloadLen); err != nil {
		return nil, fmt.Errorf("error reading payload length: %w", err)
	}
	p.Payload = make([]byte, payloadLen)
	if _, err := buf.Read(p.Payload); err != nil {
		return nil, fmt.Errorf("error reading payload: %w", err)
	}
	var receivedCRC uint16
	if err := binary.Read(buf, binary.BigEndian, &receivedCRC); err != nil {
		return nil, fmt.Errorf("error reading CRC: %w", err)
	}
	calculatedCRC := calcCRC16(decrypted[:12+payloadLen])
	if receivedCRC != calculatedCRC {
		return nil, fmt.Errorf("CRC mismatch: received %d, calculated %d", receivedCRC, calculatedCRC)
	}

	return p, nil
}

// calcCRC16 calculates the CRC16 checksum
func calcCRC16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}

	return crc
}
