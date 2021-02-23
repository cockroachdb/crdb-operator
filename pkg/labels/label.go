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

package labels

import (
	api "github.com/cockroachdb/cockroach-operator/api/v1alpha1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

// https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
const (
	// The name of a higher level application this one is part of
	NameKey = "app.kubernetes.io/name"
	// A unique name identifying the instance of an application
	InstanceKey = "app.kubernetes.io/instance"
	// The current version of the application
	VersionKey = "app.kubernetes.io/version"
	// The component within the architecture
	ComponentKey = "app.kubernetes.io/component"
	// The name of a higher level application this one is part of
	PartOfKey = "app.kubernetes.io/part-of"
	// The tool being used to manage the operation of an application
	ManagedByKey = "app.kubernetes.io/managed-by"
)

var managed = []string{NameKey, InstanceKey, VersionKey, ComponentKey, PartOfKey, ManagedByKey}

func Common(cluster *api.CrdbCluster) Labels {
	ll := Labels{}

	ll.Merge(makeCommonLabels(cluster.Labels, cluster.Name, cluster.Status.Version))

	ll.Merge(cluster.Spec.Labels)

	return ll
}

func FromObject(obj runtime.Object) (Labels, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, errors.Wrap(err, "unable to access labels")
	}

	ll := Labels{}
	ll.Merge(accessor.GetLabels())

	return ll, nil
}

func Update(ll, update Labels) {
	for k, v := range update {
		ll[k] = v
	}

	for _, k := range managed {
		_, inLl := ll[k]
		_, inUpdate := update[k]
		if inLl && !inUpdate {
			delete(ll, k)
		}
	}
}

type Labels map[string]string

func (ll Labels) Merge(other map[string]string) {
	if other == nil {
		return
	}

	for k, v := range other {
		ll[k] = v
	}
}

func (ll Labels) ApplyTo(obj runtime.Object) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return errors.Wrap(err, "unable to access labels")
	}

	accessor.SetLabels(ll)

	return nil
}

func (ll Labels) Selector(cluster *api.CrdbCluster) map[string]string {
	selector := make(map[string]string)

	for _, k := range []string{NameKey, InstanceKey, ComponentKey} {
		if _, ok := ll[k]; ok {
			selector[k] = ll[k]
		}
	}
	ll.Merge(cluster.Spec.Labels)

	return selector
}

func (ll Labels) Copy() Labels {
	res := Labels{}
	for k, v := range ll {
		res[k] = v
	}

	return res
}

func (ll Labels) AsMap() map[string]string {
	return ll
}

func makeCommonLabels(other map[string]string, instance string, version string) map[string]string {
	common := make(map[string]string)

	// keep part-of customized if it was set by high-level app
	var found bool
	if common[PartOfKey], found = other[PartOfKey]; !found {
		common[PartOfKey] = "cockroachdb"
	}

	common[NameKey] = "cockroachdb"
	common[InstanceKey] = instance
	if version != "" {
		common[VersionKey] = version
	}

	common[ComponentKey] = "database"
	common[ManagedByKey] = "cockroach-operator"

	return common
}
