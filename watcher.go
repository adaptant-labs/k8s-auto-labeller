package main

import (
	"github.com/fsnotify/fsnotify"
	"github.com/go-logr/logr"
	"os"
	"path/filepath"
	"sync"
)

func remove(slice []string, s int) []string {
	return append(slice[:s], slice[s+1:]...)
}

type LabelWatcher struct {
	fsWatcher   *fsnotify.Watcher
	watchedDirs []string
	log         logr.Logger

	sync.RWMutex
}

func (l *LabelWatcher) Close() error {
	return l.fsWatcher.Close()
}

func (l *LabelWatcher) Watch(done chan bool, refresh chan bool) {
	l.log.Info("Monitoring filesystem for events...")
	for {
		select {
		case evt := <-l.fsWatcher.Events:
			var err error

			if evt.Op&fsnotify.Create == fsnotify.Create {
				info, _ := os.Stat(evt.Name)
				if info.IsDir() {
					l.log.Info("adding watcher", "dir", evt.Name)
					l.Lock()
					l.watchedDirs = append(l.watchedDirs, evt.Name)
					l.fsWatcher.Add(evt.Name)
					l.Unlock()
				}
			} else if evt.Op&fsnotify.Remove == fsnotify.Remove {
				l.Lock()
				for iter, dir := range l.watchedDirs {
					if dir == evt.Name {
						l.log.Info("removing watcher", "dir", evt.Name)
						l.fsWatcher.Remove(evt.Name)
						l.watchedDirs = remove(l.watchedDirs, iter)
					}
				}
				l.Unlock()
			}

			// Reconstruct the list of possible labels. This must be done with the write lock held, as the manager
			// goroutine may already be in the process of iterating the map when a filesystem update is triggered.
			possibleLabelLock.Lock()
			possibleLabelMap, err = buildPossibleLabelMap()
			possibleLabelLock.Unlock()

			if err != nil {
				l.log.Error(err, "failed to update label map")
				done <- true
			} else {
				// Label map has been updated, manually trigger node label reconciliation
				refresh <- true
			}
		case err := <-l.fsWatcher.Errors:
			l.log.Error(err, "received filesystem watcher error")
			done <- true
		}
	}
}

func NewLabelWatcher(basePath string) (*LabelWatcher, error) {
	watcherLog := log.WithName("watcher")

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	watcher := &LabelWatcher{
		log:         watcherLog,
		watchedDirs: make([]string, 0),
		fsWatcher:   fsWatcher,
	}

	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		log.Error(err, "invalid label directory specified")
		return nil, err
	}

	err = filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			log.Info("Adding watcher", "dir", path)
			watcher.Lock()
			watcher.watchedDirs = append(watcher.watchedDirs, path)
			watcher.fsWatcher.Add(path)
			watcher.Unlock()
		}

		return nil
	})

	if err != nil {
		log.Error(err, "unable to scan label dir")
		return nil, err
	}

	return watcher, nil
}
