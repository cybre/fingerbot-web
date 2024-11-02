package tuyable

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/cybre/fingerbot-web/internal/logging"
	"github.com/cybre/fingerbot-web/internal/tuyable/packet"
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
)

const (
	// BLEConnectTimeout is the timeout for connecting to a BLE device
	BLEConnectTimeout = 25 * time.Second
	// ResponseWaitTimeout is the timeout for waiting for a response from the device
	ResponseWaitTimeout = 15 * time.Second

	// GattMTU is the maximum size of a GATT packet
	// https://developer.tuya.com/en/docs/iot-device-dev/tuya-ble-sdk-user-guide?id=K9h5zc4e5djd9#title-6-MTU
	GattMTU = 20

	// https://developer.tuya.com/en/docs/iot-device-dev/tuya-ble-sdk-user-guide?id=K9h5zc4e5djd9#title-5-The%20concepts%20of%20tuya%20ble%20service
	CharacteristicNotifyUUID = "2b10"
	CharacteristicWriteUUID  = "2b11"
)

// Device represents a Tuya BLE device
type Device struct {
	device            ble.Device
	client            ble.Client
	address           ble.Addr
	charWrite         *ble.Characteristic
	charNotify        *ble.Characteristic
	localKey          []byte
	loginKey          []byte
	sessionKey        []byte
	authKey           []byte
	uuid              string
	deviceID          string
	seqNum            uint32
	seqNumMutex       sync.Mutex
	sendMutex         sync.Mutex
	notificationMutex sync.Mutex
	responseCh        map[uint32]chan []byte
	responseMutex     sync.Mutex
	isPaired          bool
	isConnected       bool
	protocolVersion   byte
	flags             byte
	isBound           bool
	datapoints        map[byte]DataPoint
	assembler         *packet.Assembler
	logger            *slog.Logger
}

// NewDevice creates a new Device instance
func NewDevice(address, uuid, deviceID, localKey string, logger *slog.Logger) (*Device, error) {
	d, err := linux.NewDevice()
	if err != nil {
		return nil, fmt.Errorf("error creating BLE device: %w", err)
	}
	ble.SetDefaultDevice(d)

	localKeyBytes := []byte(localKey)
	if len(localKeyBytes) < 6 {
		return nil, fmt.Errorf("localKey must be at least 6 bytes")
	}

	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	loginKey := md5.Sum(localKeyBytes[:6]) // Use first 6 bytes for loginKey
	return &Device{
		device:          d,
		address:         ble.NewAddr(address),
		uuid:            uuid,
		deviceID:        deviceID,
		localKey:        localKeyBytes[:6],
		loginKey:        loginKey[:],
		seqNum:          1,
		responseCh:      make(map[uint32]chan []byte),
		protocolVersion: 3, // Default protocol version
		datapoints:      make(map[byte]DataPoint),
		logger:          logger.With("component", "Device"),
		assembler:       packet.NewAssemmbler(logger.With("component", "Assembler")),
	}, nil
}

// Connect connects to the Tuya BLE device
func (d *Device) Connect(ctx context.Context) error {
	d.logger.Info("Connecting to device...")
	client, err := ble.Dial(ble.WithSigHandler(context.WithTimeout(ctx, BLEConnectTimeout)), d.address)
	if err != nil {
		return fmt.Errorf("error connecting to device: %w", err)
	}
	d.client = client
	d.isConnected = true

	d.logger.Info("Discovering profile...")
	profile, err := d.client.DiscoverProfile(true)
	if err != nil {
		return fmt.Errorf("error discovering profile: %w", err)
	}

	for _, service := range profile.Services {
		for _, char := range service.Characteristics {
			switch char.UUID.String() {
			case CharacteristicWriteUUID:
				d.charWrite = char
			case CharacteristicNotifyUUID:
				d.charNotify = char
			}
		}
	}

	if d.charWrite == nil || d.charNotify == nil {
		return fmt.Errorf("required characteristics not found")
	}

	d.logger.Info("Subscribing to notifications...")
	if err = d.client.Subscribe(d.charNotify, false, d.handleNotification); err != nil {
		return fmt.Errorf("error subscribing to notifications: %w", err)
	}
	d.startPacketProcessing()

	return nil
}

