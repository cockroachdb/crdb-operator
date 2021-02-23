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
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	api "github.com/cockroachdb/cockroach-operator/api/v1alpha1"
	"github.com/cockroachdb/cockroach-operator/pkg/labels"
	"github.com/cockroachdb/cockroach-operator/pkg/resource"
	"github.com/cockroachdb/cockroach-operator/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"

	"testing"
)

var update = flag.Bool("update", false, "update the golden files of this test")

func TestStatefulSetBuilder(t *testing.T) {
	sc := testutil.InitScheme(t)

	decoder, encoder := testutil.Yamlizers(t, sc)

	inputSuffix := "_in.yaml"
	folder := filepath.Join("testdata", filepath.FromSlash(t.Name()))
	testInputs := filepath.Join(folder, "/*"+inputSuffix)
	clusterFiles, err := filepath.Glob(testInputs)
	if err != nil || len(clusterFiles) == 0 {
		t.Fatalf("failed to find cluster specs %s: %v", testInputs, err)
	}

	for _, inFile := range clusterFiles {
		testName := inFile[len(folder)+1 : len(inFile)-len(inputSuffix)]
		clusterObj := decoder(load(t, inFile))
		cr, ok := clusterObj.(*api.CrdbCluster)
		if !ok {
			t.Fatal("failed to deserialize CrdbCluster")
		}
		commonLabels := labels.Common(cr)
		cluster := resource.NewCluster(cr)

		t.Run(testName, func(t *testing.T) {
			actual := &appsv1.StatefulSet{}

			err := resource.StatefulSetBuilder{
				Cluster:  &cluster,
				Selector: commonLabels.Selector(cr),
			}.Build(actual)
			require.NoError(t, err)

			var buf bytes.Buffer
			err = encoder(actual, &buf)
			require.NoError(t, err)

			expectedStr := testutil.ReadOrUpdateGoldenFile(t, buf.String(), *update)
			expected := decoder([]byte(expectedStr))

			diff := cmp.Diff(expected, actual, testutil.RuntimeObjCmpOpts...)
			if diff != "" {
				assert.Fail(t, fmt.Sprintf("unexpected result (-want +got):\n%v", diff))
			}
		})
	}
}

func TestRHImage(t *testing.T) {
	rhImage := "redhat-coachroach-test:v22"
	os.Setenv(resource.RHEnvVar, rhImage)

	cluster := resource.NewCluster(&api.CrdbCluster{})

	b := resource.StatefulSetBuilder{
		Cluster: &cluster,
	}

	container := b.MakeContainers()

	if container[0].Image != rhImage {
		assert.Fail(t, fmt.Sprintf("unexpected result expected image to equal: %s, got: %s", rhImage, container[0].Image))
	}

	os.Setenv(resource.RHEnvVar, "")
}

func load(t *testing.T, file string) []byte {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("failed to load yaml file %s: %v", file, err)
	}

	return content
}
