package limitless

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/sirupsen/logrus"
)

var perfLogPath = fmt.Sprintf("./limitless_perf_%s.jsonl", time.Now().UTC().Format("20060102_150405"))

// perfEvent is a single JSONL event capturing timing and resource usage.
type perfEvent struct {
	Timestamp   string  `json:"ts"`
	Event       string  `json:"event"`               // phase_start, phase_end, job_start, job_end
	Phase       string  `json:"phase"`               // bootstrapper, GL, LPP, conglomeration, setup, outer_proof
	Index       *int    `json:"index,omitempty"`     // job index (GL/LPP) — pointer so 0 is not omitted
	Module      string  `json:"module,omitempty"`    // module name (GL/LPP)
	ElapsedS    float64 `json:"elapsed_s,omitempty"` // wall-clock seconds (for _end events)
	Goroutines  int     `json:"goroutines"`
	CPUPercent  float64 `json:"cpu_percent"` // sum across all cores
	HeapInUseGB float64 `json:"heap_inuse_gib"`
	HeapAllocGB float64 `json:"heap_alloc_gib"`
	SysGB       float64 `json:"sys_gib"`
	GCCycles    uint32  `json:"gc_cycles"`
	LiveObjects uint64  `json:"live_objects"`
}

// perfLogger writes JSONL events to a file, safe for concurrent use.
type perfLogger struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
}

func NewPerfLogger() *perfLogger {
	if os.Getenv("LIMITLESS_PERF_LOG") != "true" {
		return nil
	}
	f, err := os.Create(perfLogPath)
	if err != nil {
		logrus.Warnf("perf_log: could not create %s: %v (perf logging disabled)", perfLogPath, err)
		return nil
	}
	logrus.Infof("perf_log: writing events to %s", perfLogPath)
	return &perfLogger{file: f, enc: json.NewEncoder(f)}
}

func (pl *perfLogger) Close() {
	if pl == nil {
		return
	}
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.file.Close()
}

// snap captures a point-in-time resource snapshot and writes a JSONL event.
func (pl *perfLogger) snap(event, phase string, index int, module string, elapsed time.Duration) {
	if pl == nil {
		return
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	cpuPct := 0.0
	if percents, err := cpu.Percent(0, true); err == nil {
		for _, p := range percents {
			cpuPct += p
		}
	}

	idxPtr := &index
	if event == "phase_start" || event == "phase_end" {
		idxPtr = nil // phase events don't have an index
	}

	ev := perfEvent{
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		Event:       event,
		Phase:       phase,
		Index:       idxPtr,
		Module:      module,
		ElapsedS:    elapsed.Seconds(),
		Goroutines:  runtime.NumGoroutine(),
		CPUPercent:  cpuPct,
		HeapInUseGB: float64(mem.HeapInuse) / (1 << 30),
		HeapAllocGB: float64(mem.Alloc) / (1 << 30),
		SysGB:       float64(mem.Sys) / (1 << 30),
		GCCycles:    mem.NumGC,
		LiveObjects: mem.HeapObjects,
	}

	pl.mu.Lock()
	defer pl.mu.Unlock()
	if err := pl.enc.Encode(ev); err != nil {
		logrus.Warnf("perf_log: write error: %v", err)
	}
}

// phaseStart logs a phase_start event and returns the start time.
func (pl *perfLogger) phaseStart(phase string) time.Time {
	pl.snap("phase_start", phase, 0, "", 0)
	return time.Now()
}

// phaseEnd logs a phase_end event with elapsed time since start.
func (pl *perfLogger) phaseEnd(phase string, start time.Time) {
	pl.snap("phase_end", phase, 0, "", time.Since(start))
}

// jobStart logs a job_start event and returns the start time.
func (pl *perfLogger) jobStart(phase string, index int) time.Time {
	pl.snap("job_start", phase, index, "", 0)
	return time.Now()
}

// jobEnd logs a job_end event with elapsed time and module name.
func (pl *perfLogger) jobEnd(phase string, index int, module string, start time.Time) {
	pl.snap("job_end", phase, index, module, time.Since(start))
}

// flush syncs the JSONL file to disk to prevent data loss.
func (pl *perfLogger) flush() {
	if pl == nil {
		return
	}
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if pl.file != nil {
		pl.file.Sync()
	}
}

// gcStats logs GC memory statistics before and after explicit GC.
func (pl *perfLogger) gcStats(gcStart, gcEnd time.Time, heapBefore, heapAfter float64) {
	if pl == nil {
		return
	}

	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	cpuPct := 0.0
	if percents, err := cpu.Percent(0, true); err == nil {
		for _, p := range percents {
			cpuPct += p
		}
	}

	ev := perfEvent{
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		Event:       "gc_stats",
		Phase:       "GC",
		Module:      fmt.Sprintf("heap_freed_gib=%.2f", heapBefore-heapAfter),
		ElapsedS:    gcEnd.Sub(gcStart).Seconds(),
		Goroutines:  runtime.NumGoroutine(),
		CPUPercent:  cpuPct,
		HeapInUseGB: float64(memAfter.HeapInuse) / (1 << 30),
		HeapAllocGB: float64(memAfter.Alloc) / (1 << 30),
		SysGB:       float64(memAfter.Sys) / (1 << 30),
		GCCycles:    memAfter.NumGC,
		LiveObjects: memAfter.HeapObjects,
	}

	pl.mu.Lock()
	defer pl.mu.Unlock()
	if err := pl.enc.Encode(ev); err != nil {
		logrus.Warnf("perf_log: write error: %v", err)
	}
}

// jobOrder logs the order in which GL/LPP jobs will execute.
func (pl *perfLogger) jobOrder(phase string, order []int) {
	if pl == nil {
		return
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	cpuPct := 0.0
	if percents, err := cpu.Percent(0, true); err == nil {
		for _, p := range percents {
			cpuPct += p
		}
	}

	// Format order as a comma-separated string
	orderStr := ""
	for i, idx := range order {
		if i > 0 {
			orderStr += ","
		}
		orderStr += fmt.Sprintf("%d", idx)
	}

	ev := perfEvent{
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		Event:       "job_order",
		Phase:       phase,
		Module:      orderStr,
		Goroutines:  runtime.NumGoroutine(),
		CPUPercent:  cpuPct,
		HeapInUseGB: float64(memStats.HeapInuse) / (1 << 30),
		HeapAllocGB: float64(memStats.Alloc) / (1 << 30),
		SysGB:       float64(memStats.Sys) / (1 << 30),
		GCCycles:    memStats.NumGC,
		LiveObjects: memStats.HeapObjects,
	}

	pl.mu.Lock()
	defer pl.mu.Unlock()
	if err := pl.enc.Encode(ev); err != nil {
		logrus.Warnf("perf_log: write error: %v", err)
	}
}
