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

package actor

import (
	"context"

	api "github.com/cockroachdb/cockroach-operator/api/v1alpha1"
	"github.com/cockroachdb/cockroach-operator/pkg/condition"
	"github.com/cockroachdb/cockroach-operator/pkg/kube"
	"github.com/cockroachdb/cockroach-operator/pkg/resource"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newDeploy(scheme *runtime.Scheme, cl client.Client) Actor {
	return &deploy{
		action: newAction("deploy", scheme, cl),
	}
}

// deploy initializes and reconciles the Kubernetes resources needed by the CockroachDB cluster:
// services, a statefulset and a pod disruption budget
type deploy struct {
	action
}

func (d deploy) Handles(conds []api.ClusterCondition) bool {
	return condition.False(api.NotInitializedCondition, conds) || condition.True(api.NotInitializedCondition, conds)
}

func (d deploy) Act(ctx context.Context, cluster *resource.Cluster) error {
	log := d.log.WithValues("CrdbCluster", cluster.ObjectKey())
	log.Info("reconciling resources")

	r := resource.NewManagedKubeResource(ctx, d.client, cluster, kube.AnnotatingPersister)

	owner := cluster.Unwrap()

	changed, err := (resource.Reconciler{
		ManagedResource: r,
		Builder: resource.DiscoveryServiceBuilder{
			Cluster:  cluster,
			Selector: r.Labels.Selector(cluster.Unwrap()),
		},
		Owner:  owner,
		Scheme: d.scheme,
	}).Reconcile()
	if err != nil {
		return errors.Wrap(err, "failed to reconcile discovery service")
	}

	if changed {
		log.Info("created/updated discovery service, stopping request processing")
		CancelLoop(ctx)
		return nil
	}

	changed, err = (resource.Reconciler{
		ManagedResource: r,
		Builder: resource.PublicServiceBuilder{
			Cluster:  cluster,
			Selector: r.Labels.Selector(cluster.Unwrap()),
		},
		Owner:  owner,
		Scheme: d.scheme,
	}).Reconcile()
	if err != nil {
		return errors.Wrap(err, "failed to reconcile public service")
	}

	if changed {
		log.Info("created/updated public service, stopping request processing")
		CancelLoop(ctx)
		return nil
	}

	changed, err = (resource.Reconciler{
		ManagedResource: r,
		Builder: resource.StatefulSetBuilder{
			Cluster:  cluster,
			Selector: r.Labels.Selector(cluster.Unwrap()),
		},
		Owner:  owner,
		Scheme: d.scheme,
	}).Reconcile()
	if err != nil {
		return errors.Wrap(err, "failed to reconcile statefulset")
	}

	if changed {
		log.Info("created/updated statefulset, stopping request processing")
		CancelLoop(ctx)
		return nil
	}

	// if we only have one Node we cannot have a PDB
	// TODO we need to validate this in the CRD API
	if cluster.Spec().Nodes > 1 {
		changed, err = (resource.Reconciler{
			ManagedResource: r,
			Builder: resource.PdbBuilder{
				Cluster:  cluster,
				Selector: r.Labels.Selector(cluster.Unwrap()),
			},
			Owner:  owner,
			Scheme: d.scheme,
		}).Reconcile()
		if err != nil {
			return errors.Wrap(err, "failed to reconcile pdb")
		}

		if changed {
			log.Info("created/updated pdb, stopping request processing")
			CancelLoop(ctx)
			return nil
		}
	}

	log.Info("completed")
	return nil
}
