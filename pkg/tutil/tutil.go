package tutil

import (
	"os"
	"strings"
)

func IsIntegrationTest() bool {
	testType := os.Getenv("MC_TEST")
	return strings.ToLower(testType) == "integration"
}
