package cli_test

import (
	"os"
	"testing"

	"github.com/zahT3/hotseat/internal/paths"
)

// The suite was written for the macOS layout (~/.zshrc, ~/Library/...); pin
// it so it also passes when run on Linux. Linux behaviour has its own tests.
func TestMain(m *testing.M) {
	paths.GOOS = "darwin"
	os.Exit(m.Run())
}
