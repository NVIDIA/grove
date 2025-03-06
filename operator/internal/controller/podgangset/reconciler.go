// /*
// Copyright 2024 The Grove Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package podgangset

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllogger "sigs.k8s.io/controller-runtime/pkg/log"

	configv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/utils"
)

// Reconciler reconciles PodGangSet resources.
type Reconciler struct {
	config        configv1alpha1.PodGangSetControllerConfiguration
	client        ctrlclient.Client
	eventRecorder record.EventRecorder
	logger        logr.Logger
}

// NewReconciler creates a new reconciler for PodGangSet.
func NewReconciler(mgr ctrl.Manager, controllerCfg configv1alpha1.PodGangSetControllerConfiguration) *Reconciler {
	logger := ctrllogger.Log.WithName(controllerName)
	return &Reconciler{
		config:        controllerCfg,
		client:        mgr.GetClient(),
		eventRecorder: mgr.GetEventRecorderFor(controllerName),
		logger:        logger,
	}
}

// Reconcile reconciles a PodGangSet resource.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger.Info("PodGangSet reconciliation started", "resource", req.NamespacedName)

	pgs := &v1alpha1.PodGangSet{}
	if err := r.client.Get(ctx, req.NamespacedName, pgs); err != nil {
		r.logger.Error(err, "PodGangSet not found", "name", req.NamespacedName.String())
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Add finalizer if not being deleted and finalizer is missing
	if pgs.ObjectMeta.DeletionTimestamp.IsZero() {
		if !utils.ContainsString(pgs.ObjectMeta.Finalizers, finalizerName) {
			pgs.ObjectMeta.Finalizers = append(pgs.ObjectMeta.Finalizers, finalizerName)
		}
		return r.reconcile(ctx, pgs)
	} else {
		// Handle deletion if finalizer is present
		if utils.ContainsString(pgs.ObjectMeta.Finalizers, finalizerName) {
			// Remove finalizer and update object
			pgs.ObjectMeta.Finalizers = utils.RemoveString(pgs.ObjectMeta.Finalizers, finalizerName)
		}
		return r.delete(ctx, pgs)
	}
}

func (r *Reconciler) reconcile(ctx context.Context, pgs *v1alpha1.PodGangSet) (ctrl.Result, error) {
	spec := &pgs.Spec
	for replicaID := range spec.Replicas {
		for _, templ := range spec.Template.Cliques {
			pclq := r.getPodClique(pgs, &templ, replicaID)

			// Check if the PodClique already exists
			found := &v1alpha1.PodClique{}
			err := r.client.Get(ctx, types.NamespacedName{Name: pclq.Name, Namespace: pclq.Namespace}, found)
			if err != nil {
				if client.IgnoreNotFound(err) == nil {
					r.logger.Info("Creating PodClique", "name", pclq.Name, "namespace", pclq.Namespace)
					if err := r.client.Create(ctx, pclq); err != nil {
						return ctrl.Result{}, err
					}
				} else {
					return ctrl.Result{}, err
				}
			} else {
				// If the PodClique exists, ensure it matches the desired state
				r.logger.Info("Updating PodClique", "name", pclq.Name, "namespace", pclq.Namespace)
				if err := r.client.Update(ctx, found); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}

	// TODO: in case an error in the internal loop, do we need to roll back already created objects?
	return ctrl.Result{}, nil
}

func (r *Reconciler) delete(ctx context.Context, pgs *v1alpha1.PodGangSet) (ctrl.Result, error) {
	// TODO: implement
	r.logger.Info("Deleting PodGangSet", "name", pgs.Name, "namespace", pgs.Namespace)
	err := r.client.Delete(ctx, pgs)

	return ctrl.Result{}, err
}

func (r *Reconciler) getPodClique(pgs *v1alpha1.PodGangSet, templ *v1alpha1.PodCliqueTemplateSpec, replicaID int32) *v1alpha1.PodClique {
	pclq := &v1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%d", templ.Name, replicaID),
			Namespace:   pgs.Namespace,
			Labels:      templ.Labels,
			Annotations: templ.Annotations,
		},
		Spec: v1alpha1.PodCliqueSpec{
			Replicas:    templ.Spec.Replicas,
			PodSpec:     templ.Spec.PodSpec,
			StartsAfter: templ.Spec.StartsAfter,
			ScaleConfig: templ.Spec.ScaleConfig,
		},
	}

	// Add required labels
	if pclq.Labels == nil {
		pclq.Labels = make(map[string]string)
	}
	pclq.Labels[v1alpha1.LabelManagedByKey] = v1alpha1.LabelManagedByValue

	// Set the owner reference and finalizers
	controllerutil.SetControllerReference(pgs, pclq, r.client.Scheme())

	return pclq
}
