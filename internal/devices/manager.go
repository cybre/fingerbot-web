package devices

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/cybre/fingerbot-web/internal/tuyable"
	"github.com/cybre/fingerbot-web/internal/tuyable/fingerbot"
	"github.com/cybre/fingerbot-web/internal/utils"
)

type DeviceView struct {
	Name      string
	Address   string
	RSSI      int16
	Saved     bool
	Connected bool
}

func (d DeviceView) ID() string {
	return strings.ReplaceAll(d.Address, ":", "")
}

type Manager struct {
	repository      *Repository
	discoverer      *tuyable.Discoverer
	logger          *slog.Logger
	conectedDevices map[string]*fingerbot.Fingerbot
	connectingMutex sync.Mutex
}

func NewManager(repository *Repository, discoverer *tuyable.Discoverer, logger *slog.Logger) *Manager {
	return &Manager{
		repository:      repository,
		discoverer:      discoverer,
		logger:          logger,
		conectedDevices: map[string]*fingerbot.Fingerbot{},
	}
}

func (m *Manager) ConnectToSavedDevices(ctx context.Context) error {
	devices, err := m.repository.GetDevices(ctx)
	if err != nil {
		return fmt.Errorf("failed to get devices: %w", err)
	}

	for _, device := range devices {
		if err := m.connectDevice(ctx, device); err != nil {
			m.logger.Error("failed to connect to device", slog.Any("device", device), slog.Any("error", err))
		}
	}

	return nil
}

type DeviceConnection struct {
	Address  string `json:"address" form:"address"`
	Name     string `json:"name" form:"name"`
	DeviceID string `json:"deviceId" form:"deviceId"`
	LocalKey string `json:"localKey" form:"localKey"`
}

func (m *Manager) ConnectToSavedDevice(ctx context.Context, address string) (*DeviceView, error) {
	device, err := m.repository.GetDevice(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	if err := m.connectDevice(ctx, device); err != nil {
		return nil, err
	}

	return &DeviceView{
		Name:      device.Name,
		Address:   device.Address,
		RSSI:      0,
		Saved:     true,
		Connected: true,
	}, nil
}

func (m *Manager) Connect(ctx context.Context, conn DeviceConnection) (*DeviceView, error) {
	discoveredDevice, err := m.discoverer.DiscoverDevice(ctx, conn.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to discover device: %w", err)
	}

	device := &Device{
		DeviceID: conn.DeviceID,
		Address:  discoveredDevice.Address,
		Name:     conn.Name,
		LocalKey: conn.LocalKey,
		UUID:     string(discoveredDevice.UUID),
	}

	if err := m.repository.CreateDevice(ctx, device); err != nil {
		return nil, fmt.Errorf("failed to create device: %w", err)
	}

	if err := m.connectDevice(ctx, device); err != nil {
		return nil, err
	}

	return &DeviceView{
		Name:      device.Name,
		Address:   device.Address,
		RSSI:      discoveredDevice.RSSI,
		Saved:     true,
		Connected: true,
	}, nil
}

func (m *Manager) Discover(ctx context.Context, output chan<- DeviceView) error {
	m.connectingMutex.Lock()
	defer m.connectingMutex.Unlock()

	for tuyaDevice := range m.discoverer.Discover(ctx) {
		device := DeviceView{
			Name:      tuyaDevice.LocalName,
			Address:   tuyaDevice.Address,
			RSSI:      tuyaDevice.RSSI,
			Saved:     false,
			Connected: false,
		}

		saved, err := m.repository.GetDevice(ctx, tuyaDevice.Address)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}

			return fmt.Errorf("failed to get device: %w", err)
		}
		if saved != nil {
			device.Name = saved.Name
			device.Saved = true
			device.Connected = m.conectedDevices[saved.Address] != nil
		}

		output <- device
	}

	m.logger.Info("discovery complete, closing output channel")

	close(output)

	return nil
}

func (m *Manager) GetFingerbot(address string) *fingerbot.Fingerbot {
	return m.conectedDevices[address]
}

func (m *Manager) GetSavedDevices(ctx context.Context) ([]DeviceView, error) {
	devices, err := m.repository.GetDevices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	return utils.Map(devices, func(device *Device) DeviceView {
		return DeviceView{
			Name:      device.Name,
			Address:   device.Address,
			RSSI:      0,
			Saved:     true,
			Connected: m.conectedDevices[device.Address] != nil,
		}
	}), nil
}

func (m *Manager) DisconnectDevice(ctx context.Context, address string) (*DeviceView, error) {
	device, err := m.repository.GetDevice(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}
	if device == nil {
		return nil, fmt.Errorf("device not found: %s", address)
	}

	if err := m.GetFingerbot(device.Address).Disconnect(); err != nil {
		return nil, fmt.Errorf("failed to disconnect device: %w", err)
	}

	delete(m.conectedDevices, device.Address)

	return &DeviceView{
		Name:      device.Name,
		Address:   device.Address,
		RSSI:      0,
		Saved:     true,
		Connected: false,
	}, nil
}

func (m *Manager) ForgetDevice(ctx context.Context, address string) error {
	if err := m.repository.DeleteDevice(ctx, address); err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
	}

	return nil
}

func (m *Manager) DisconnectDevices() {
	for _, device := range m.conectedDevices {
		if err := device.Disconnect(); err != nil {
			m.logger.Error("failed to disconnect device", slog.Any("error", err))
		}
	}
}

func (m *Manager) connectDevice(ctx context.Context, device *Device) error {
	if _, err := m.discoverer.DiscoverDevice(ctx, device.Address); err != nil {
		return err
	}

	m.discoverer.StopDiscovery()

	m.connectingMutex.Lock()
	defer m.connectingMutex.Unlock()

	tuyadevice, err := tuyable.NewDevice(device.Address, device.UUID, device.DeviceID, device.LocalKey, m.logger)
	if err != nil {
		return err
	}

	if err := tuyadevice.Connect(ctx); err != nil {
		return err
	}

	if err := tuyadevice.Pair(); err != nil {
		return err
	}

	m.conectedDevices[device.Address] = fingerbot.NewFingerbot(tuyadevice)

	return nil
}
