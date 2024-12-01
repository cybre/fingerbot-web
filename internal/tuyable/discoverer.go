package tuyable

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"fmt"
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
	deviceCache map[string]DiscoveredDevice
}

func NewDiscoverer() *Discoverer {
	return &Discoverer{
		deviceCache: map[string]DiscoveredDevice{},
	}
}

func (dm *Discoverer) StopDiscovery() error {
	return bluetooth.DefaultAdapter.StopScan()
}

func (dm *Discoverer) Discover(ctx context.Context, output chan<- DiscoveredDevice) error {
	bluetooth.DefaultAdapter.StopScan()

	if err := bluetooth.DefaultAdapter.Scan(func(a *bluetooth.Adapter, sr bluetooth.ScanResult) {
		select {
		case <-ctx.Done():
			if err := a.StopScan(); err != nil {
				slog.Error("error stopping scan", slog.Any("error", err))
				return
			}
		default:
		}

		service, ok := utils.Find(sr.ServiceData(), func(element bluetooth.ServiceDataElement) bool {
			return element.UUID == ServiceUUID
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

		device := DiscoveredDevice{
			LocalName:       sr.LocalName(),
			Address:         sr.Address.String(),
			IsBound:         (manufacturerData.Data[0] & 0x80) != 0,
			ProtocolVersion: manufacturerData.Data[1],
			UUID:            decrypted,
			RSSI:            sr.RSSI,
		}

		output <- device

		dm.deviceCache[sr.Address.String()] = device
	}); err != nil {
		return fmt.Errorf("error scanning: %w", err)
	}

	return nil
}

func (dm *Discoverer) GetDevice(address string) (DiscoveredDevice, bool) {
	device, ok := dm.deviceCache[address]
	return device, ok
}
