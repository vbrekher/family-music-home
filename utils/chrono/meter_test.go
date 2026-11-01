package chrono_test

import (
	"testing"
	"testing/synctest"
	"time"

	. "github.com/navidrome/navidrome/utils/chrono"
)

// Virtual time verifies stopwatch behavior without assuming how quickly a
// loaded or emulated host schedules two independent wall-clock sleeps.
func TestMeterStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := &Meter{}
		m.Start()
		time.Sleep(50 * time.Millisecond)
		if got := m.Stop(); got != 50*time.Millisecond {
			t.Fatalf("elapsed = %v", got)
		}
		time.Sleep(time.Second)
		if got := m.Stop(); got != 50*time.Millisecond {
			t.Fatalf("stopped meter advanced: %v", got)
		}
	})
}

func TestMeterAccumulates(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := &Meter{}
		m.Start()
		time.Sleep(50 * time.Millisecond)
		m.Stop()
		time.Sleep(time.Second)
		m.Start()
		time.Sleep(80 * time.Millisecond)
		if got := m.Stop(); got != 130*time.Millisecond {
			t.Fatalf("accumulated = %v", got)
		}
	})
}

func TestMeterElapsedExcludesPausedTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := &Meter{}
		m.Start()
		time.Sleep(50 * time.Millisecond)
		m.Stop()
		time.Sleep(time.Second)
		if got := m.Elapsed(); got != 50*time.Millisecond {
			t.Fatalf("paused elapsed = %v", got)
		}
		m.Start()
		time.Sleep(50 * time.Millisecond)
		m.Stop()
		if got := m.Elapsed(); got != 100*time.Millisecond {
			t.Fatalf("total elapsed = %v", got)
		}
	})
}

func TestMeterElapsedIncludesRunningTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := &Meter{}
		m.Start()
		time.Sleep(50 * time.Millisecond)
		if got := m.Elapsed(); got != 50*time.Millisecond {
			t.Fatalf("running elapsed = %v", got)
		}
		time.Sleep(50 * time.Millisecond)
		if got := m.Elapsed(); got != 100*time.Millisecond {
			t.Fatalf("running elapsed did not advance: %v", got)
		}
	})
}
