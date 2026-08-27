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
	// IgnoreTopFunction only matches when that function is the literal top
	// frame, but this goroutine spends most of its life blocked inside
	// time.Sleep, which becomes the top frame instead - use IgnoreAnyFunction
	// so it matches regardless of which frame goleak catches it at.
	goleak.VerifyNone(t, goleak.IgnoreAnyFunction("github.com/orca-zhang/ecache.init.0.func1"))

	if d, e := NewOtherService().Search("test"); e != nil || d == nil {
		t.Error("then test search error", e)
	}
}
