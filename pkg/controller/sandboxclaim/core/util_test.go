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
	"testing"
	"time"

	agentsv1alpha1 "github.com/openkruise/agents/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetDesiredReplicas(t *testing.T) {
	tests := []struct {
		name     string
		claim    *agentsv1alpha1.SandboxClaim
		expected int32
	}{
		{
			name: "replicas not set (nil)",
			claim: &agentsv1alpha1.SandboxClaim{
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test",
				},
			},
			expected: DefaultReplicasCount,
		},
		{
			name: "replicas set to 1",
			claim: &agentsv1alpha1.SandboxClaim{
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test",
					Replicas:     int32Ptr(1),
				},
			},
			expected: 1,
		},
		{
			name: "replicas set to 10",
			claim: &agentsv1alpha1.SandboxClaim{
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test",
					Replicas:     int32Ptr(10),
				},
			},
			expected: 10,
		},
		{
			name: "replicas set to 100",
			claim: &agentsv1alpha1.SandboxClaim{
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test",
					Replicas:     int32Ptr(100),
				},
			},
			expected: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDesiredReplicas(tt.claim)
			if got != tt.expected {
				t.Errorf("getDesiredReplicas() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsClaimTimeout(t *testing.T) {
	now := metav1.Now()
	pastTime := metav1.NewTime(now.Add(-10 * time.Second))
	futureTime := metav1.NewTime(now.Add(10 * time.Second)) // For clock skew test

	tests := []struct {
		name     string
		claim    *agentsv1alpha1.SandboxClaim
		status   *agentsv1alpha1.SandboxClaimStatus
		expected bool
	}{
		{
			name: "no timeout set",
			claim: &agentsv1alpha1.SandboxClaim{
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test",
				},
			},
			status: &agentsv1alpha1.SandboxClaimStatus{
				ClaimStartTime: &pastTime,
			},
			expected: false,
		},
		{
			name: "no start time",
			claim: &agentsv1alpha1.SandboxClaim{
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test",
					ClaimTimeout: &metav1.Duration{Duration: 5 * time.Second},
				},
			},
			status:   &agentsv1alpha1.SandboxClaimStatus{},
			expected: false,
		},
		{
			name: "not timed out yet",
			claim: &agentsv1alpha1.SandboxClaim{
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test",
					ClaimTimeout: &metav1.Duration{Duration: 20 * time.Second},
				},
			},
			status: &agentsv1alpha1.SandboxClaimStatus{
				ClaimStartTime: &pastTime,
			},
			expected: false,
		},
		{
			name: "timed out",
			claim: &agentsv1alpha1.SandboxClaim{
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test",
					ClaimTimeout: &metav1.Duration{Duration: 5 * time.Second},
				},
			},
			status: &agentsv1alpha1.SandboxClaimStatus{
				ClaimStartTime: &pastTime,
			},
			expected: true,
		},
		{
			name: "clock skew - start time in future",
			claim: &agentsv1alpha1.SandboxClaim{
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test",
					ClaimTimeout: &metav1.Duration{Duration: 5 * time.Second},
				},
			},
			status: &agentsv1alpha1.SandboxClaimStatus{
				ClaimStartTime: &futureTime,
			},
			expected: false, // Should handle clock skew gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isClaimTimeout(tt.claim, tt.status)
			if got != tt.expected {
				t.Errorf("isClaimTimeout() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsReplicasMet(t *testing.T) {
	tests := []struct {
		name     string
		claim    *agentsv1alpha1.SandboxClaim
		status   *agentsv1alpha1.SandboxClaimStatus
		expected bool
	}{
		{
			name: "replicas met - default 1",
			claim: &agentsv1alpha1.SandboxClaim{
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test",
				},
			},
			status: &agentsv1alpha1.SandboxClaimStatus{
				ClaimedReplicas: 1,
			},
			expected: true,
		},
		{
			name: "replicas not met yet",
			claim: &agentsv1alpha1.SandboxClaim{
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test",
					Replicas:     int32Ptr(10),
				},
			},
			status: &agentsv1alpha1.SandboxClaimStatus{
				ClaimedReplicas: 5,
			},
			expected: false,
		},
		{
			name: "replicas exactly met",
			claim: &agentsv1alpha1.SandboxClaim{
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test",
					Replicas:     int32Ptr(10),
				},
			},
			status: &agentsv1alpha1.SandboxClaimStatus{
				ClaimedReplicas: 10,
			},
			expected: true,
		},
		{
			name: "replicas exceeded (should still be true)",
			claim: &agentsv1alpha1.SandboxClaim{
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test",
					Replicas:     int32Ptr(10),
				},
			},
			status: &agentsv1alpha1.SandboxClaimStatus{
				ClaimedReplicas: 15,
			},
			expected: true,
		},
		{
			name: "zero claimed, zero desired (edge case)",
			claim: &agentsv1alpha1.SandboxClaim{
				Spec: agentsv1alpha1.SandboxClaimSpec{
					TemplateName: "test",
				},
			},
			status: &agentsv1alpha1.SandboxClaimStatus{
				ClaimedReplicas: 0,
			},
			expected: false, // Default is 1, so 0 < 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isReplicasMet(tt.claim, tt.status)
			if got != tt.expected {
				t.Errorf("isReplicasMet() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCalculateClaimStatus(t *testing.T) {
	now := metav1.Now()
	pastTime := metav1.NewTime(now.Add(-10 * time.Second))

	tests := []struct {
		name              string
		args              ClaimArgs
		expectedPhase     agentsv1alpha1.SandboxClaimPhase
		shouldRequeue     bool
		checkCompletedSet bool // Whether CompletionTime should be set
	}{
		{
			name: "initialize new claim",
			args: ClaimArgs{
				Claim: &agentsv1alpha1.SandboxClaim{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: agentsv1alpha1.SandboxClaimSpec{
						TemplateName: "test",
					},
				},
				SandboxSet: &agentsv1alpha1.SandboxSet{},
				NewStatus:  &agentsv1alpha1.SandboxClaimStatus{},
			},
			expectedPhase: agentsv1alpha1.SandboxClaimPhasePending,
			shouldRequeue: false,
		},
		{
			name: "already completed",
			args: ClaimArgs{
				Claim: &agentsv1alpha1.SandboxClaim{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: agentsv1alpha1.SandboxClaimSpec{
						TemplateName: "test",
					},
				},
				SandboxSet: &agentsv1alpha1.SandboxSet{},
				NewStatus: &agentsv1alpha1.SandboxClaimStatus{
					Phase: agentsv1alpha1.SandboxClaimPhaseCompleted,
				},
			},
			expectedPhase: agentsv1alpha1.SandboxClaimPhaseCompleted,
			shouldRequeue: false, // allow EnsureClaimCompleted to run for TTL cleanup
		},
		{
			name: "sandboxset not found",
			args: ClaimArgs{
				Claim: &agentsv1alpha1.SandboxClaim{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: agentsv1alpha1.SandboxClaimSpec{
						TemplateName: "test",
					},
				},
				SandboxSet: nil, // SandboxSet not found
				NewStatus: &agentsv1alpha1.SandboxClaimStatus{
					Phase: agentsv1alpha1.SandboxClaimPhaseClaiming,
				},
			},
			expectedPhase:     agentsv1alpha1.SandboxClaimPhaseCompleted,
			shouldRequeue:     true,
			checkCompletedSet: true,
		},
		{
			name: "claim timeout",
			args: ClaimArgs{
				Claim: &agentsv1alpha1.SandboxClaim{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: agentsv1alpha1.SandboxClaimSpec{
						TemplateName: "test",
						ClaimTimeout: &metav1.Duration{Duration: 5 * time.Second},
					},
				},
				SandboxSet: &agentsv1alpha1.SandboxSet{},
				NewStatus: &agentsv1alpha1.SandboxClaimStatus{
					Phase:          agentsv1alpha1.SandboxClaimPhaseClaiming,
					ClaimStartTime: &pastTime,
				},
			},
			expectedPhase:     agentsv1alpha1.SandboxClaimPhaseCompleted,
			shouldRequeue:     true,
			checkCompletedSet: true,
		},
		{
			name: "replicas met",
			args: ClaimArgs{
				Claim: &agentsv1alpha1.SandboxClaim{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: agentsv1alpha1.SandboxClaimSpec{
						TemplateName: "test",
						Replicas:     int32Ptr(5),
					},
				},
				SandboxSet: &agentsv1alpha1.SandboxSet{},
				NewStatus: &agentsv1alpha1.SandboxClaimStatus{
					Phase:           agentsv1alpha1.SandboxClaimPhaseClaiming,
					ClaimedReplicas: 5,
				},
			},
			expectedPhase:     agentsv1alpha1.SandboxClaimPhaseCompleted,
			shouldRequeue:     true,
			checkCompletedSet: true,
		},
		{
			name: "still claiming",
			args: ClaimArgs{
				Claim: &agentsv1alpha1.SandboxClaim{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: agentsv1alpha1.SandboxClaimSpec{
						TemplateName: "test",
						Replicas:     int32Ptr(10),
					},
				},
				SandboxSet: &agentsv1alpha1.SandboxSet{},
				NewStatus: &agentsv1alpha1.SandboxClaimStatus{
					Phase:           agentsv1alpha1.SandboxClaimPhaseClaiming,
					ClaimedReplicas: 5,
				},
			},
			expectedPhase: agentsv1alpha1.SandboxClaimPhaseClaiming,
			shouldRequeue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotShouldRequeue := CalculateClaimStatus(tt.args)

			if gotStatus.Phase != tt.expectedPhase {
				t.Errorf("CalculateClaimStatus() phase = %v, want %v", gotStatus.Phase, tt.expectedPhase)
			}

			if gotShouldRequeue != tt.shouldRequeue {
				t.Errorf("CalculateClaimStatus() shouldRequeue = %v, want %v", gotShouldRequeue, tt.shouldRequeue)
			}

			if tt.checkCompletedSet && gotStatus.CompletionTime == nil {
				t.Errorf("CalculateClaimStatus() CompletionTime should be set but is nil")
			}

			// Check ObservedGeneration is updated
			if gotStatus.ObservedGeneration != tt.args.Claim.Generation {
				t.Errorf("CalculateClaimStatus() ObservedGeneration = %v, want %v",
					gotStatus.ObservedGeneration, tt.args.Claim.Generation)
			}
		})
	}
}

func TestSetCondition(t *testing.T) {
	now := metav1.Now()
	condition1 := metav1.Condition{
		Type:               "TestCondition",
		Status:             metav1.ConditionTrue,
		Reason:             "TestReason",
		Message:            "Test message",
		LastTransitionTime: now,
	}

	condition2 := metav1.Condition{
		Type:               "AnotherCondition",
		Status:             metav1.ConditionFalse,
		Reason:             "AnotherReason",
		Message:            "Another message",
		LastTransitionTime: now,
	}

	updatedCondition1 := metav1.Condition{
		Type:               "TestCondition",
		Status:             metav1.ConditionFalse,
		Reason:             "UpdatedReason",
		Message:            "Updated message",
		LastTransitionTime: metav1.NewTime(now.Add(1 * time.Hour)),
	}

	tests := []struct {
		name              string
		initialConditions *[]metav1.Condition
		newCondition      metav1.Condition
		expectedLen       int
		checkUpdated      bool
	}{
		{
			name:              "nil conditions slice",
			initialConditions: nil,
			newCondition:      condition1,
			expectedLen:       0, // Should not crash, but also not add
		},
		{
			name:              "add first condition",
			initialConditions: &[]metav1.Condition{},
			newCondition:      condition1,
			expectedLen:       1,
		},
		{
			name:              "add second condition",
			initialConditions: &[]metav1.Condition{condition1},
			newCondition:      condition2,
			expectedLen:       2,
		},
		{
			name:              "update existing condition",
			initialConditions: &[]metav1.Condition{condition1},
			newCondition:      updatedCondition1,
			expectedLen:       1,
			checkUpdated:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setCondition(tt.initialConditions, tt.newCondition)

			if tt.initialConditions == nil {
				return // Expected to handle nil gracefully
			}

			if len(*tt.initialConditions) != tt.expectedLen {
				t.Errorf("setCondition() resulted in %d conditions, want %d",
					len(*tt.initialConditions), tt.expectedLen)
			}

			if tt.checkUpdated && tt.expectedLen > 0 {
				found := false
				for _, c := range *tt.initialConditions {
					if c.Type == tt.newCondition.Type {
						found = true
						if c.Reason != tt.newCondition.Reason {
							t.Errorf("condition not updated: reason = %v, want %v",
								c.Reason, tt.newCondition.Reason)
						}
					}
				}
				if !found {
					t.Errorf("condition type %s not found after update", tt.newCondition.Type)
				}
			}
		})
	}
}