// Disconnect disconnects from the Tuya BLE device
func (d *Device) Disconnect() error {
	if d.client != nil {
		d.isConnected = false
		d.logger.Info("Disconnecting from device...")
		if err := d.client.ClearSubscriptions(); err != nil {
			return fmt.Errorf("error clearing subscriptions: %w", err)
		}
		if err := d.client.CancelConnection(); err != nil {
			return fmt.Errorf("error cancelling connection: %w", err)
		}
	}

	d.assembler.Stop()

	return nil
}

// Pair initiates the pairing process with the device
func (d *Device) Pair() error {
	if d.isPaired {
		return fmt.Errorf("device is already paired")
	}
	if !d.isConnected {
		return fmt.Errorf("device is not connected")
	}

	d.logger.Info("Starting pairing process...")

	// Send device info request
	// This is required to get the protocol version and to start the session
	resp, err := d.sendPacket(packet.FUN_SENDER_DEVICE_INFO, []byte{})
	if err != nil {
		return fmt.Errorf("error sending device info request: %w", err)
	}
	if err = d.processDeviceInfoResponse(resp); err != nil {
		return fmt.Errorf("error processing device info response: %w", err)
	}

	// Send pairing command
	resp, err = d.sendPacket(packet.FUN_SENDER_PAIR, d.buildPairingRequest())
	if err != nil {
		return fmt.Errorf("error sending pairing command: %w", err)
	}
	if err = d.processPairingResponse(resp); err != nil {
		return fmt.Errorf("error processing pairing response: %w", err)
	}

	d.isPaired = true

	d.logger.Info("Pairing successful, syncing datapoints...")
	return d.Update()
}

// Update updates the device information
func (d *Device) Update() error {
	if _, err := d.sendPacket(packet.FUN_SENDER_DEVICE_STATUS, []byte{}); err != nil {
		return fmt.Errorf("error sending device status request: %w", err)
	}

	time.Sleep(1 * time.Second)

	return nil
}

// GetDatapoint returns the requested data point
func (d *Device) GetDatapoint(id byte) (DataPoint, bool) {
	dp, exists := d.datapoints[id]
	return dp, exists
}

// SetDatapointValue sets the value of a data point
func (d *Device) SetDatapoint(dp DataPoint) error {
	if err := dp.Validate(); err != nil {
		return fmt.Errorf("invalid data point: %w", err)
	}

	payload, err := dp.Payload()
	if err != nil {
		return err
	}

	resp, err := d.sendPacket(packet.FUN_SENDER_DPS, payload)
	if err != nil {
		return err
	}

	return d.checkResponse(resp)
}

// SetDatapoints sets multiple data points at once
func (d *Device) SetDatapoints(datapoints []DataPoint) error {
	if len(datapoints) == 0 {
		return nil
	}

	payload := make([]byte, 0)
	for _, dp := range datapoints {
		if err := dp.Validate(); err != nil {
			return fmt.Errorf("invalid data point: %w", err)
		}

		dpPayload, err := dp.Payload()
		if err != nil {
			return err
		}

		payload = append(payload, dpPayload...)
	}

	resp, err := d.sendPacket(packet.FUN_SENDER_DPS, payload)
	if err != nil {
		return err
	}

	if err := d.checkResponse(resp); err != nil {
		return err
	}

	return nil
}

// handleNotification processes incoming notifications from the device
func (d *Device) handleNotification(data []byte) {
	d.notificationMutex.Lock()
	defer d.notificationMutex.Unlock()

	d.assembler.Incoming() <- data
}

// buildPairingRequest constructs the pairing request payload
func (d *Device) buildPairingRequest() []byte {
	payload := make([]byte, 0, 44)
	payload = append(payload, []byte(d.uuid)...)
	payload = append(payload, d.localKey...)
	payload = append(payload, []byte(d.deviceID)...)

	// Pad with zeros to reach 44 bytes
	for len(payload) < 44 {
		payload = append(payload, 0x00)
	}

	return payload
}

// getSeqNum returns the next sequence number
func (d *Device) getSeqNum() uint32 {
	d.seqNumMutex.Lock()
	defer d.seqNumMutex.Unlock()
	seqNum := d.seqNum
	d.seqNum++
	return seqNum
}

