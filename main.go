package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"os"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sync"
)

const (
	controllerName = "k8s-auto-labeller"
)

var (
	appfs    afero.Fs
	fsutil   *afero.Afero
	labelDir = "labels"
	log      = logf.Log.WithName(controllerName)

	// possibleLabelMap contains the global state of all possible labels gleaned from the label dirs. It is dynamically
	// rebuilt when the filesystem watchers observe changes to the label files. It is implemented in the form of
	// label: [dependendent labels...]
	possibleLabelMap  map[string][]string
	possibleLabelLock sync.RWMutex

	// nodeLabelMap contains the global node label state reflecting set (true) and recently cleared (false) labels
	// for the reconciler to act on, in the form of node name -> label -> true/false.
	nodeLabelMap *NodeLabelMap
)

func init() {
	appfs = afero.NewOsFs()
	fsutil = &afero.Afero{Fs: appfs}
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

	labelWatcher, err := NewLabelWatcher(labelDir)
	if err != nil {
		entryLog.Error(err, "failed to initialize label watcher")
		os.Exit(1)
	}
	defer labelWatcher.Close()

	nodeLabelMap = NewNodeLabelMap()

	possibleLabelMap, err = buildPossibleLabelMap()
	if err != nil {
		entryLog.Error(err, "unable to build initial label map")
		os.Exit(1)
	}

	cfg := config.GetConfigOrDie()
	clientset := kubernetes.NewForConfigOrDie(cfg)

	mgr, err := manager.New(cfg, manager.Options{})
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

			nodeLabelMap.Lock()
			defer nodeLabelMap.Unlock()

			nodeLabelMap.Add(name)
			nodeLabelMap.SetPossible(name, nodeLabels)

			return nodeLabelMap.Valid(name)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			nodeLabelMap.Lock()
			nodeLabelMap.Remove(e.Meta.GetName())
			nodeLabelMap.Unlock()
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldName := e.MetaOld.GetName()
			newName := e.MetaNew.GetName()

			oldLabels := e.MetaOld.GetLabels()
			newLabels := e.MetaNew.GetLabels()

			eq := reflect.DeepEqual(oldLabels, newLabels)
			if !eq {
				nodeLabelMap.Lock()

				nodeLabelMap.ResetLabels(oldName)
				nodeLabelMap.Add(newName)
				nodeLabelMap.SetPossible(newName, newLabels)

				nodeLabelMap.Unlock()
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

	done := make(chan bool)
	refresh := make(chan bool)

	go func() {
		entryLog.Info("starting manager")
		if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
			entryLog.Error(err, "unable to run manager")
			os.Exit(1)
		}
	}()

	go labelWatcher.Watch(done, refresh)

	for {
		select {
		case <-refresh:
			entryLog.Info("Refreshing node labels")
			nodeList, _ := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
			for _, node := range nodeList.Items {
				nodeLabelMap.Lock()

				nodeLabelMap.ResetLabels(node.Name)
				nodeLabelMap.Add(node.Name)
				nodeLabelMap.SetPossible(node.Name, node.Labels)

				nodeLabelMap.Unlock()

				// Kick-off node reconciliation
				c.Reconcile(reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      node.Name,
						Namespace: node.Namespace,
					},
				})
			}
		case <-done:
			entryLog.Info("Exiting..")
			return
		}
	}
}
