package apimiddleware

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type GetUserByAPIKeyFN func(string) (*mcmodel.User, error)

type APIKeyConfig struct {
	Skipper         middleware.Skipper
	Keyname         string
	GetUserByAPIKey GetUserByAPIKeyFN
}

func APIKeyAuth(config APIKeyConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			value, err := getAPIKeyFromRequest(config.Keyname, c)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, err.Error())
			}

			user, err := config.GetUserByAPIKey(value)
			switch {
			case err != nil:
				return echo.ErrUnauthorized
			case user == nil:
				return echo.ErrUnauthorized
			default:
				c.Set("User", *user)
				return next(c)
			}
		}
	}
}

func getAPIKeyFromRequest(key string, c echo.Context) (string, error) {
	if value, err := keyFromHeader(key, c); err == nil {
		return value, nil
	}

	if value, err := keyFromQuery(key, c); err == nil {
		return value, nil
	}

	return "", fmt.Errorf("no apikey '%s' as query param or header", key)
}

func keyFromHeader(key string, c echo.Context) (string, error) {
	value := c.Request().Header.Get(key)
	if value == "" {
		return "", fmt.Errorf("no apikey '%s' as header", key)
	}
	return value, nil
}

func keyFromQuery(key string, c echo.Context) (string, error) {
	value := c.QueryParam(key)
	if value == "" {
		return "", fmt.Errorf("no apikey '%s' as query param", key)
	}
	return value, nil
}
