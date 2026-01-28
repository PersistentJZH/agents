/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sandboxclaim

import (
	"context"
	"testing"
	"time"

	agentsv1alpha1 "github.com/openkruise/agents/api/v1alpha1"
	"github.com/openkruise/agents/pkg/controller/sandboxclaim/core"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconciler_Reconcile_BasicFlow(t *testing.T) {
	tests := []struct {
		name              string
		claim             *agentsv1alpha1.SandboxClaim
		sandboxSet        *agentsv1alpha1.SandboxSet
		existingSandboxes []*agentsv1alpha1.Sandbox
		expectedPhase     agentsv1alpha1.SandboxClaimPhase
		wantErr           bool
		wantRequeue       bool
	}{
		{
			name: "claim not found",
			claim: &agentsv1alpha1.SandboxClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nonexistent-claim",
					Namespace: "default",
				},
			},
			expectedPhase: "",
			wantErr:       false,
			wantRequeue:   false,
		},
		{
			name: "new claim - pending phase",
			claim: &agentsv1alpha1.SandboxClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-claim",
					Namespace:  "default",
					Generation: 1,
				},
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test-sandboxset",
				},
			},
			sandboxSet: &agentsv1alpha1.SandboxSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sandboxset",
					Namespace: "default",
				},
			},
			expectedPhase: agentsv1alpha1.SandboxClaimPhaseClaiming,
			wantErr:       false,
			wantRequeue:   true, // EnsureClaimPending returns RequeueImmediately
		},
		{
			name: "sandboxset not found",
			claim: &agentsv1alpha1.SandboxClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-claim",
					Namespace:  "default",
					Generation: 1,
				},
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "nonexistent-sandboxset",
				},
			},
			sandboxSet:    nil, // SandboxSet doesn't exist
			expectedPhase: agentsv1alpha1.SandboxClaimPhaseCompleted,
			wantErr:       false,
			wantRequeue:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = agentsv1alpha1.AddToScheme(scheme)

			objects := []client.Object{}
			if tt.name != "claim not found" {
				objects = append(objects, tt.claim)
			}
			if tt.sandboxSet != nil {
				objects = append(objects, tt.sandboxSet)
			}
			for _, sb := range tt.existingSandboxes {
				objects = append(objects, sb)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				WithStatusSubresource(&agentsv1alpha1.SandboxClaim{}).
				Build()

			fakeRecorder := record.NewFakeRecorder(100)

			reconciler := &Reconciler{
				Client:   fakeClient,
				Scheme:   scheme,
				controls: core.NewClaimControl(fakeClient, fakeRecorder, nil, nil),
			recorder: fakeRecorder,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.claim.Name,
					Namespace: tt.claim.Namespace,
				},
			}

			result, err := reconciler.Reconcile(context.Background(), req)

			if (err != nil) != tt.wantErr {
				t.Errorf("Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantRequeue != (result.RequeueAfter > 0 || result.Requeue) {
				t.Errorf("Reconcile() requeue = %v, wantRequeue %v", result, tt.wantRequeue)
			}

			// Verify the claim status if it exists
			if tt.expectedPhase != "" && tt.name != "claim not found" {
				updatedClaim := &agentsv1alpha1.SandboxClaim{}
				err := fakeClient.Get(context.Background(),
					types.NamespacedName{Name: tt.claim.Name, Namespace: tt.claim.Namespace},
					updatedClaim)

				if err != nil {
					t.Fatalf("Failed to get updated claim: %v", err)
				}

				if updatedClaim.Status.Phase != tt.expectedPhase {
					t.Errorf("Reconcile() phase = %v, want %v", updatedClaim.Status.Phase, tt.expectedPhase)
				}
			}
		})
	}
}

