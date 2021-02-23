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
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach-operator/pkg/labels"
	"github.com/cockroachdb/cockroach-operator/pkg/resource"
	"github.com/cockroachdb/cockroach-operator/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPublicServiceBuilder(t *testing.T) {
	cluster := testutil.NewBuilder("test-cluster").Namespaced("test-ns")
	commonLabels := labels.Common(cluster.Cr())

	tests := []struct {
		name     string
		cluster  *resource.Cluster
		selector map[string]string
		expected *corev1.Service
	}{
		{
			name:     "builds default public service",
			cluster:  cluster.Cluster(),
			selector: commonLabels.Selector(cluster.Cr()),
			expected: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-cluster-public",
					Labels: map[string]string{},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
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
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := &corev1.Service{}

			err := resource.PublicServiceBuilder{
				Cluster:  tt.cluster,
				Selector: tt.selector,
			}.Build(actual)
			require.NoError(t, err)

			diff := cmp.Diff(tt.expected, actual, testutil.RuntimeObjCmpOpts...)
			if diff != "" {
				assert.Fail(t, fmt.Sprintf("unexpected result (-want +got):\n%v", diff))
			}
		})
	}
}
