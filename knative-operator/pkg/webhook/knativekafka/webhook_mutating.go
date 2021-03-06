package knativekafka

import (
	"context"
	"encoding/json"
	"net/http"

	operatorv1alpha1 "github.com/openshift-knative/serverless-operator/knative-operator/pkg/apis/operator/v1alpha1"
	"github.com/openshift-knative/serverless-operator/knative-operator/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Configurator annotates KKs
type Configurator struct {
	decoder *admission.Decoder
}

// NewConfigurator creates a new Configurator instance to configure KnativeKafka CRs.
func NewConfigurator(decoder *admission.Decoder) *Configurator {
	return &Configurator{
		decoder: decoder,
	}
}

// Implement admission.Handler so the controller can handle admission request.
var _ admission.Handler = (*Configurator)(nil)

// Handle implements the Handler interface.
func (v *Configurator) Handle(ctx context.Context, req admission.Request) admission.Response {
	kk := &operatorv1alpha1.KnativeKafka{}

	err := v.decoder.Decode(req, kk)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	common.MutateKafka(kk)

	marshaled, err := json.Marshal(kk)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.AdmissionRequest.Object.Raw, marshaled)
}
