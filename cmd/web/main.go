package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"github.com/golang-cz/devslog"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/mattn/go-sqlite3"

	"github.com/cybre/fingerbot-web/internal/config"
	"github.com/cybre/fingerbot-web/internal/devices"
	"github.com/cybre/fingerbot-web/internal/logging"
	"github.com/cybre/fingerbot-web/internal/tuyable"
	"github.com/cybre/fingerbot-web/internal/webapp"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	fmt.Println("Starting Fingerbot Web")
	config, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	var handler slog.Handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.Level(config.LoggingLevel),
	})
	if config.LoggingDevOutput {
		handler = devslog.NewHandler(os.Stdout, &devslog.Options{
			HandlerOptions: &slog.HandlerOptions{
				AddSource: true,
				Level:     slog.Level(config.LoggingLevel),
			},
			NewLineAfterLog:   true,
			StringerFormatter: true,
		})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx = logging.Context(ctx, logger)

	db, err := sql.Open("sqlite3", "./fingerbot-web.db")
	if err != nil {
		log.Fatalf("error opening database: %s", err)
	}

	deviceManager := devices.NewManager(devices.NewRepository(db), tuyable.NewDiscoverer(logger), logger)

	if err := deviceManager.ConnectToSavedDevices(ctx); err != nil {
		log.Fatalf("error connecting to existing devices: %s", err)
	}

	application := webapp.NewWebApp(deviceManager)
	e := echo.New()
	e.Renderer = application
	e.Use(middleware.Recover())
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:       true,
		LogMethod:       true,
		LogURI:          true,
		LogError:        true,
		LogLatency:      true,
		LogResponseSize: true,
		HandleError:     true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			logger := logging.FromContext(c.Request().Context())

			if v.Error == nil {
				logger.LogAttrs(ctx, slog.LevelInfo, "REQUEST",
					slog.String("method", v.Method),
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.Duration("latency", v.Latency),
					slog.Int64("response_size", v.ResponseSize),
				)
			} else {
				logger.LogAttrs(ctx, slog.LevelError, "REQUEST_ERROR",
					slog.String("method", v.Method),
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.Duration("latency", v.Latency),
					slog.Int64("response_size", v.ResponseSize),
					slog.String("err", v.Error.Error()),
				)
			}
			return nil
		},
	}))
	application.RegisterRoutes(e)

	go func() {
		if err := e.Start(":" + config.ServicePort); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				log.Println("server closed")
				return
			}

			log.Fatalf("error starting server: %s", err)
		}
	}()

	<-ctx.Done()

	deviceManager.DisconnectDevices()

	if err := e.Close(); err != nil {
		logger.Error("error shutting down server", slog.Any("error", err))
	}

	db.Close()
}
