package common

import "time"

// ComponentSyncRetryInterval is a retry interval with which a reconcile request will be requeued.
const ComponentSyncRetryInterval = 5 * time.Second
