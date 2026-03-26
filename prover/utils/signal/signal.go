package signal

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/pkg/profile"
	"github.com/sirupsen/logrus"
)

// Registers a signal handler that dumps the StackTraces upon receiving a
// SIGUSR1. On SIGUSR2, it starts a profiling session for 10 seconds and then
// writes the result in a unique timestamped file.
func RegisterStackTraceDumpHandler() {

	sigChan := make(chan os.Signal, 1<<8)

	go func() {
		for sig := range sigChan {

			switch sig {
			case syscall.SIGUSR1:
				logrus.Infof("received signal %v -> dumping the stack traces", sig)
				// Allocate a buffer to hold the stack data
				buf := make([]byte, 1024*1024)
				n := runtime.Stack(buf, true) // 'true' gets all goroutines
				logrus.Infof("stack trace: %s", string(buf[:n]))

			case syscall.SIGUSR2:

				profilePath := fmt.Sprintf("profiling/onsigusr2-%d", time.Now().Unix())
				logrus.Infof("received signal %v -> profiling for 10 sec and dumping the result", sig)

				p := profile.Start(
					profile.ProfilePath(profilePath),
					profile.CPUProfile,
				)

				// Purposefully block the signal handler as we don't want several
				// profiles to be created simultaneously to avoid interferences.
				<-time.After(10 * time.Second)

				p.Stop()
				logrus.Infof("done profile, you may now open %v", profilePath)
			}
		}
	}()

	signal.Notify(sigChan, syscall.SIGUSR1, syscall.SIGUSR2)
	logrus.Infof("Registered the signal handler for %s|%s. If you send that signal to the process it will dump the stack traces", syscall.SIGUSR1, syscall.SIGUSR2)
}
