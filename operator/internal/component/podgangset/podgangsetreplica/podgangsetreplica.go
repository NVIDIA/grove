// /*
// Copyright 2025 The Grove Authors.
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

package podgangsetreplica

import (
	"context"
	"fmt"
	"maps"
	"slices"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errCodeComputingPGSReplicaDeletionWork grovecorev1alpha1.ErrorCode = "ERR_COMPUTE_PGS_REPLICA_DELETION_WORK"
	errCodeDeletePGSReplica                grovecorev1alpha1.ErrorCode = "ERR_DELETE_PGS_REPLICA"
	errCodeListPCLQs                       grovecorev1alpha1.ErrorCode = "ERR_LIST_PCLQs"
	errCodeListPCSGs                       grovecorev1alpha1.ErrorCode = "ERR_LIST_PCGS"
	errCodeUpdatePGSStatus                 grovecorev1alpha1.ErrorCode = "ERR_UPDATE_PGS_STATUS"
)

type _resource struct {
	client        client.Client
	eventRecorder record.EventRecorder
}

// New creates a new instance of the PodGangSetReplica operator.
func New(client client.Client, eventRecorder record.EventRecorder) component.Operator[grovecorev1alpha1.PodCliqueSet] {
	return &_resource{
		client:        client,
		eventRecorder: eventRecorder,
	}
}

// GetExistingResourceNames is a no-op.
func (r _resource) GetExistingResourceNames(_ context.Context, _ logr.Logger, _ metav1.ObjectMeta) ([]string, error) {
	return []string{}, nil
}

func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet) error {
	pgsObjectKey := client.ObjectKeyFromObject(pgs)

	delWork, err := r.getPGSReplicaDeletionWork(ctx, logger, pgs)
	if err != nil {
		return groveerr.WrapError(err,
			errCodeComputingPGSReplicaDeletionWork,
			component.OperationSync,
			fmt.Sprintf("Could not compute pending replica deletion delWork for PGS: %v", pgsObjectKey))
	}

	if delWork.hasPendingPGSReplicaDeletion() {
		if runResult := utils.RunConcurrently(ctx, logger, delWork.deletionTasks); runResult.HasErrors() {
			return groveerr.WrapError(runResult.GetAggregatedError(),
				errCodeDeletePGSReplica,
				component.OperationSync,
				fmt.Sprintf("Error deleting PodCliques for PodCliqueSet: %v", pgsObjectKey),
			)
		}
	}

	if isRollingUpdateInProgress(pgs) {
		minAvailableBreachedPGSReplicaIndices := slices.Collect(maps.Keys(delWork.minAvailableBreachedConstituents))
		if err := r.orchestrateRollingUpdate(ctx, logger, pgs, delWork.pgsIndicesToTerminate, minAvailableBreachedPGSReplicaIndices); err != nil {
			return err
		}
	}

	if delWork.shouldRequeue() {
		return groveerr.New(groveerr.ErrCodeContinueReconcileAndRequeue,
			component.OperationSync,
			"Requeuing to re-process PGS replicas that have breached MinAvailable but not crossed TerminationDelay",
		)
	}
	return nil
}

// Delete is a no-op
func (r _resource) Delete(_ context.Context, _ logr.Logger, _ metav1.ObjectMeta) error {
	return nil
}
