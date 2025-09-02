package apimiddleware

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type HasAccessToProjectFN func(userID, projectID int) (bool, error)

type ProjectAccessConfig struct {
	Skipper            middleware.Skipper
	HasAccessToProject HasAccessToProjectFN
}

// ProjectAccessAuth middleware checks that the user has access to the project. It assumes that the
// project ID is passed in as a query parameter and that the user is stored in the context as "user".
// This means the APIKeyAuth middleware must be used before this middleware.
func ProjectAccessAuth(config ProjectAccessConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			projectId, err := strconv.Atoi(c.QueryParam("project_id"))
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "Invalid project ID")
			}

			user := c.Get("user").(*mcmodel.User)
			if user == nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "User not authenticated")
			}

			hasAccess, err := config.HasAccessToProject(user.ID, projectId)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}

			if !hasAccess {
				return echo.NewHTTPError(http.StatusForbidden, "User does not have access to project")
			}
			return next(c)
		}
	}
}