func TestReconciler_Reconcile_Claiming(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = agentsv1alpha1.AddToScheme(scheme)

	claim := &agentsv1alpha1.SandboxClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-claim",
			Namespace:  "default",
			UID:        "test-uid",
			Generation: 1,
		},
		Spec: agentsv1alpha1.SandboxClaimSpec{
			TemplateName: "test-sandboxset",
			Replicas:     int32Ptr(2),
		},
	}

	sandboxSet := &agentsv1alpha1.SandboxSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sandboxset",
			Namespace: "default",
		},
	}

	sandbox1 := &agentsv1alpha1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sandbox-1",
			Namespace: "default",
			Labels: map[string]string{
				"sandboxset": "test-sandboxset",
			},
		},
	}

	sandbox2 := &agentsv1alpha1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sandbox-2",
			Namespace: "default",
			Labels: map[string]string{
				"sandboxset": "test-sandboxset",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(claim, sandboxSet, sandbox1, sandbox2).
		WithStatusSubresource(&agentsv1alpha1.SandboxClaim{}).
		Build()

	fakeRecorder := record.NewFakeRecorder(100)

	reconciler := &Reconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		controls: core.NewClaimControl(fakeClient, fakeRecorder, nil, nil),
		recorder: fakeRecorder,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      claim.Name,
			Namespace: claim.Namespace,
		},
	}

	// First reconcile - should transition to Claiming
	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("First Reconcile() error = %v", err)
	}

	// Note: requeueAfter = 0 means immediate requeue, but appears as both false
	// The controller will still reconcile immediately

	// Get updated claim
	updatedClaim := &agentsv1alpha1.SandboxClaim{}
	err = fakeClient.Get(context.Background(),
		types.NamespacedName{Name: claim.Name, Namespace: claim.Namespace},
		updatedClaim)

	if err != nil {
		t.Fatalf("Failed to get updated claim: %v", err)
	}

	if updatedClaim.Status.Phase != agentsv1alpha1.SandboxClaimPhaseClaiming {
		t.Errorf("After first reconcile, phase = %v, want Claiming", updatedClaim.Status.Phase)
	}
}

func TestReconciler_Reconcile_ConditionalRequeue(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = agentsv1alpha1.AddToScheme(scheme)

	t.Run("requeue immediately when sandboxes claimed", func(t *testing.T) {
		// Skip: This test requires cache and sandboxClient to be initialized,
		// which is only available in e2e/integration tests
		t.Skip("Requires cache initialization - tested in e2e tests")
	})

	t.Run("requeue with delay when no sandboxes available", func(t *testing.T) {
		// Skip: This test requires cache and sandboxClient to be initialized,
		// which is only available in e2e/integration tests
		t.Skip("Requires cache initialization - tested in e2e tests")
	})
}

func TestReconciler_Reconcile_Timeout(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = agentsv1alpha1.AddToScheme(scheme)

	claim := &agentsv1alpha1.SandboxClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-claim",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: agentsv1alpha1.SandboxClaimSpec{
			TemplateName: "test-sandboxset",
			Replicas:     int32Ptr(10),
			ClaimTimeout: &metav1.Duration{Duration: 1 * time.Second}, // Very short timeout
		},
		Status: agentsv1alpha1.SandboxClaimStatus{
			Phase: agentsv1alpha1.SandboxClaimPhaseClaiming,
			ClaimStartTime: &metav1.Time{
				Time: time.Now().Add(-5 * time.Second), // Started 5 seconds ago
			},
			ClaimedReplicas: 3, // Only claimed 3 out of 10
		},
	}

	sandboxSet := &agentsv1alpha1.SandboxSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sandboxset",
			Namespace: "default",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(claim, sandboxSet).
		WithStatusSubresource(&agentsv1alpha1.SandboxClaim{}).
		Build()

	fakeRecorder := record.NewFakeRecorder(100)

	reconciler := &Reconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		controls: core.NewClaimControl(fakeClient, fakeRecorder, nil, nil),
		recorder: fakeRecorder,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      claim.Name,
			Namespace: claim.Namespace,
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Get updated claim
	updatedClaim := &agentsv1alpha1.SandboxClaim{}
	err = fakeClient.Get(context.Background(),
		types.NamespacedName{Name: claim.Name, Namespace: claim.Namespace},
		updatedClaim)

	if err != nil {
		t.Fatalf("Failed to get updated claim: %v", err)
	}

	// Should transition to Completed due to timeout
	if updatedClaim.Status.Phase != agentsv1alpha1.SandboxClaimPhaseCompleted {
		t.Errorf("After timeout, phase = %v, want Completed", updatedClaim.Status.Phase)
	}

	// Should have CompletionTime set
	if updatedClaim.Status.CompletionTime == nil {
		t.Error("CompletionTime should be set after timeout")
	}

	// Should not requeue
	if result.Requeue || result.RequeueAfter > 0 {
		t.Errorf("Should not requeue after completion, got %v", result)
	}
}

func TestReconciler_GetControl(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = agentsv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	fakeRecorder := record.NewFakeRecorder(10)

	reconciler := &Reconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		controls: core.NewClaimControl(fakeClient, fakeRecorder, nil, nil),
		recorder: fakeRecorder,
	}

	control := reconciler.getControl()
	if control == nil {
		t.Error("getControl() returned nil")
	}
}

// TestReconciler_SetupWithManager tests the setup function
// Note: This is a basic test. Full integration testing would require a real Manager.
func TestReconciler_SetupWithManager(t *testing.T) {
	// Skip this test as it requires a full Manager implementation
	// which is better tested in integration tests
	t.Skip("Requires full Manager implementation - tested in e2e tests")
}

// Helper functions
func int32Ptr(i int32) *int32 {
	return &i
}
