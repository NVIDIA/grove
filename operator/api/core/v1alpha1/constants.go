package v1alpha1

// Common label keys to be placed on all resources managed by grove operator.
const (
	// LabelAppNameKey is a key of a label which sets the name of the resource.
	LabelAppNameKey = "app.kubernetes.io/name"
	// LabelManagedByKey is a key of a label which sets the operator which manages this resource.
	LabelManagedByKey = "app.kubernetes.io/managed-by"
	// LabelPartOfKey is a key of a label which sets the type of the resource.
	LabelPartOfKey = "app.kubernetes.io/part-of"
	// LabelManagedByValue is the value for LabelManagedByKey
	LabelManagedByValue = "grove-operator"
	// LabelComponentKey is a key for a label that sets the component type on resources provisioned for a PodGangSet.
	LabelComponentKey = "app.kubernetes.io/component"
	// LabelPodGangNameKey is a key for a label that sets the name of the PodGang on resources provisioned for a PodGang.
	LabelPodGangNameKey = "grove.io/podgang-name"
)

// Constants for finalizers.
const (
	// FinalizerPodGangSet is the finalizer for PodGangSet that is added to `.metadata.finalizers[]` slice. This will be placed on all PodGangSet resources
	// during reconciliation. This finalizer is used to clean up resources that are created for a PodGangSet when it is deleted.
	FinalizerPodGangSet = "podgangset.grove.io"
	// FinalizerPodClique is the finalizer for PodClique that is added to `.metadata.finalizers[]` slice. This will be placed on all PodClique resources
	// during reconciliation. This finalizer is used to clean up resources that are created for a PodClique when it is deleted.
	FinalizerPodClique = "podclique.grove.io"
)

// Constants for events.
const (
	// EventReconciling is the event type which indicates that the reconcile operation has started.
	EventReconciling = "Reconciling"
	// EventReconciled is the event type which indicates that the reconcile operation has completed successfully.
	EventReconciled = "Reconciled"
	// EventReconcileError is the event type which indicates that the reconcile operation has failed.
	EventReconcileError = "ReconcileError"
	// EventDeleting is the event type which indicates that the delete operation has started.
	EventDeleting = "Deleting"
	// EventDeleted is the event type which indicates that the delete operation has completed successfully.
	EventDeleted = "Deleted"
	// EventDeleteError is the event type which indicates that the delete operation has failed.
	EventDeleteError = "DeleteError"
)
