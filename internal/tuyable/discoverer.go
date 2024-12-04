package tuyable

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/cybre/fingerbot-web/internal/utils"
	"github.com/go-ble/ble"
	"github.com/google/uuid"
)

const (
	ManufacturerID = 0x07D0
)

var DiscoverServiceUUID = ble.UUID16(0xa201)

type DiscoveredDevice struct {
	LocalName       string
	Address         string
	IsBound         bool
	ProtocolVersion byte
	UUID            []byte
	RSSI            int
}

func (d DiscoveredDevice) ID() string {
	return strings.ReplaceAll(d.Address, ":", "")
}

type Discoverer struct {
	deviceCache     map[string]*DiscoveredDevice
	logger          *slog.Logger
	listeners       map[string]chan *DiscoveredDevice
	discoveryMutex  sync.Mutex
	cancelDiscovery context.CancelFunc
}

func NewDiscoverer(logger *slog.Logger) *Discoverer {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &Discoverer{
		deviceCache: map[string]*DiscoveredDevice{},
		logger:      logger,
		listeners:   map[string]chan *DiscoveredDevice{},
	}
}

func (d *Discoverer) DiscoverDevice(ctx context.Context, address string) (*DiscoveredDevice, error) {
	d.logger.Debug("discovering device", slog.String("address", address))

	if device, ok := d.deviceCache[address]; ok {
		d.logger.Debug("device found in cache", slog.Any("device", device))
		return device, nil
	}

	devices, unsubscribe := d.Discover()
	defer unsubscribe()

	for device := range devices {
		if device.Address == address {
			return device, nil
		}
	}

	return nil, errors.New("device not found")
}

func (d *Discoverer) Discover() (<-chan *DiscoveredDevice, func()) {
	d.discoveryMutex.Lock()
	defer d.discoveryMutex.Unlock()

	output := make(chan *DiscoveredDevice)
	listenerID := uuid.NewString()
	d.listeners[listenerID] = output
	d.logger.Debug("subscribed to discovery", slog.String("listener_id", listenerID))

	if d.cancelDiscovery == nil {
		d.logger.Debug("starting discovery")
		go func() {
			ctx, cancel := context.WithCancel(context.Background())
			d.cancelDiscovery = cancel
			defer func() {
				cancel()
				d.cancelDiscovery = nil
			}()

			if err := ble.Scan(ctx, true, func(a ble.Advertisement) {
				if len(a.ManufacturerData()) < 2 {
					return
				}

				companyID := uint16(a.ManufacturerData()[1])<<8 | uint16(a.ManufacturerData()[0])
				if companyID != ManufacturerID {
					return
				}

				manufacturerData := a.ManufacturerData()[2:]
				if len(manufacturerData) <= 6 {
					return
				}

				service, ok := utils.Find(a.ServiceData(), func(element ble.ServiceData) bool {
					return element.UUID.Equal(DiscoverServiceUUID)
				})
				if !ok {
					return
				}

				if len(service.Data) < 1 {
					return
				}
				rawProductId := service.Data[1:]

				rawUUID := manufacturerData[6:]
				key := md5.Sum(rawProductId)
				block, err := aes.NewCipher(key[:])
				if err != nil {
					d.logger.Error("error creating cipher", slog.Any("error", err))
					return
				}
				mode := cipher.NewCBCDecrypter(block, key[:])
				decrypted := make([]byte, len(rawUUID))
				mode.CryptBlocks(decrypted, rawUUID)

				device := &DiscoveredDevice{
					LocalName:       a.LocalName(),
					Address:         strings.ToUpper(a.Addr().String()),
					IsBound:         (manufacturerData[0] & 0x80) != 0,
					ProtocolVersion: manufacturerData[1],
					UUID:            decrypted,
					RSSI:            a.RSSI(),
				}

				d.deviceCache[strings.ToUpper(a.Addr().String())] = device

				for _, listener := range d.listeners {
					listener <- device
				}

				d.logger.Debug("device discovered", slog.Any("device", device))
			}, nil); err != nil {
				if !errors.Is(err, context.Canceled) {
					d.logger.Error("error scanning", slog.Any("error", err))
				}
			}
		}()
	}

	return output, func() {
		d.logger.Debug("unsubscribing from discovery", slog.String("listener_id", listenerID))
		close(output)
		delete(d.listeners, listenerID)
		if len(d.listeners) == 0 {
			d.logger.Debug("stopping discovery")
			d.cancelDiscovery()
			d.cancelDiscovery = nil
		}
	}
}
