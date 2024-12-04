package tuyable

import (
	"fmt"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
)

func init() {
	d, err := linux.NewDevice()
	if err != nil {
		panic(fmt.Errorf("error creating BLE device: %w", err))
	}
	ble.SetDefaultDevice(d)
}
