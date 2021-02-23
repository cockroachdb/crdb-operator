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

package resource_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach-operator/pkg/kube"
	"github.com/cockroachdb/cockroach-operator/pkg/labels"
	"github.com/cockroachdb/cockroach-operator/pkg/ptr"
	"github.com/cockroachdb/cockroach-operator/pkg/resource"
	"github.com/cockroachdb/cockroach-operator/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	amtypes "k8s.io/apimachinery/pkg/types"
)

func TestReconcile(t *testing.T) {
	ctx := context.TODO()
	scheme := testutil.InitScheme(t)

	tests := []struct {
		name         string
		cluster      *resource.Cluster
		existingObjs []runtime.Object
		wantUpserted bool
		expected     *corev1.Service
	}{
		{
			name: "creates object when it is missing",
			cluster: testutil.NewBuilder("test-cluster").Namespaced("default").
				WithUID("test-cluster-uid").Cluster(),
			existingObjs: []runtime.Object{},
			wantUpserted: true,
			expected:     makeTestService(),
		},
		{
			name: "updates object when its spec is different",
			cluster: testutil.NewBuilder("test-cluster").Namespaced("default").
				WithUID("test-cluster-uid").WithHTTPPort(8443).Cluster(),
			existingObjs: []runtime.Object{makeTestService()},
			wantUpserted: true,
			expected:     modifyHTTPPort(8443, makeTestService()),
		},
		{
			name: "does not touch existing object if it has the same configuration",
			cluster: testutil.NewBuilder("test-cluster").Namespaced("default").
				WithUID("test-cluster-uid").Cluster(),
			existingObjs: []runtime.Object{makeTestService()},
			wantUpserted: false,
			expected:     makeTestService(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commonLabels := labels.Common(tt.cluster.Unwrap())

			builder := resource.DiscoveryServiceBuilder{
				Cluster:  tt.cluster,
				Selector: commonLabels.Selector(tt.cluster.Unwrap()),
			}

			client := testutil.NewFakeClient(scheme, tt.existingObjs...)

			r := resource.Reconciler{
				ManagedResource: resource.NewManagedKubeResource(ctx, client, tt.cluster, kube.AnnotatingPersister),
				Builder:         builder,
				Owner:           tt.cluster.Unwrap(),
				Scheme:          scheme,
			}

			upserted, err := r.Reconcile()
			require.NoError(t, err)

			assert.Equal(t, tt.wantUpserted, upserted)

			// TODO: change to placeholder?
			actual := &corev1.Service{}
			assert.NoError(t, client.Get(ctx, amtypes.NamespacedName{Name: "test-cluster", Namespace: "default"}, actual))

			stripOutLastAppliedAnnotation(tt.expected.Annotations)
			stripOutLastAppliedAnnotation(actual.Annotations)

			diff := cmp.Diff(tt.expected, actual, testutil.RuntimeObjCmpOpts...)
			if diff != "" {
				assert.Fail(t, fmt.Sprintf("unexpected result (-want +got):\n%v", diff))
			}
		})
	}
}

func makeTestService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/component":  "database",
				"app.kubernetes.io/instance":   "test-cluster",
				"app.kubernetes.io/managed-by": "cockroach-operator",
				"app.kubernetes.io/name":       "cockroachdb",
				"app.kubernetes.io/part-of":    "cockroachdb",
			},
			Annotations: map[string]string{
				"prometheus.io/scrape": "true",
				"prometheus.io/path":   "_status/vars",
				"prometheus.io/port":   "8080",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "crdb.cockroachlabs.com/v1alpha1",
					Kind:               "CrdbCluster",
					Name:               "test-cluster",
					UID:                "test-cluster-uid",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				},
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP:                "None",
			PublishNotReadyAddresses: true,
			Ports: []corev1.ServicePort{
				{Name: "grpc", Port: 26257},
				{Name: "http", Port: 8080},
			},
			Selector: map[string]string{
				"app.kubernetes.io/name":      "cockroachdb",
				"app.kubernetes.io/instance":  "test-cluster",
				"app.kubernetes.io/component": "database",
			},
		},
	}
}

func modifyHTTPPort(newValue int32, service *corev1.Service) *corev1.Service {
	service.Spec.Ports = []corev1.ServicePort{
		{Name: "grpc", Port: 26257},
		{Name: "http", Port: newValue},
	}
	service.ObjectMeta.Annotations = map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/path":   "_status/vars",
		"prometheus.io/port":   fmt.Sprintf("%d", newValue),
	}

	return service
}

func stripOutLastAppliedAnnotation(aa map[string]string) {
	delete(aa, kube.LastAppliedAnnotation)
}