func TestTransitionFunctions(t *testing.T) {
	now := metav1.Now()
	pastTime := metav1.NewTime(now.Add(-10 * time.Second))

	t.Run("transitionToCompleted", func(t *testing.T) {
		status := &agentsv1alpha1.SandboxClaimStatus{
			Phase: agentsv1alpha1.SandboxClaimPhaseClaiming,
		}

		result := transitionToCompleted(status, "TestReason", "Test message")

		if result.Phase != agentsv1alpha1.SandboxClaimPhaseCompleted {
			t.Errorf("transitionToCompleted() phase = %v, want Completed", result.Phase)
		}

		if result.CompletionTime == nil {
			t.Error("transitionToCompleted() CompletionTime should be set")
		}

		if result.Message != "Test message" {
			t.Errorf("transitionToCompleted() message = %v, want 'Test message'", result.Message)
		}
	})

	t.Run("transitionToCompletedWithTimeout", func(t *testing.T) {
		claim := &agentsv1alpha1.SandboxClaim{
			Spec: agentsv1alpha1.SandboxClaimSpec{
				Replicas: int32Ptr(10),
			},
		}
		status := &agentsv1alpha1.SandboxClaimStatus{
			ClaimedReplicas: 5,
		}

		elapsed := 30 * time.Second
		result := transitionToCompletedWithTimeout(status, elapsed, claim)

		if result.Phase != agentsv1alpha1.SandboxClaimPhaseCompleted {
			t.Errorf("transitionToCompletedWithTimeout() phase = %v, want Completed", result.Phase)
		}

		// Check timeout condition is set
		foundTimeout := false
		for _, c := range result.Conditions {
			if c.Type == string(agentsv1alpha1.SandboxClaimConditionTimedOut) {
				foundTimeout = true
				if c.Status != metav1.ConditionTrue {
					t.Error("TimedOut condition should be True")
				}
			}
		}
		if !foundTimeout {
			t.Error("TimedOut condition not found")
		}
	})

	t.Run("transitionToCompletedWithSuccess", func(t *testing.T) {
		claim := &agentsv1alpha1.SandboxClaim{
			Spec: agentsv1alpha1.SandboxClaimSpec{
				Replicas: int32Ptr(10),
			},
		}
		status := &agentsv1alpha1.SandboxClaimStatus{
			ClaimedReplicas: 10,
			ClaimStartTime:  &pastTime,
		}

		result := transitionToCompletedWithSuccess(status, claim)

		if result.Phase != agentsv1alpha1.SandboxClaimPhaseCompleted {
			t.Errorf("transitionToCompletedWithSuccess() phase = %v, want Completed", result.Phase)
		}

		// Check completed condition is set
		foundCompleted := false
		for _, c := range result.Conditions {
			if c.Type == string(agentsv1alpha1.SandboxClaimConditionCompleted) {
				foundCompleted = true
				if c.Status != metav1.ConditionTrue {
					t.Error("Completed condition should be True")
				}
				if c.Reason != "AllReplicasClaimed" {
					t.Errorf("Completed condition reason = %v, want 'AllReplicasClaimed'", c.Reason)
				}
			}
		}
		if !foundCompleted {
			t.Error("Completed condition not found")
		}
	})
}
