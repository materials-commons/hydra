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
}

// NewLogController creates a new LogController
func NewLogController() *LogController {
	return &LogController{
		CurrentLogLevel: log.InfoLevel,
		CurrentLogFile:  "stdout",
	}
}

/////////////////// Handlers ///////////////////

func (c *LogController) SetLoggingHandler(ctx echo.Context) error {
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
		_, _ = clog.SetGlobalLoggerLevel(c.CurrentLogLevel)
		return err
	}

	return ctx.JSON(http.StatusOK, c)
}

func (c *LogController) SetLogLevelHandler(ctx echo.Context) error {
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

func (c *LogController) SetLogOutputHandler(ctx echo.Context) error {
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

func (c *LogController) ShowCurrentLoggingHandler(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, c)
}

/////////////////// Utility Functions ///////////////////

func (c *LogController) setLoggingLevel(logLevel string) error {
	level, err := clog.SetGlobalLoggerLevelFromString(logLevel)
	if err != nil {
		return errors.Wrapf(err, "SetLogLevelController: Invalid log level %s", logLevel)
	}

	c.CurrentLogLevel = level

	return nil
}

func (c *LogController) setLoggingOutput(logOutput string) error {
	var (
		writer *os.File
		err    error
	)

	switch logOutput {
	case "stdout":
		writer = os.Stdout
	case "stderr":
		writer = os.Stderr
	default:
		writer, err = os.Create(logOutput)
		if err != nil {
			return errors.Wrapf(err, "Failed to open LogOutput %s", logOutput)
		}
	}

	if err = clog.SetGlobalOutput(writer); err != nil {
		return errors.Wrapf(err, "failed to set output to %s", logOutput)
	}

	c.CurrentLogFile = logOutput
	return nil
}
