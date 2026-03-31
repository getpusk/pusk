package bot

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("github.com/pusk-platform/pusk/internal/bot.init.0.func1"),
	)
}
