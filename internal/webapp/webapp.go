package webapp

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"time"

	"github.com/cybre/fingerbot-web/internal/devices"
	"github.com/cybre/fingerbot-web/internal/tuyable/fingerbot"
	"github.com/labstack/echo/v4"
)

type WebApp struct {
	deviceManager *devices.Manager
	templates     *template.Template
}

func NewWebApp(
	deviceManager *devices.Manager,
) *WebApp {
	return &WebApp{
		deviceManager: deviceManager,
		templates:     template.Must(template.ParseGlob("public/*.html")),
	}
}

func (a *WebApp) RegisterRoutes(e *echo.Echo) {
	e.GET("/discover", a.handleDiscover)
	devicesGroup := e.Group("/devices")
	devicesGroup.GET("", a.handleDevices)
	devicesGroup.POST("", a.handleConnectDevice)

	deviceGroup := e.Group("/devices/:address")
	deviceGroup.POST("/connect", a.handleConnectToSavedDevice)
	deviceGroup.POST("/disconnect", a.handleDisconnectDevice)
	deviceGroup.POST("/forget", a.handleForgetDevice)
	deviceGroup.PUT("/toggle", a.handleToggle)
	deviceGroup.GET("", a.handleDeviceIndex)
	deviceGroup.GET("/configure", a.handleGetConfiguration)
	deviceGroup.PUT("/configure", a.handleSaveConfiguration)
	deviceGroup.GET("/battery-status", a.handleGetBatteryStatus)
}

func (t *WebApp) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func (a *WebApp) handleDevices(c echo.Context) error {
	savedDevices, err := a.deviceManager.GetSavedDevices(c.Request().Context())
	if err != nil {
		return fmt.Errorf("failed to get saved devices: %w", err)
	}

	return c.Render(http.StatusOK, "devices.html", savedDevices)
}

func (a *WebApp) handleDiscover(c echo.Context) error {
	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	output := make(chan devices.DeviceView)
	go func() {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 1*time.Minute)
		defer cancel()

		if err := a.deviceManager.Discover(ctx, output); err != nil {
			panic(err)
		}

		close(output)
	}()

	for device := range output {
		buff := bytes.NewBuffer(nil)
		if !device.Saved {
			if err := c.Echo().Renderer.Render(buff, "discovered_device.html", device, c); err != nil {
				return fmt.Errorf("failed to render discovered device: %w", err)
			}
		} else {
			if err := c.Echo().Renderer.Render(buff, "saved_device.html", device, c); err != nil {
				return fmt.Errorf("failed to render saved device: %w", err)
			}
		}
		event := Event{
			Event: []byte("device"),
			Data:  buff.Bytes(),
		}
		if err := event.MarshalTo(w); err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}
		w.Flush()
	}

	event := Event{
		Event: []byte("finished"),
		Data:  []byte("Scan finished"),
	}
	if err := event.MarshalTo(w); err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	w.Flush()

	return nil
}

func (a *WebApp) handleConnectDevice(c echo.Context) error {
	var request devices.DeviceConnection
	if err := c.Bind(&request); err != nil {
		return err
	}

	device, err := a.deviceManager.Connect(c.Request().Context(), request)
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "saved_device.html", device)
}

func (a *WebApp) handleConnectToSavedDevice(c echo.Context) error {
	device, err := a.deviceManager.ConnectToSavedDevice(c.Request().Context(), c.Param("address"))
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "saved_device.html", device)
}

func (a *WebApp) handleDisconnectDevice(c echo.Context) error {
	device, err := a.deviceManager.DisconnectDevice(c.Request().Context(), c.Param("address"))
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "saved_device.html", device)
}
func (a *WebApp) handleForgetDevice(c echo.Context) error {
	if err := a.deviceManager.ForgetDevice(c.Request().Context(), c.Param("address")); err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	return c.NoContent(http.StatusOK)
}

func (a *WebApp) handleToggle(c echo.Context) error {
	fingerbot := a.deviceManager.GetFingerbot(c.Param("address"))
	if fingerbot == nil {
		return c.NoContent(http.StatusBadRequest)
	}

	return fingerbot.SetSwitch(!fingerbot.Switch())
}

func (a *WebApp) handleDeviceIndex(c echo.Context) error {
	fingerbot := a.deviceManager.GetFingerbot(c.Param("address"))
	if fingerbot == nil {
		return c.Redirect(http.StatusTemporaryRedirect, "/devices")
	}

	return c.Render(http.StatusOK, "index.html", NewIndexData(fingerbot))
}

func (a *WebApp) handleGetConfiguration(c echo.Context) error {
	fingerbot := a.deviceManager.GetFingerbot(c.Param("address"))
	if fingerbot == nil {
		return c.Redirect(http.StatusTemporaryRedirect, "/devices")
	}

	return c.Render(http.StatusOK, "configure.html", NewConfigurationData(fingerbot))
}

func (a *WebApp) handleSaveConfiguration(c echo.Context) error {
	var config ConfigurationData
	if err := c.Bind(&config); err != nil {
		return err
	}

	device := a.deviceManager.GetFingerbot(c.Param("address"))
	if device == nil {
		return c.NoContent(http.StatusBadRequest)
	}

	if err := device.Transaction(func(t *fingerbot.FingerbotTransaction) error {
		if config.Mode != uint32(device.Mode()) {
			t.SetMode(fingerbot.Mode(config.Mode))
		}
		if config.ClickSustainTime != device.ClickSustainTime() {
			t.SetClickSustainTime(config.ClickSustainTime)
		}
		if config.ControlBack != uint32(device.ControlBack()) {
			t.SetControlBack(fingerbot.ControlBack(config.ControlBack))
		}
		if config.ArmDownPercent != device.ArmDownPercent() ||
			config.ArmUpPercent != device.ArmUpPercent() {
			t.SetArmPercent(config.ArmUpPercent, config.ArmDownPercent)
		}

		return nil
	}); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func (a *WebApp) handleGetBatteryStatus(c echo.Context) error {
	fingerbot := a.deviceManager.GetFingerbot(c.Param("address"))
	if fingerbot == nil {
		return c.NoContent(http.StatusBadRequest)
	}

	return c.JSON(http.StatusOK, NewBatteryStatusData(fingerbot))
}
