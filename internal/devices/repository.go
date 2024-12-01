package devices

import (
	"context"
	"database/sql"
	"fmt"
)

type Device struct {
	Address  string `sql:"address"`
	DeviceID string `sql:"device_id"`
	Name     string `sql:"name"`
	LocalKey string `sql:"local_key"`
	UUID     string `sql:"uuid"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	repo := &Repository{db: db}
	if err := repo.init(); err != nil {
		panic(err)
	}

	return repo
}

func (r *Repository) init() error {
	if _, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS devices (
			address TEXT PRIMARY KEY,
			device_id TEXT,
			name TEXT NOT NULL,
			local_key TEXT NOT NULL,
			uuid TEXT NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("error creating devices table: %w", err)
	}

	return nil
}

func (r *Repository) CreateDevice(ctx context.Context, d Device) error {
	if _, err := r.db.ExecContext(
		ctx,
		"INSERT INTO devices (address, device_id, name, local_key, uuid) VALUES ($1, $2, $3, $4, $5)",
		d.Address, d.DeviceID, d.Name, d.LocalKey, d.UUID,
	); err != nil {
		return fmt.Errorf("error creating device: %w", err)
	}

	return nil
}

func (r *Repository) DeleteDevice(ctx context.Context, address string) error {
	if _, err := r.db.ExecContext(
		ctx, "DELETE FROM devices WHERE address = $1", address,
	); err != nil {
		return fmt.Errorf("error deleting device: %w", err)
	}

	return nil
}

func (r *Repository) GetDevice(ctx context.Context, address string) (*Device, error) {
	var d Device
	err := r.db.QueryRowContext(
		ctx, "SELECT * FROM devices WHERE address = $1", address,
	).Scan(&d.Address, &d.DeviceID, &d.Name, &d.LocalKey, &d.UUID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("error getting device by address: %w", err)
	}

	return &d, nil
}

func (r *Repository) GetDevices(ctx context.Context) ([]Device, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT * FROM devices")
	if err != nil {
		return nil, fmt.Errorf("error getting devices: %w", err)
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.Address, &d.DeviceID, &d.Name, &d.LocalKey, &d.UUID); err != nil {
			return nil, fmt.Errorf("error scanning device: %w", err)
		}

		devices = append(devices, d)
	}

	return devices, nil
}
