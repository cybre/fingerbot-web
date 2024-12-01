package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/cybre/fingerbot-web/internal/tuyable"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	output := make(chan tuyable.DiscoveredDevice, 100)
	devicemanager := tuyable.NewDeviceManager()
	go func() {
		if err := devicemanager.Scan(ctx, output); err != nil {
			panic(err)
		}

		close(output)
	}()

	slog.Info("Scanning for devices")
	for device := range output {

		slog.Info(
			"Discovered device",
			"localName", device.LocalName,
			"address", device.Address.String(),
			"isBound", device.IsBound,
			"protocolVersion", device.ProtocolVersion,
			"uuid", string(device.UUID),
		)
	}
}
