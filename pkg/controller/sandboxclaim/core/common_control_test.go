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

package core

import (
	"context"
	"testing"
	"time"

	agentsv1alpha1 "github.com/openkruise/agents/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCommonControl_EnsureClaimPending(t *testing.T) {
	tests := []struct {
		name          string
		args          ClaimArgs
		expectedPhase agentsv1alpha1.SandboxClaimPhase
		wantErr       bool
	}{
		{
			name: "transition to claiming",
			args: ClaimArgs{
				Claim: &agentsv1alpha1.SandboxClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-claim",
						Namespace: "default",
					},
					Spec: agentsv1alpha1.SandboxClaimSpec{
						TemplateName: "test-sandboxset",
					},
				},
				SandboxSet: &agentsv1alpha1.SandboxSet{},
				NewStatus:  &agentsv1alpha1.SandboxClaimStatus{},
			},
			expectedPhase: agentsv1alpha1.SandboxClaimPhaseClaiming,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = agentsv1alpha1.AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

		fakeRecorder := record.NewFakeRecorder(10)
		control := NewCommonControl(fakeClient, fakeRecorder, nil, nil)

		_, err := control.EnsureClaimPending(context.Background(), tt.args)

		if (err != nil) != tt.wantErr {
			t.Errorf("EnsureClaimPending() error = %v, wantErr %v", err, tt.wantErr)
			return
		}

			if tt.args.NewStatus.Phase != tt.expectedPhase {
				t.Errorf("EnsureClaimPending() phase = %v, want %v", tt.args.NewStatus.Phase, tt.expectedPhase)
			}

			if tt.args.NewStatus.ClaimStartTime == nil {
				t.Error("EnsureClaimPending() ClaimStartTime should be set")
			}
		})
	}
}

func TestCommonControl_CountClaimedSandboxes(t *testing.T) {
	tests := []struct {
		name              string
		existingSandboxes []*agentsv1alpha1.Sandbox
		expectedCount     int32
		wantErr           bool
	}{
		{
			name:              "no sandboxes",
			existingSandboxes: []*agentsv1alpha1.Sandbox{},
			expectedCount:     0,
			wantErr:           false,
		},
		{
			name: "one claimed sandbox",
			existingSandboxes: []*agentsv1alpha1.Sandbox{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sandbox-1",
						Namespace: "default",
					Labels: map[string]string{
						agentsv1alpha1.LabelSandboxClaimedBy:     "test-claim-uid",
						agentsv1alpha1.LabelSandboxClaimedByName: "test-claim",
					},
					},
				},
			},
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name: "multiple claimed sandboxes",
			existingSandboxes: []*agentsv1alpha1.Sandbox{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sandbox-1",
						Namespace: "default",
					Labels: map[string]string{
						agentsv1alpha1.LabelSandboxClaimedBy:     "test-claim-uid",
						agentsv1alpha1.LabelSandboxClaimedByName: "test-claim",
					},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sandbox-2",
						Namespace: "default",
					Labels: map[string]string{
						agentsv1alpha1.LabelSandboxClaimedBy:     "test-claim-uid",
						agentsv1alpha1.LabelSandboxClaimedByName: "test-claim",
					},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sandbox-3",
						Namespace: "default",
					Labels: map[string]string{
						agentsv1alpha1.LabelSandboxClaimedBy:     "test-claim-uid",
						agentsv1alpha1.LabelSandboxClaimedByName: "test-claim",
					},
					},
				},
			},
			expectedCount: 3,
			wantErr:       false,
		},
		{
			name: "mixed sandboxes - only count claimed ones",
			existingSandboxes: []*agentsv1alpha1.Sandbox{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sandbox-claimed",
						Namespace: "default",
					Labels: map[string]string{
						agentsv1alpha1.LabelSandboxClaimedBy:     "test-claim-uid",
						agentsv1alpha1.LabelSandboxClaimedByName: "test-claim",
					},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sandbox-other",
						Namespace: "default",
						Labels: map[string]string{
							agentsv1alpha1.LabelSandboxClaimedBy:     "other-uid",
							agentsv1alpha1.LabelSandboxClaimedByName: "other-claim",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sandbox-unclaimed",
						Namespace: "default",
					},
				},
			},
			expectedCount: 1,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = agentsv1alpha1.AddToScheme(scheme)

			objects := make([]client.Object, len(tt.existingSandboxes))
			for i, sb := range tt.existingSandboxes {
				objects[i] = sb
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			fakeRecorder := record.NewFakeRecorder(10)
			control := NewCommonControl(fakeClient, fakeRecorder, nil, nil).(*commonControl)

			claim := &agentsv1alpha1.SandboxClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "default",
					UID:       "test-claim-uid",
				},
			}

			got, err := control.countClaimedSandboxes(context.Background(), claim)

			if (err != nil) != tt.wantErr {
				t.Errorf("countClaimedSandboxes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.expectedCount {
				t.Errorf("countClaimedSandboxes() = %v, want %v", got, tt.expectedCount)
			}
		})
	}
}

