package mcproxy

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// implements middleware.ProxyBalancer for echo
type Balancer struct {
}

func (b *Balancer) AddTarget(t *middleware.ProxyTarget) bool {
	return false
}

func (b *Balancer) RemoveTarget(t string) bool {
	return false
}

func (b *Balancer) Next(c echo.Context) *middleware.ProxyTarget {
	return nil
}
