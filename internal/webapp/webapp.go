package webapp

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"

	"github.com/cybre/fingerbot-web/internal/tuyable"
	"github.com/cybre/fingerbot-web/internal/tuyable/fingerbot"
	"github.com/labstack/echo/v4"
)

type WebApp struct {
	deviceManager *tuyable.DeviceManager
	device        *fingerbot.Fingerbot
	templates     *template.Template
}

func NewWebApp(
	deviceManager *tuyable.DeviceManager,
	device *fingerbot.Fingerbot,
) *WebApp {
	return &WebApp{
		device:    device,
		templates: template.Must(template.ParseGlob("public/*.html")),
	}
}

func (a *WebApp) RegisterRoutes(e *echo.Echo) {
	e.GET("/devices", a.handleDevices)
	e.GET("/scan", a.handleScan)
	e.PUT("/toggle", a.handleToggle)
	e.GET("/", a.handleIndex)
	e.GET("/configure", a.handleGetConfiguration)
	e.PUT("/configure", a.handleSaveConfiguration)
	e.GET("/battery-status", a.handleGetBatteryStatus)
}

func (t *WebApp) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func (a *WebApp) handleDevices(c echo.Context) error {
	return c.Render(http.StatusOK, "devices.html", nil)
}

func (a *WebApp) handleScan(c echo.Context) error {
	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	output := make(chan tuyable.DiscoveredDevice)
	go func() {
		if err := a.deviceManager.Scan(c.Request().Context(), output); err != nil {
			panic(err)
		}

		close(output)
	}()

	for device := range output {
		buff := bytes.NewBuffer(nil)
		if err := c.Echo().Renderer.Render(buff, "device.html", device, c); err != nil {
			return fmt.Errorf("failed to render device: %w", err)
		}
		event := Event{
			Data: buff.Bytes(),
		}
		if err := event.MarshalTo(w); err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}
		w.Flush()
	}

	return nil
}

func (a *WebApp) handleToggle(c echo.Context) error {
	return a.device.SetSwitch(!a.device.Switch())
}

func (a *WebApp) handleIndex(c echo.Context) error {
	return c.Render(http.StatusOK, "index.html", NewIndexData(a.device))
}

func (a *WebApp) handleGetConfiguration(c echo.Context) error {
	return c.Render(http.StatusOK, "configure.html", NewConfigurationData(a.device))
}

func (a *WebApp) handleSaveConfiguration(c echo.Context) error {
	var config ConfigurationData
	if err := c.Bind(&config); err != nil {
		return err
	}

	if err := a.device.Transaction(func(t *fingerbot.FingerbotTransaction) error {
		if config.Mode != uint32(a.device.Mode()) {
			t.SetMode(fingerbot.Mode(config.Mode))
		}
		if config.ClickSustainTime != a.device.ClickSustainTime() {
			t.SetClickSustainTime(config.ClickSustainTime)
		}
		if config.ControlBack != uint32(a.device.ControlBack()) {
			t.SetControlBack(fingerbot.ControlBack(config.ControlBack))
		}
		if config.ArmDownPercent != a.device.ArmDownPercent() ||
			config.ArmUpPercent != a.device.ArmUpPercent() {
			t.SetArmPercent(config.ArmUpPercent, config.ArmDownPercent)
		}

		return nil
	}); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func (a *WebApp) handleGetBatteryStatus(c echo.Context) error {
	return c.JSON(http.StatusOK, NewBatteryStatusData(a.device))
}
