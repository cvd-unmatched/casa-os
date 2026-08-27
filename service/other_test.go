package service

import (
	"testing"

	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"go.uber.org/goleak"
)

func TestSearch(t *testing.T) {
	logger.LogInitConsoleOnly()
	// ecache starts its own permanent background sweep goroutine in an
	// init() the moment the package is imported (not something Search or
	// any other code under test spins up) - goleak flags it as "leaked"
	// regardless, since it's still running whenever this check runs.
	// Confirmed live in CI, which had never actually run this test before.
	goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/orca-zhang/ecache.init.0.func1"))

	if d, e := NewOtherService().Search("test"); e != nil || d == nil {
		t.Error("then test search error", e)
	}
}