func (d *Device) startPacketProcessing() {
	go func() {
		for assembledData := range d.assembler.Assembled() {
			pkt, err := d.decryptPacket(assembledData)
			if err != nil {
				d.logger.Error("Failed to decrypt and parse packet", slog.Any("error", err))
				continue
			}

			d.logger.Debug(
				"Parsed packet",
				slog.Any("packet", pkt),
			)

			d.handleParsedPacket(pkt)
		}
	}()
}

// checkResponse checks whether the response indicates success
func (d *Device) checkResponse(resp []byte) error {
	if len(resp) < 1 {
		return fmt.Errorf("response too short")
	}

	result := resp[0]
	if result != 0 {
		return fmt.Errorf("command failed with error code: %d", result)
	}

	return nil
}

// sendPacket constructs and sends a Tuya BLE packet to the device
func (d *Device) sendPacket(commandType packet.CommandType, payload []byte) ([]byte, error) {
	seqNum := d.getSeqNum()
	respCh := make(chan []byte)
	d.responseMutex.Lock()
	d.responseCh[seqNum] = respCh
	d.responseMutex.Unlock()

	securityFlag := packet.SecurityFlagSession
	if commandType == packet.FUN_SENDER_DEVICE_INFO {
		securityFlag = packet.SecurityFlagLogin
	}

	pkt := packet.NewPacket(seqNum, 0, commandType, payload, securityFlag)
	packetData, err := pkt.BuildAndEncryptPacket(d.getKey(securityFlag))
	if err != nil {
		return nil, err
	}

	d.logger.Debug(
		"Sending packet",
		slog.Any("packet", pkt),
	)

	packets := d.splitPackets(packetData)
	if err := d.sendPackets(packets); err != nil {
		return nil, err
	}

	resp, err := d.waitForResponse(seqNum)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// sendResponse sends a response packet to the device
func (d *Device) sendResponse(responseTo uint32, commandType packet.CommandType, payload []byte) {
	pkt := packet.NewPacket(d.getSeqNum(), responseTo, commandType, payload, packet.SecurityFlagSession)

	d.logger.Debug(
		"Sending response",
		slog.Any("packet", pkt),
	)

	packetData, err := pkt.BuildAndEncryptPacket(d.sessionKey)
	if err != nil {
		d.logger.Error("Failed to build and encrypt packet", slog.Any("error", err))
		return
	}

	packets := d.splitPackets(packetData)
	if err := d.sendPackets(packets); err != nil {
		d.logger.Error("Failed to send response", logging.ErrAttr(err))
	}
}

// sendPackets sends a list of packets over BLE
func (d *Device) sendPackets(packets [][]byte) error {
	d.sendMutex.Lock()
	defer d.sendMutex.Unlock()

	for i, packet := range packets {
		d.logger.Debug("Sending packet part", slog.Int("packet_num", i), slog.Int("total_packets", len(packets)))
		if err := d.client.WriteCharacteristic(d.charWrite, packet, true); err != nil {
			return fmt.Errorf("error writing packet %d: %w", i, err)
		}
	}

	return nil
}

// splitPackets splits the packet data into GATT MTU-sized packets
func (d *Device) splitPackets(packetData []byte) [][]byte {
	var packets [][]byte
	packetNum := 0
	pos := 0
	length := len(packetData)

	for pos < length {
		packet := make([]byte, 0)
		packet = append(packet, packInt(packetNum)...)

		if packetNum == 0 {
			totalLengthBytes := packInt(length)
			packet = append(packet, totalLengthBytes...)
			protocolVersion := d.protocolVersion << 4
			packet = append(packet, protocolVersion)
		}

		remaining := GattMTU - len(packet)
		if remaining <= 0 {
			break
		}

		end := pos + remaining
		if end > length {
			end = length
		}
		dataPart := packetData[pos:end]
		packet = append(packet, dataPart...)
		packets = append(packets, packet)
		pos += len(dataPart)
		packetNum++
	}

	return packets
}

// handleParsedPacket handles the parsed packet from the device
func (d *Device) handleParsedPacket(pkt *packet.Packet) {
	if pkt.ResponseTo != 0 {
		d.responseMutex.Lock()
		respCh, exists := d.responseCh[pkt.ResponseTo]
		if exists {
			respCh <- pkt.Payload
			delete(d.responseCh, pkt.ResponseTo)
		}
		d.responseMutex.Unlock()
		return
	}

	switch pkt.CommandType {
	case packet.FUN_RECEIVE_TIME1_REQ:
		d.handleTime1Request(pkt.SeqNum)
	case packet.FUN_RECEIVE_DP:
		if err := d.parseDatapoints(pkt.Payload); err != nil {
			d.logger.Error("Failed to parse datapoints", logging.ErrAttr(err))
		}
		go d.sendResponse(pkt.SeqNum, packet.FUN_RECEIVE_DP, []byte{})
	}
}

// parseDatapoints parses the datapoints contained in the payload
func (d *Device) parseDatapoints(payload []byte) error {
	pos := 0
	for len(payload)-pos >= 4 {
		id := payload[pos]
		pos += 1

		typeByte := payload[pos]
		if typeByte > byte(DPTypeBitmap) {
			return fmt.Errorf("Invalid datapoint type: %d", typeByte)
		}
		_type := DPType(typeByte)
		pos += 1

		dataLen := payload[pos]
		pos += 1

		nextPos := pos + int(dataLen)
		if nextPos > len(payload) {
			return fmt.Errorf("invalid payload length: %d", len(payload))
		}

		datapoint, err := ParseDataPoint(id, _type, payload[pos:nextPos])
		if err != nil {
			return fmt.Errorf("error creating datapoint: %w", err)
		}

		d.datapoints[id] = datapoint
		d.logger.Debug(
			"Received datapoint",
			slog.Any("datapoint", datapoint),
		)

		pos = nextPos
	}

	return nil
}

// decryptPacket decrypts the packet data
func (d *Device) decryptPacket(data []byte) (*packet.Packet, error) {
	securityFlag := packet.SecurityFlag(data[0])
	pkt, err := packet.DecryptAndParsePacket(data, d.getKey(securityFlag))
	if err != nil {
		return nil, err
	}

	return pkt, nil
}

// waitForResponse waits for a response with the specified sequence number
func (d *Device) waitForResponse(seqNum uint32) ([]byte, error) {
	d.responseMutex.Lock()
	respCh, exists := d.responseCh[seqNum]
	d.responseMutex.Unlock()
	if !exists {
		return nil, fmt.Errorf("no pending response for seqNum %d", seqNum)
	}

	select {
	case resp := <-respCh:
		return resp, nil
	case <-time.After(ResponseWaitTimeout):
		d.responseMutex.Lock()
		delete(d.responseCh, seqNum)
		d.responseMutex.Unlock()
		return nil, fmt.Errorf("timeout waiting for response to seqNum %d", seqNum)
	}
}

// processDeviceInfoResponse processes the device info response
func (d *Device) processDeviceInfoResponse(data []byte) error {
	if len(data) < 46 {
		return fmt.Errorf("device info response too short")
	}

	d.protocolVersion = data[2]
	d.flags = data[4]
	d.isBound = data[5] != 0

	srand := data[6:12]
	sessionKey := md5.Sum(append(d.localKey, srand...))
	d.sessionKey = sessionKey[:]

	d.authKey = data[14:46]

	return nil
}

// processPairingResponse processes the pairing response
func (d *Device) processPairingResponse(data []byte) error {
	if len(data) < 1 {
		return fmt.Errorf("pairing response too short")
	}
	result := data[0]
	if result != 0 && result != 2 {
		return fmt.Errorf("pairing failed with error code: %d", result)
	}
	return nil
}

// handleTime1Request handles the Time1 request from the device
func (d *Device) handleTime1Request(seqNum uint32) {
	d.logger.Debug("Handling Time1 request...")

	timestamp := time.Now().UnixMilli()

	_, offsetSeconds := time.Now().Zone()
	timezone := int16(offsetSeconds / 36)

	timestampStr := fmt.Sprintf("%d", timestamp)
	data := []byte(timestampStr)

	timezoneBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(timezoneBytes, uint16(timezone))
	data = append(data, timezoneBytes...)

	go d.sendResponse(seqNum, packet.FUN_RECEIVE_TIME1_REQ, data)
}

// getKey returns the encryption key for the specified security flag
func (d *Device) getKey(securityFlag packet.SecurityFlag) []byte {
	switch securityFlag {
	case packet.SecurityFlagAuth:
		return d.authKey
	case packet.SecurityFlagLogin:
		return d.loginKey
	case packet.SecurityFlagSession:
		return d.sessionKey
	default:
		panic("invalid security flag")
	}
}
