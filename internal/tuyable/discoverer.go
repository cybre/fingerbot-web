package tuyable

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/cybre/fingerbot-web/internal/utils"
	"tinygo.org/x/bluetooth"
)

const (
	ManufacturerID = 0x07D0
)

var ServiceUUID = bluetooth.New16BitUUID(0xa201)

type DiscoveredDevice struct {
	LocalName       string
	Address         string
	IsBound         bool
	ProtocolVersion byte
	UUID            []byte
	RSSI            int16
}

func (d DiscoveredDevice) ID() string {
	return strings.ReplaceAll(d.Address, ":", "")
}

type Discoverer struct {
	deviceCache map[string]*DiscoveredDevice
	logger      *slog.Logger
	disovery    chan *DiscoveredDevice
}

func NewDiscoverer(logger *slog.Logger) *Discoverer {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &Discoverer{
		deviceCache: map[string]*DiscoveredDevice{},
		logger:      logger,
	}
}

func (dm *Discoverer) DiscoverDevice(ctx context.Context, address string) (*DiscoveredDevice, error) {
	dm.logger.Info("discovering device", slog.String("address", address))

	if device, ok := dm.deviceCache[address]; ok {
		dm.logger.Info("device found in cache", slog.Any("device", device))
		return device, nil
	}

	for device := range dm.Discover(ctx) {
		if device.Address == address {
			return device, nil
		}
	}

	return nil, errors.New("device not found")
}

func (dm *Discoverer) StopDiscovery() {
	bluetooth.DefaultAdapter.StopScan()
}

func (dm *Discoverer) Discover(ctx context.Context) <-chan *DiscoveredDevice {
	dm.logger.Info("discovering devices")
	if dm.disovery != nil {
		dm.logger.Debug("discovery already in progress")
		return dm.disovery
	}

	dm.disovery = make(chan *DiscoveredDevice)

	go func() {
		dm.logger.Info("starting discovery")
		if err := bluetooth.DefaultAdapter.Scan(func(a *bluetooth.Adapter, sr bluetooth.ScanResult) {
			select {
			case <-ctx.Done():
				dm.logger.Info("context done, stopping scan")
				if err := a.StopScan(); err != nil {
					dm.logger.Error("error stopping scan", slog.Any("error", err))
					return
				}
			default:
			}

			device, err := dm.handleScanResult(sr)
			if err != nil {
				dm.logger.Debug("error handling scan result", slog.Any("error", err))
				return
			} else if device != nil {
				dm.disovery <- device
			}
		}); err != nil {
			dm.logger.Error("error scanning", slog.Any("error", err))
		}

		dm.logger.Info("discovery finished")
		close(dm.disovery)
		dm.disovery = nil
	}()

	return dm.disovery
}

func (dm *Discoverer) handleScanResult(sr bluetooth.ScanResult) (*DiscoveredDevice, error) {
	service, ok := utils.Find(sr.ServiceData(), func(element bluetooth.ServiceDataElement) bool {
		return element.UUID == ServiceUUID
	})
	if !ok {
		return nil, errors.New("service data not found")
	}

	if len(service.Data) < 1 {
		return nil, errors.New("service data too short")
	}
	rawProductId := service.Data[1:]

	manufacturerData, ok := utils.Find(sr.ManufacturerData(), func(element bluetooth.ManufacturerDataElement) bool {
		return element.CompanyID == ManufacturerID
	})
	if !ok || len(manufacturerData.Data) <= 6 {
		return nil, errors.New("manufacturer data not found")
	}

	rawUUID := manufacturerData.Data[6:]
	key := md5.Sum(rawProductId)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("error creating cipher: %w", err)
	}
	mode := cipher.NewCBCDecrypter(block, key[:])
	decrypted := make([]byte, len(rawUUID))
	mode.CryptBlocks(decrypted, rawUUID)

	device := &DiscoveredDevice{
		LocalName:       sr.LocalName(),
		Address:         sr.Address.String(),
		IsBound:         (manufacturerData.Data[0] & 0x80) != 0,
		ProtocolVersion: manufacturerData.Data[1],
		UUID:            decrypted,
		RSSI:            sr.RSSI,
	}

	dm.deviceCache[sr.Address.String()] = device

	return device, nil
}
