package tuyable

import (
	"fmt"

	"tinygo.org/x/bluetooth"
)

func init() {
	if err := bluetooth.DefaultAdapter.Enable(); err != nil {
		panic(fmt.Errorf("error enabling adapter: %w", err))
	}
}
