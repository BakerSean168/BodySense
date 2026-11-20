package service

import (
	"os"

	"github.com/bodysense/api/internal/config"
)

// testProcessConfig performs the same composition-root read as the server, so
// policy and wiring tests keep exercising real environment resolution.
func testProcessConfig() config.Config { return config.Load(os.Getenv) }
