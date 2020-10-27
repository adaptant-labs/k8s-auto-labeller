package main

import (
	"context"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconcileNodeLabels struct {
	client client.Client
	log    logr.Logger
}

// make sure reconcileNodeLabels implement the Reconciler interface
var _ reconcile.Reconciler = &reconcileNodeLabels{}

func (r *reconcileNodeLabels) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// set up a convenient log object so we don't have to type request over and over again
	log := r.log.WithValues("request", request)

	log.Info("Reconciling node", "node", request.Name)

	node := &corev1.Node{}
	err := r.client.Get(context.TODO(), request.NamespacedName, node)
	if errors.IsNotFound(err) {
		log.Error(nil, "Could not find Node")
		return reconcile.Result{}, nil
	}

	if err != nil {
		log.Error(err, "Could not fetch Node")
		return reconcile.Result{}, err
	}

	nodeLabelMapLock.Lock()

	// Reconcile an individual node's labels with its defined state in the node label map.
	// As the node label map may be updated for cleared labels, this must be called with the
	// label map's write lock held.
	for label, set := range nodeLabelMap[request.Name] {
		if set {
			log.Info("Setting label", "label", label)
			node.Labels[label] = "true"
		} else {
			log.Info("Clearing label", "label", label)
			delete(node.Labels, label)
			delete(nodeLabelMap[request.Name], label)
		}
	}

	nodeLabelMapLock.Unlock()

	err = r.client.Update(context.TODO(), node)
	if err != nil {
		log.Error(err, "Could not write Node")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
