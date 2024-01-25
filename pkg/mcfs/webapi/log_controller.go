package webapi

import (
	"net/http"
	"os"
	"sync"

	"github.com/apex/log"
	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/clog"
	"github.com/pkg/errors"
)

type LogController struct {
	mu              sync.Mutex
	CurrentLogLevel log.Level `json:"current_log_level"`
	CurrentLogFile  string    `json:"current_log_file"`
	currentHandler  *clog.Handler
}

// NewLogController creates a new LogController
func NewLogController() *LogController {
	handler := clog.NewHandler(os.Stdout)
	log.SetHandler(handler)

	return &LogController{
		CurrentLogLevel: log.InfoLevel,
		CurrentLogFile:  "stdout",
		currentHandler:  handler,
	}
}

func (c *LogController) SetLogging(ctx echo.Context) error {
	var req struct {
		LogLevel  string `json:"log_level"`
		LogOutput string `json:"log_output"`
	}

	if err := ctx.Bind(&req); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	oldLevel := c.CurrentLogLevel
	if err := c.setLoggingLevel(req.LogLevel); err != nil {
		return err
	}

	if err := c.setLoggingOutput(req.LogOutput); err != nil {
		// Reset logging level to original level since we couldn't perform
		// both setting the level and the output.
		c.CurrentLogLevel = oldLevel
		log.SetLevel(c.CurrentLogLevel)
		return err
	}

	return ctx.JSON(http.StatusOK, c)
}

func (c *LogController) SetLogLevel(ctx echo.Context) error {
	var req struct {
		LogLevel string `json:"log_level"`
	}

	if err := ctx.Bind(&req); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.setLoggingLevel(req.LogLevel); err != nil {
		return err
	}

	return ctx.JSON(http.StatusOK, c)
}

func (c *LogController) setLoggingLevel(logLevel string) error {
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		return errors.Wrapf(err, "SetLogLevelController: Invalid log level %s", logLevel)
	}

	c.CurrentLogLevel = level
	log.SetLevel(c.CurrentLogLevel)

	return nil
}

func (c *LogController) SetLogOutput(ctx echo.Context) error {
	var req struct {
		LogOutput string `json:"log_output"`
	}

	if err := ctx.Bind(&req); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.setLoggingOutput(req.LogOutput); err != nil {
		return err
	}

	return ctx.JSON(http.StatusOK, c)
}

func (c *LogController) setLoggingOutput(logOutput string) error {
	if logOutput == "stdout" || logOutput == "stderr" {
		c.closeCurrentLogHandler()
		writer := os.Stdout
		if logOutput == "stderr" {
			writer = os.Stderr
		}

		c.CurrentLogFile = logOutput
		c.currentHandler = clog.NewHandler(writer)
		log.SetHandler(c.currentHandler)

		return nil
	}

	// If we are here then a file was specified. We need to verify that
	// we can write to the file.

	f, err := os.Create(logOutput)
	if err != nil {
		return errors.Wrapf(err, "Failed to open LogOutput %s", logOutput)
	}

	c.closeCurrentLogHandler()
	c.CurrentLogFile = logOutput
	c.currentHandler = clog.NewHandler(f)
	log.SetHandler(c.currentHandler)

	return nil
}

func (c *LogController) ShowCurrentLogging(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, c)
}

func (c *LogController) closeCurrentLogHandler() {
	switch c.CurrentLogFile {
	case "stdout", "stderr":
		// no need to close
	default:
		_ = c.currentHandler.Writer.Close()
	}
}
