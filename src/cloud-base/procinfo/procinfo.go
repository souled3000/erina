package procinfo

import (
	"fmt"
	"time"

	stat "cloud-base/goprocinfo/linux"
)

// Event when no error happened, Usage is current cpu usage(eg: 0.5).
type Event struct {
	Usage float64
	Error error
}

// NewWatcher produce one cpu usage num per |dur|; the returned chan
// would be closed when error happened, or close the chan when client code
// want to stop the watcher goroutine.
func NewWatcher(dur time.Duration) chan Event {
	ch := make(chan Event)
	go watchCpu(ch, dur)
	return ch
}

func watchCpu(ch chan Event, dur time.Duration) {
	defer func() {
		if e := recover(); e != nil {
			err := fmt.Errorf("watchCpu failed: %v", e)
			ch <- Event{Usage: 0.0, Error: err}
		}
	}()

	idlePre, totalPre, err := getCurCpu()
	if err != nil {
		ch <- Event{0.0, err}
		return
	}

	ticker := time.NewTicker(dur)
	defer ticker.Stop()

	for _ = range ticker.C {
		idle, total, err := getCurCpu()
		if err != nil {
			ch <- Event{0.0, err}
			break
		}

		totalDur := total - totalPre

		usage := float64(totalDur-idle+idlePre) / float64(totalDur)
		ch <- Event{Usage: usage, Error: nil}

		idlePre = idle
		totalPre = total
	}
}

func getCurCpu() (idle, total uint64, err error) {
	stats, e := stat.ReadStat("/proc/stat")
	if e != nil {
		err = fmt.Errorf("Failed to ReadStat: %v", e)
		return
	}
	info := stats.CPUStatAll

	idle = info.Idle
	total = info.User + info.Nice + info.System + info.Idle + info.IOWait +
		info.IRQ + info.SoftIRQ + info.Steal + info.Guest + info.GuestNice
	return
}
