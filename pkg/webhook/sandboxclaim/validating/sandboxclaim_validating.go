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

package validating

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	agentsv1alpha1 "github.com/openkruise/agents/api/v1alpha1"
	"github.com/openkruise/agents/pkg/discovery"
	"github.com/openkruise/agents/pkg/features"
	utilfeature "github.com/openkruise/agents/pkg/utils/feature"
)

const (
	webhookPath = "/validate-sandboxclaim"
)

var (
	webhookKind = agentsv1alpha1.GroupVersion.WithKind("SandboxClaim")
)

type SandboxClaimValidatingHandler struct {
	Client  client.Client
	Decoder admission.Decoder
}

// +kubebuilder:webhook:path=/validate-sandboxclaim,mutating=false,failurePolicy=fail,sideEffects=None,admissionReviewVersions=v1;v1beta1,groups=agents.kruise.io,resources=sandboxclaims,verbs=create;update,versions=v1alpha1,name=vsandboxclaim.kb.io

// Path returns the webhook path
func (h *SandboxClaimValidatingHandler) Path() string {
	return webhookPath
}

// Enabled returns whether the webhook is enabled
func (h *SandboxClaimValidatingHandler) Enabled() bool {
	return utilfeature.DefaultFeatureGate.Enabled(features.SandboxClaimGate) && discovery.DiscoverGVK(webhookKind)
}

// Handle handles the admission request
func (h *SandboxClaimValidatingHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := klog.FromContext(ctx).WithValues("webhook", "sandboxclaim-validating", "operation", req.Operation)

	switch req.Operation {
	case admissionv1.Create:
		return h.handleCreate(ctx, req, logger)
	case admissionv1.Update:
		return h.handleUpdate(ctx, req, logger)
	case admissionv1.Delete:
		return admission.Allowed("")
	default:
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unsupported operation: %s", req.Operation))
	}
}

// handleCreate validates SandboxClaim creation
func (h *SandboxClaimValidatingHandler) handleCreate(ctx context.Context, req admission.Request, logger klog.Logger) admission.Response {
	claim := &agentsv1alpha1.SandboxClaim{}
	if err := h.Decoder.Decode(req, claim); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	logger.V(4).Info("Validating SandboxClaim creation", "name", claim.Name, "namespace", claim.Namespace)

	// No special validation for create
	return admission.Allowed("")
}

// handleUpdate validates SandboxClaim update
func (h *SandboxClaimValidatingHandler) handleUpdate(ctx context.Context, req admission.Request, logger klog.Logger) admission.Response {
	oldClaim := &agentsv1alpha1.SandboxClaim{}
	if err := h.Decoder.DecodeRaw(req.OldObject, oldClaim); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to decode old object: %w", err))
	}

	newClaim := &agentsv1alpha1.SandboxClaim{}
	if err := h.Decoder.Decode(req, newClaim); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to decode new object: %w", err))
	}

	logger.V(4).Info("Validating SandboxClaim update", "name", newClaim.Name, "namespace", newClaim.Namespace)

	// Validate Replicas immutability
	oldReplicas := int32(1)
	if oldClaim.Spec.Replicas != nil {
		oldReplicas = *oldClaim.Spec.Replicas
	}

	newReplicas := int32(1)
	if newClaim.Spec.Replicas != nil {
		newReplicas = *newClaim.Spec.Replicas
	}

	if oldReplicas != newReplicas {
		msg := fmt.Sprintf("spec.replicas is immutable, cannot change from %d to %d", oldReplicas, newReplicas)
		logger.Info("Rejecting SandboxClaim update", "reason", msg)
		return admission.Denied(msg)
	}

	return admission.Allowed("")
}
