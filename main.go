package main

import (
	"flag"
	"fmt"
	"github.com/fsnotify/fsnotify"
	corev1 "k8s.io/api/core/v1"
	"os"
	"path/filepath"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sync"
)

const (
	controllerName = "k8s-auto-labeller"
)

var (
	labelDir    = "labels"
	log         = logf.Log.WithName(controllerName)
	watcher     *fsnotify.Watcher
	watchLock   sync.RWMutex
	watchedDirs []string

	// nodeLabelMap contains the global node label state reflecting set (true) and recently cleared (false) labels
	// for the reconciler to act on, in the form of node name -> label -> true/false.
	nodeLabelMap     map[string]map[string]bool
	nodeLabelMapLock sync.RWMutex
)

func remove(slice []string, s int) []string {
	return append(slice[:s], slice[s+1:]...)
}

func main() {
	flag.StringVar(&labelDir, "label-dir", labelDir, "Label directory to monitor")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Node Auto Labeller for Kubernetes\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	logf.SetLogger(zap.New(zap.UseDevMode(false)))
	entryLog := log.WithName("entrypoint")

	watchedDirs = make([]string, 0)
	watcher, _ = fsnotify.NewWatcher()
	defer watcher.Close()

	if _, err := os.Stat(labelDir); os.IsNotExist(err) {
		entryLog.Error(err, "invalid label directory specified")
		os.Exit(1)
	}

	err := filepath.Walk(labelDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			entryLog.Info("Adding watcher", "dir", path)
			watchLock.Lock()
			watchedDirs = append(watchedDirs, path)
			watchLock.Unlock()
			_ = watcher.Add(path)
		}

		return nil
	})

	if err != nil {
		entryLog.Error(err, "unable to scan label dir")
		os.Exit(1)
	}

	nodeLabelMap = make(map[string]map[string]bool)

	labelMap, err := buildLabelMap()
	if err != nil {
		entryLog.Error(err, "unable to build label map")
		os.Exit(1)
	}

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		entryLog.Error(err, "unable to instantiate manager")
		os.Exit(1)
	}

	c, err := controller.New(controllerName, mgr, controller.Options{
		Reconciler: &reconcileNodeLabels{
			client: mgr.GetClient(),
			log:    log.WithName("reconciler"),
		},
	})

	if err != nil {
		entryLog.Error(err, "unable to set up individual controller")
		os.Exit(1)
	}

	pred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			name := e.Meta.GetName()
			nodeLabels := e.Meta.GetLabels()

			nodeLabelMapLock.Lock()
			defer nodeLabelMapLock.Unlock()

			nodeLabelMap[name] = make(map[string]bool)
			for label := range nodeLabels {
				for labelKey, fileLabels := range labelMap {
					for _, fileLabel := range fileLabels {
						if fileLabel == label {
							nodeLabelMap[name][labelKey] = true
							break
						}
					}
				}
			}

			if _, ok := nodeLabelMap[name]; ok {
				return len(nodeLabelMap[name]) > 0
			}

			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			nodeLabelMapLock.Lock()
			delete(nodeLabelMap, e.Meta.GetName())
			nodeLabelMapLock.Unlock()
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldName := e.MetaOld.GetName()
			newName := e.MetaNew.GetName()

			oldLabels := e.MetaOld.GetLabels()
			newLabels := e.MetaNew.GetLabels()

			eq := reflect.DeepEqual(oldLabels, newLabels)
			if !eq {
				nodeLabelMapLock.Lock()

				// Disable any previously set labels, the reconciler will use this to clear stale node labels
				for k := range nodeLabelMap[oldName] {
					nodeLabelMap[oldName][k] = false
				}

				if _, exists := nodeLabelMap[newName]; !exists {
					nodeLabelMap[newName] = make(map[string]bool)
				}

				// Identify labels to set
				for label := range newLabels {
					for labelKey, fileLabels := range labelMap {
						for _, fileLabel := range fileLabels {
							if fileLabel == label {
								nodeLabelMap[newName][labelKey] = true
								break
							}
						}
					}
				}

				nodeLabelMapLock.Unlock()
				return true
			}

			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	// Watch Nodes and enqueue Nodes object key
	if err := c.Watch(&source.Kind{Type: &corev1.Node{}}, &handler.EnqueueRequestForObject{}, &pred); err != nil {
		entryLog.Error(err, "unable to watch Node")
		os.Exit(1)
	}

	go func() {
		entryLog.Info("starting manager")
		if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
			entryLog.Error(err, "unable to run manager")
			os.Exit(1)
		}
	}()

	done := make(chan bool)

	go func() {
		entryLog.Info("Monitoring filesystem for events...")
		for {
			select {
			case evt := <-watcher.Events:
				if evt.Op&fsnotify.Create == fsnotify.Create {
					info, _ := os.Stat(evt.Name)
					if info.IsDir() {
						entryLog.Info("adding watcher", "dir", evt.Name)
						watchLock.Lock()
						watchedDirs = append(watchedDirs, evt.Name)
						watchLock.Unlock()
						_ = watcher.Add(evt.Name)
					}
				} else if evt.Op&fsnotify.Remove == fsnotify.Remove {
					watchLock.Lock()
					for iter, dir := range watchedDirs {
						if dir == evt.Name {
							entryLog.Info("removing watcher", "dir", evt.Name)
							_ = watcher.Remove(evt.Name)
							watchedDirs = remove(watchedDirs, iter)
						}
					}
					watchLock.Unlock()
				}

				labelMap, _ = buildLabelMap()
			case err := <-watcher.Errors:
				entryLog.Error(err, "received filesystem watcher error")
			}
		}
	}()

	<-done
}
