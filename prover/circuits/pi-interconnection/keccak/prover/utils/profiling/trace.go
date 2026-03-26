package profiling

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"runtime/trace"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	_ = 1 << (10 * iota)
	KiB
	MiB
	GiB
	TiB
)

func Trace(ctx context.Context, dir string) {

	LogMemUsage()

	go func() {
		ticker := time.NewTicker(20 * time.Second)
		counter := 0
		_path := path.Join(dir, fmt.Sprintf("trace-%v.out", counter))
		f := startTracingInFile(_path)
		fPtr := &f
		defer closeTraceFile(*fPtr)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				closeTraceFile(f) // gracefully stops the current trace
				counter++
				_path := path.Join(dir, fmt.Sprintf("trace-%v.out", counter))
				f = startTracingInFile(_path) // and start a new one
				logrus.Debugf("Started tracing in %v", _path)
			case <-ctx.Done():
				// we don't want to profile anymore
				logrus.Debug("Closing the tracer")
				return
			}
		}
	}()

}

// start tracing in a file
func startTracingInFile(path string) *os.File {
	var f *os.File
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}

	err = trace.Start(f)
	if err != nil {
		panic(err)
	}

	return f
}

// close trace
func closeTraceFile(f *os.File) {
	// close the file after we stop the tracer otherwise,
	// this will create a failure in the tracer
	trace.Stop()
	f.Close()
}

// Log memory usage
func LogMemUsage() {
	go func() {
		ticker := time.NewTicker(time.Second)
		for {
			<-ticker.C
			// read information about the memory usage
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			// {Alloc:362521518000 TotalAlloc:1076776397968 Sys:378298843400 Lookups:0 Mallocs:12131948845 Frees:8937352031 HeapAlloc:362521518000 HeapSys:362600792064 HeapIdle:28925952 HeapInuse:362571866112 HeapReleased:28925952 HeapObjects:3194596814 StackInuse:5177344 StackSys:5177344 MSpanInuse:1939723120 MSpanSys:1988836800 MCacheInuse:115200 MCacheSys:124800 BuckHashSys:1967672 GCSys:13477260696 OtherSys:224684024 NextGC:448199664696 LastGC:1690054704225362140
			logrus.Debugf(
				"Memory usage (GiB) : Alloc %.4f - TotalAlloc %.4f - Sys %.4f - Mallocs %.4f - Frees %.4f - HeapAlloc %.4f - NextGC %.4f",
				float64(m.Alloc)/GiB, float64(m.TotalAlloc)/GiB, float64(m.Sys)/GiB, float64(m.Mallocs)/GiB, float64(m.Frees)/GiB, float64(m.HeapAlloc)/GiB, float64(m.NextGC)/GiB,
			)

			if m.Alloc > 800_000_000_000 {
				logrus.Panicf("Out of memory") // the panic gives a chance to
			}
		}
	}()
}
