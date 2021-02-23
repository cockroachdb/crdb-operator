/*
Copyright 2021 The Cockroach Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package resource

import (
	"errors"

	"github.com/cockroachdb/cockroach-operator/pkg/labels"
	policy "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// PdbBuilder models the PodDistruptionBudget that the
// operator maintains.
type PdbBuilder struct {
	*Cluster

	Selector map[string]string
}

// Build creates a policy.PodDisruptionBudget for the
// StatefulSet.
func (b PdbBuilder) Build(obj runtime.Object) error {
	pdb, ok := obj.(*policy.PodDisruptionBudget)
	if !ok {
		return errors.New("failed to cast to PDB object")
	}

	if pdb.ObjectMeta.Name == "" {
		pdb.ObjectMeta.Name = b.DiscoveryServiceName()
	}

	if pdb.ObjectMeta.Labels == nil {
		pdb.ObjectMeta.Labels = map[string]string{}
	}

	// Using the Common labels that will match
	// the statefulset
	commonLabels := labels.Common(b.Cluster.cr)
	selector := commonLabels.Selector(b.Cluster.cr)
	pdb.Spec = policy.PodDisruptionBudgetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: selector,
		},
	}

	// TODO should we always create a PDB?
	// Open question here:
	// https://github.com/cockroachdb/cockroach-operator/issues/79

	// Setup MinAvailable
	if b.Cluster.cr.Spec.MinAvailable != nil {
		minAvailable := b.Cluster.cr.Spec.MinAvailable
		minAvailableIS := intstr.FromInt(int(*minAvailable))
		pdb.Spec.MinAvailable = &minAvailableIS
	} else {
		// Set MaxUnavailbe or use the default value
		maxUnavailable := b.Cluster.cr.Spec.MaxUnavailable
		maxUnavailableIS := intstr.FromInt(int(*maxUnavailable))
		pdb.Spec.MaxUnavailable = &maxUnavailableIS
	}

	return nil
}

// TODO update func command - what does this do???

func (b PdbBuilder) Placeholder() runtime.Object {
	return &policy.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name: b.DiscoveryServiceName(),
		},
	}
}
