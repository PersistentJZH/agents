package sandboxutils

import (
	"fmt"
	"strings"
	"sync"
	"time"

	agentsv1alpha1 "github.com/openkruise/agents/api/v1alpha1"
	"github.com/openkruise/agents/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// sandboxStateCache stores cached state and reason for sandboxes
type sandboxStateCacheEntry struct {
	state  string
	reason string
}

var (
	// sandboxStateCache is a global cache for GetSandboxState results
	// Key: namespace/name/resourceVersion (string), Value: cache entry with state and reason (*sandboxStateCacheEntry)
	// NOTE: entries are cleaned up when sandbox is deleted via DeleteSandboxCache
	sandboxStateCache sync.Map
)

// GetSandboxState the state of agentsv1alpha1 Sandbox.
// NOTE: the reason is unique and hard-coded, so we can easily search the conditions of some reason when debugging.
func GetSandboxState(sbx *agentsv1alpha1.Sandbox) (state string, reason string) {

	resourceVersion := sbx.ResourceVersion
	// ResourceVersion is empty, cannot safely cache. Compute and return directly.
	if resourceVersion == "" {
		return computeSandboxState(sbx)
	}

	// Generate cache key: namespace/name/resourceVersion
	cacheKey := fmt.Sprintf("%s/%s/%s", sbx.Namespace, sbx.Name, resourceVersion)

	// Try to read from cache first
	if entry, exists := sandboxStateCache.Load(cacheKey); exists {
		e, ok := entry.(*sandboxStateCacheEntry)
		if !ok {
			// Type assertion failed, cache entry corrupted. Recompute.
			// This should never happen in normal operation, but handle gracefully.
			state, reason = computeSandboxState(sbx)
			return state, reason
		}
		return e.state, e.reason
	}

	// Cache miss, compute the state
	state, reason = computeSandboxState(sbx)

	// Update cache: add new entry
	// Using LoadOrStore to handle concurrent writes: if another goroutine stored first, use its value
	entry := &sandboxStateCacheEntry{
		state:  state,
		reason: reason,
	}
	if actual, loaded := sandboxStateCache.LoadOrStore(cacheKey, entry); loaded {
		// Another goroutine stored first, use the cached value
		e, ok := actual.(*sandboxStateCacheEntry)
		if !ok {
			// Type assertion failed, cache entry corrupted. Use our computed value.
			// This should never happen in normal operation, but handle gracefully.
			return state, reason
		}
		return e.state, e.reason
	}

	// We stored the value, return our computed result
	return state, reason
}

// computeSandboxState computes the sandbox state without caching.
// This is the actual computation logic extracted from the original GetSandboxState.
func computeSandboxState(sbx *agentsv1alpha1.Sandbox) (state string, reason string) {
	if sbx.DeletionTimestamp != nil {
		return agentsv1alpha1.SandboxStateDead, "ResourceDeleted"
	}
	if sbx.Spec.ShutdownTime != nil && time.Since(sbx.Spec.ShutdownTime.Time) > 0 {
		return agentsv1alpha1.SandboxStateDead, "ShutdownTimeReached"
	}
	if sbx.Status.Phase == agentsv1alpha1.SandboxPending {
		return agentsv1alpha1.SandboxStateCreating, "ResourcePending"
	}
	if sbx.Status.Phase == agentsv1alpha1.SandboxSucceeded {
		return agentsv1alpha1.SandboxStateDead, "ResourceSucceeded"
	}
	if sbx.Status.Phase == agentsv1alpha1.SandboxFailed {
		return agentsv1alpha1.SandboxStateDead, "ResourceFailed"
	}
	if sbx.Status.Phase == agentsv1alpha1.SandboxTerminating {
		return agentsv1alpha1.SandboxStateDead, "ResourceTerminating"
	}

	sandboxReady := IsSandboxReady(sbx)
	if IsControlledBySandboxCR(sbx) {
		if sandboxReady {
			return agentsv1alpha1.SandboxStateAvailable, "ResourceControlledBySbsAndReady"
		} else {
			return agentsv1alpha1.SandboxStateCreating, "ResourceControlledBySbsButNotReady"
		}
	} else {
		if sbx.Status.Phase == agentsv1alpha1.SandboxRunning {
			if sbx.Spec.Paused {
				return agentsv1alpha1.SandboxStatePaused, "RunningResourceClaimedAndPaused"
			} else {
				if sandboxReady {
					return agentsv1alpha1.SandboxStateRunning, "RunningResourceClaimedAndReady"
				} else {
					return agentsv1alpha1.SandboxStateDead, "RunningResourceClaimedButNotReady"
				}
			}
		} else {
			// Paused and Resuming phases are both treated as paused state
			return agentsv1alpha1.SandboxStatePaused, "NotRunningResourceClaimed"
		}
	}
}

// DeleteSandboxCache removes all cached entries for a sandbox from the cache.
// This should be called when a sandbox delete event is detected.
func DeleteSandboxCache(namespace, name string) {
	if namespace == "" || name == "" {
		return
	}

	baseKey := fmt.Sprintf("%s/%s/", namespace, name)
	// Delete all entries for this sandbox (all resourceVersions)
	sandboxStateCache.Range(func(key, value interface{}) bool {
		if keyStr, ok := key.(string); ok {
			// Use strings.HasPrefix for clearer and safer prefix matching
			if strings.HasPrefix(keyStr, baseKey) {
				sandboxStateCache.Delete(key)
			}
		}
		return true
	})
}

func IsControlledBySandboxCR(sbx *agentsv1alpha1.Sandbox) bool {
	controller := metav1.GetControllerOfNoCopy(sbx)
	if controller == nil {
		return false
	}
	return controller.Kind == agentsv1alpha1.SandboxSetControllerKind.Kind &&
		// ** REMEMBER TO MODIFY THIS WHEN A NEW API VERSION(LIKE v1beta1) IS ADDED **
		controller.APIVersion == agentsv1alpha1.SandboxSetControllerKind.GroupVersion().String()
}

func GetSandboxID(sbx *agentsv1alpha1.Sandbox) string {
	return fmt.Sprintf("%s--%s", sbx.Namespace, sbx.Name)
}

func IsSandboxReady(sbx *agentsv1alpha1.Sandbox) bool {
	if sbx.Status.PodInfo.PodIP == "" {
		return false
	}
	readyCond := utils.GetSandboxCondition(&sbx.Status, string(agentsv1alpha1.SandboxConditionReady))
	return readyCond != nil && readyCond.Status == metav1.ConditionTrue
}