func TestCommonControl_EnsureClaimClaiming(t *testing.T) {
	tests := []struct {
		name              string
		claim             *agentsv1alpha1.SandboxClaim
		existingSandboxes []*agentsv1alpha1.Sandbox
		wantErr           bool
		wantImmediateRequeue bool
	}{
		{
			name: "all replicas already claimed",
			claim: &agentsv1alpha1.SandboxClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "default",
					UID:       "test-claim-uid",
				},
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test-sandboxset",
					Replicas:     int32Ptr(2),
				},
			},
			existingSandboxes: []*agentsv1alpha1.Sandbox{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sandbox-1",
						Namespace: "default",
					Labels: map[string]string{
						agentsv1alpha1.LabelSandboxClaimedBy:     "test-claim-uid",
						agentsv1alpha1.LabelSandboxClaimedByName: "test-claim",
					},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sandbox-2",
						Namespace: "default",
					Labels: map[string]string{
						agentsv1alpha1.LabelSandboxClaimedBy:     "test-claim-uid",
						agentsv1alpha1.LabelSandboxClaimedByName: "test-claim",
					},
					},
				},
			},
			wantErr:              false,
			wantImmediateRequeue: true, // Already completed, should requeue immediately to transition
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = agentsv1alpha1.AddToScheme(scheme)

			objects := make([]client.Object, len(tt.existingSandboxes))
			for i, sb := range tt.existingSandboxes {
				objects[i] = sb
			}

			sandboxSet := &agentsv1alpha1.SandboxSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sandboxset",
					Namespace: "default",
					UID:       "test-sandboxset-uid",
				},
			}
			objects = append(objects, sandboxSet)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			fakeRecorder := record.NewFakeRecorder(100)
			control := NewCommonControl(fakeClient, fakeRecorder, nil, nil)

			args := ClaimArgs{
				Claim:      tt.claim,
				SandboxSet: sandboxSet,
				NewStatus:  &agentsv1alpha1.SandboxClaimStatus{},
			}

			strategy, err := control.EnsureClaimClaiming(context.Background(), args)

			if (err != nil) != tt.wantErr {
				t.Errorf("EnsureClaimClaiming() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if tt.wantImmediateRequeue && !strategy.Immediate {
					t.Errorf("EnsureClaimClaiming() expected immediate requeue, got strategy = %+v", strategy)
				}
			}
		})
	}
}

func TestCommonControl_EnsureClaimCompleted(t *testing.T) {
	now := metav1.Now()

	tests := []struct {
		name            string
		claim           *agentsv1alpha1.SandboxClaim
		status          *agentsv1alpha1.SandboxClaimStatus
		wantErr         bool
		wantNoRequeue   bool
		wantRequeueAfter bool
	}{
		{
			name: "no TTL set",
			claim: &agentsv1alpha1.SandboxClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "default",
				},
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test",
				},
			},
			status: &agentsv1alpha1.SandboxClaimStatus{
				Phase:          agentsv1alpha1.SandboxClaimPhaseCompleted,
				CompletionTime: &now,
			},
			wantErr:          false,
			wantNoRequeue:    true,
			wantRequeueAfter: false,
		},
		{
			name: "TTL not reached yet",
			claim: &agentsv1alpha1.SandboxClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "default",
				},
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName:      "test",
					TTLAfterCompleted: &metav1.Duration{Duration: 1000 * time.Minute}, // 1000 minutes
				},
			},
			status: &agentsv1alpha1.SandboxClaimStatus{
				Phase:          agentsv1alpha1.SandboxClaimPhaseCompleted,
				CompletionTime: &now,
			},
			wantErr:          false,
			wantNoRequeue:    false,
			wantRequeueAfter: true, // Should requeue after remaining time
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = agentsv1alpha1.AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.claim).
				Build()

			fakeRecorder := record.NewFakeRecorder(10)
			control := NewCommonControl(fakeClient, fakeRecorder, nil, nil)

			args := ClaimArgs{
				Claim:      tt.claim,
				SandboxSet: &agentsv1alpha1.SandboxSet{},
				NewStatus:  tt.status,
			}

			strategy, err := control.EnsureClaimCompleted(context.Background(), args)

			if (err != nil) != tt.wantErr {
				t.Errorf("EnsureClaimCompleted() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if tt.wantNoRequeue {
					if strategy.Immediate || strategy.After > 0 {
						t.Errorf("EnsureClaimCompleted() expected NoRequeue, got strategy = %+v", strategy)
					}
				}
				if tt.wantRequeueAfter {
					if strategy.After <= 0 {
						t.Errorf("EnsureClaimCompleted() expected RequeueAfter, got strategy = %+v", strategy)
					}
				}
			}
		})
	}
}

func TestNewCommonControl(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = agentsv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	fakeRecorder := record.NewFakeRecorder(10)

	control := NewCommonControl(fakeClient, fakeRecorder, nil, nil)

	if control == nil {
		t.Error("NewCommonControl() returned nil")
	}

	// Check it implements the interface
	var _ ClaimControl = control
}

// Helper function
func int32Ptr(i int32) *int32 {
	return &i
}
