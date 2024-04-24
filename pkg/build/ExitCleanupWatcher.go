package build

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

type ExitCleanupWatcher struct {
	paths []string
}

// NewExitCleanupWatcher creates a new ExitCleanupWatcher and starts watching for exit signals.
// This is useful for situations like layer creation, where a temp file's lifetime is not tied closely to the creation point.
func NewExitCleanupWatcher() *ExitCleanupWatcher {
	watcher := &ExitCleanupWatcher{}
	watcher.watch()
	return watcher
}

func (w *ExitCleanupWatcher) Append(path string) {
	w.paths = append(w.paths, path)
}

func (w *ExitCleanupWatcher) Close() {
	for _, path := range w.paths {
		if e := os.RemoveAll(path); e != nil {
			log.Println("failed to remove", path, ":", e)
		}
	}
}

func (w *ExitCleanupWatcher) watch() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		w.Close()
	}()
}
