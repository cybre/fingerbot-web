package tuyable

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"fmt"
	"log/slog"

	"github.com/cybre/fingerbot-web/internal/utils"
	"tinygo.org/x/bluetooth"
)

const (
	ServiceUUID    = "0000a201-0000-1000-8000-00805f9b34fb"
	ManufacturerID = 0x07D0
)

var adapter = bluetooth.DefaultAdapter

type DiscoveredDevice struct {
	LocalName       string
	Address         bluetooth.Address
	IsBound         bool
	ProtocolVersion byte
	RawUUID         []byte
	UUID            bluetooth.UUID
}

type DeviceManager struct {
	devices map[string]*Device
}

func NewDeviceManager() *DeviceManager {
	return &DeviceManager{}
}

func (dm *DeviceManager) Scan(ctx context.Context, output chan<- DiscoveredDevice) error {
	if err := adapter.Enable(); err != nil {
		return fmt.Errorf("error enabling bluetooth adapter: %w", err)
	}

	scanned := map[string]struct{}{}
	if err := adapter.Scan(func(a *bluetooth.Adapter, sr bluetooth.ScanResult) {
		select {
		case <-ctx.Done():
			if err := a.StopScan(); err != nil {
				slog.Error("error stopping scan", slog.Any("error", err))
				return
			}
		default:
		}
		if _, ok := scanned[sr.Address.String()]; ok {
			return
		}

		service, ok := utils.Find(sr.ServiceData(), func(element bluetooth.ServiceDataElement) bool {
			return element.UUID.String() == ServiceUUID
		})
		if !ok {
			return
		}

		if len(service.Data) < 1 {
			return
		}
		rawProductId := service.Data[1:]

		manufacturerData, ok := utils.Find(sr.ManufacturerData(), func(element bluetooth.ManufacturerDataElement) bool {
			return element.CompanyID == ManufacturerID
		})
		if !ok || len(manufacturerData.Data) <= 6 {
			return
		}

		rawUUID := manufacturerData.Data[6:]
		key := md5.Sum(rawProductId)
		block, err := aes.NewCipher(key[:])
		if err != nil {
			slog.Error("error creating cipher", slog.Any("error", err))
			return
		}
		mode := cipher.NewCBCDecrypter(block, key[:])
		decrypted := make([]byte, len(rawUUID))
		mode.CryptBlocks(decrypted, rawUUID)

		output <- DiscoveredDevice{
			LocalName:       sr.LocalName(),
			Address:         sr.Address,
			IsBound:         (manufacturerData.Data[0] & 0x80) != 0,
			ProtocolVersion: manufacturerData.Data[1],
			RawUUID:         decrypted,
			UUID:            bluetooth.NewUUID([16]byte(decrypted)),
		}

		scanned[sr.Address.String()] = struct{}{}
	}); err != nil {
		return fmt.Errorf("error scanning: %w", err)
	}

	return nil
}
