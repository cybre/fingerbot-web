package devices

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cybre/fingerbot-web/internal/tuyable"
	"github.com/cybre/fingerbot-web/internal/tuyable/fingerbot"
	"github.com/cybre/fingerbot-web/internal/utils"
)

type Manager struct {
	repository      *Repository
	discoverer      *tuyable.Discoverer
	logger          *slog.Logger
	conectedDevices map[string]*fingerbot.Fingerbot
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
		m.logger.Info("connecting to device", slog.Any("device", device))
		if err := m.connectDevice(ctx, device); err != nil {
			m.logger.Error("failed to connect to device", slog.Any("device", device), slog.Any("error", err))
		}
	}

	return nil
}

type DeviceConnection struct {
	Address  string `json:"address" form:"address"`
	Name     string `json:"name" form:"name"`
	Slug     string `json:"slug" form:"slug"`
	DeviceID string `json:"deviceId" form:"deviceId"`
	LocalKey string `json:"localKey" form:"localKey"`
}

func (m *Manager) ConnectToSavedDevice(ctx context.Context, address string) (*DeviceView, error) {
	device, err := m.repository.GetDevice(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}
	if device == nil {
		return nil, fmt.Errorf("device not found: %s", address)
	}

	if err := m.connectDevice(ctx, *device); err != nil {
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
	discoveredDevice, ok := m.discoverer.GetDevice(conn.Address)
	if !ok {
		return nil, fmt.Errorf("device not found: %s", conn.Address)
	}

	device := Device{
		DeviceID: conn.DeviceID,
		Address:  discoveredDevice.Address,
		Name:     conn.Name,
		Slug:     conn.Slug,
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

func (m *Manager) Discover(ctx context.Context, output chan<- DeviceView) error {
	devices := make(chan tuyable.DiscoveredDevice)
	go func() {
		if err := m.discoverer.Discover(ctx, devices); err != nil {
			m.logger.Error("failed to discover devices", slog.Any("error", err))
		}

		close(devices)
	}()

	for tuyaDevice := range devices {
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
			device.Connected = m.conectedDevices[saved.Slug] != nil
			fmt.Printf("device: %v\n", device)
		}

		output <- device
	}

	return nil
}

func (m *Manager) GetDevice(slug string) *fingerbot.Fingerbot {
	return m.conectedDevices[slug]
}

func (m *Manager) GetSavedDevices(ctx context.Context) ([]DeviceView, error) {
	devices, err := m.repository.GetDevices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	return utils.Map(devices, func(device Device) DeviceView {
		return DeviceView{
			Name:      device.Name,
			Address:   device.Address,
			RSSI:      0,
			Saved:     true,
			Connected: m.conectedDevices[device.Slug] != nil,
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

	if err := m.GetDevice(device.Slug).Disconnect(); err != nil {
		return nil, fmt.Errorf("failed to disconnect device: %w", err)
	}

	delete(m.conectedDevices, device.Slug)

	return &DeviceView{
		Name:      device.Name,
		Address:   device.Address,
		RSSI:      0,
		Saved:     true,
		Connected: false,
	}, nil
}

func (m *Manager) connectDevice(ctx context.Context, device Device) error {
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

	m.conectedDevices[device.Slug] = fingerbot.NewFingerbot(tuyadevice)

	return nil
}

func (m *Manager) DisconnectDevices() {
	for _, device := range m.conectedDevices {
		if err := device.Disconnect(); err != nil {
			m.logger.Error("failed to disconnect device", slog.Any("error", err))
		}
	}
}
