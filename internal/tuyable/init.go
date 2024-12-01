package tuyable

import (
	"fmt"

	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

func init() {
	if err := adapter.Enable(); err != nil {
		panic(fmt.Errorf("error enabling bluetooth adapter: %w", err))
	}

	fmt.Printf("ServiceUUID: %#v\n", ServiceUUID)
	fmt.Printf("CharacteristicNotifyUUID: %#v\n", CharacteristicNotifyUUID)
	fmt.Printf("CharacteristicWriteUUID: %#v\n", CharacteristicWriteUUID)
}
