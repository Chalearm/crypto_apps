package integration

import (
	"os"
	"strings"
	"testing"
)

// pkEnv reads PRIVATE_KEY from env or config.env. Never hardcoded.
func pkEnv(t *testing.T) string {
	t.Helper()
	if pk := os.Getenv("PRIVATE_KEY"); pk != "" {
		return strings.TrimSpace(pk)
	}
	for _, p := range []string{"../config.env", "../../config.env", "config.env"} {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRIVATE_KEY=") {
				return strings.TrimSpace(strings.TrimPrefix(line, "PRIVATE_KEY="))
			}
		}
	}
	t.Skip("PRIVATE_KEY not found")
	return ""
}
