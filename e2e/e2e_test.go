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

package e2e

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	api "github.com/cockroachdb/cockroach-operator/api/v1alpha1"
	"github.com/cockroachdb/cockroach-operator/pkg/actor"
	"github.com/cockroachdb/cockroach-operator/pkg/controller"
	"github.com/cockroachdb/cockroach-operator/pkg/testutil"
	testenv "github.com/cockroachdb/cockroach-operator/pkg/testutil/env"
	"github.com/cockroachdb/cockroach-operator/pkg/testutil/exec"
	"github.com/cockroachdb/cockroach-operator/pkg/testutil/paths"
	"github.com/cockroachdb/cockroach-operator/pkg/utilfeature"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"k8s.io/apimachinery/pkg/runtime"
)

var updateOpt = flag.Bool("update", false, "update the golden files of this test")

var env *testenv.ActiveEnv

type Step struct {
	name string
	test func(t *testing.T)
}

type Steps []Step

func (ss Steps) WithStep(s Step) Steps {
	return append(ss, s)
}

func (ss Steps) Run(t *testing.T) {
	for _, s := range ss {
		if !t.Run(s.name, s.test) {
			t.FailNow()
		}
	}
}

// TestMain wraps the unit tests. Set TEST_DO_NOT_USE_KIND evnvironment variable to any value
// if you do not want this test to start a k8s cluster using kind.
func TestMain(m *testing.M) {
	flag.Parse()

	// We are running in bazel so set up the directory for the test binaries
	if os.Getenv("TEST_WORKSPACE") != "" {
		// TODO create a toolchain for this
		paths.MaybeSetEnv("PATH", "kubetest2-kind", "hack", "bin", "kubetest2-kind")
	}

	noKind := os.Getenv("TEST_DO_NOT_USE_KIND")
	if noKind == "" {
		os.Setenv("USE_EXISTING_CLUSTER", "true")

		// TODO random name for server and also random open port
		err := exec.StartKubeTest2("test")
		if err != nil {
			panic(err)
		}
	}

	// TODO verify success of cluster start? Does kind do it?

	e := testenv.NewEnv(runtime.NewSchemeBuilder(api.AddToScheme),
		filepath.Join("..", "config", "crd", "bases"),
		filepath.Join("..", "config", "rbac", "bases"))

	env = e.Start()
	code := m.Run()
	e.Stop()

	if noKind == "" {
		err := exec.StopKubeTest2("test")
		if err != nil {
			panic(err)
		}
	}
	os.Exit(code)
}
func TestCreatesInsecureCluster(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	testLog := zapr.NewLogger(zaptest.NewLogger(t))

	actor.Log = testLog

	sb := testenv.NewDiffingSandbox(t, env)
	sb.StartManager(t, controller.InitClusterReconcilerWithLogger(testLog))

	b := testutil.NewBuilder("crdb").WithNodeCount(3).WithEmptyDirDataStore().
		WithLabels(map[string]string{"customlabel": "Test"})

	create := Step{
		name: "creates 3-node insecure cluster",
		test: func(t *testing.T) {
			require.NoError(t, sb.Create(b.Cr()))

			requireClusterToBeReadyEventually(t, sb, b)

			state, err := sb.Diff()
			require.NoError(t, err)

			expected := testutil.ReadOrUpdateGoldenFile(t, state, *updateOpt)

			testutil.AssertDiff(t, expected, state)
		},
	}

	steps := Steps{create}

	steps.Run(t)
}

func TestCreatesSecureClusterWithGeneratedCert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	testLog := zapr.NewLogger(zaptest.NewLogger(t))

	actor.Log = testLog

	sb := testenv.NewDiffingSandbox(t, env)
	sb.StartManager(t, controller.InitClusterReconcilerWithLogger(testLog))

	b := testutil.NewBuilder("crdb").WithNodeCount(1).WithTLS().WithEmptyDirDataStore().
		WithLabels(map[string]string{"customlabel": "Test"})

	create := Step{
		name: "creates 1-node secure cluster",
		test: func(t *testing.T) {
			require.NoError(t, sb.Create(b.Cr()))

			requireClusterToBeReadyEventually(t, sb, b)

			state, err := sb.Diff()
			require.NoError(t, err)

			expected := testutil.ReadOrUpdateGoldenFile(t, state, *updateOpt)

			testutil.AssertDiff(t, expected, state)
		},
	}

	steps := Steps{create}

	steps.Run(t)
}

func TestCreatesSecureClusterWithGeneratedCertCRv20(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	testLog := zapr.NewLogger(zaptest.NewLogger(t))

	actor.Log = testLog

	sb := testenv.NewDiffingSandbox(t, env)
	sb.StartManager(t, controller.InitClusterReconcilerWithLogger(testLog))

	builder := testutil.NewBuilder("crdb").WithNodeCount(3).WithTLS().
		WithImage("cockroachdb/cockroach:v20.1.6").
		WithPVDataStore("1Gi", "standard" /* default storage class in KIND */).
		WithLabels(map[string]string{"customlabel": "Test"})

	create := Step{
		name: "creates 3-node secure cluster with v20.1.6",
		test: func(t *testing.T) {
			require.NoError(t, sb.Create(builder.Cr()))
			requireClusterToBeReadyEventually(t, sb, builder)
		},
	}

	steps := Steps{create}

	steps.Run(t)
}

func TestUpgradesMinorVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	testLog := zapr.NewLogger(zaptest.NewLogger(t))

	actor.Log = testLog

	sb := testenv.NewDiffingSandbox(t, env)
	sb.StartManager(t, controller.InitClusterReconcilerWithLogger(testLog))

	builder := testutil.NewBuilder("crdb").WithNodeCount(1).WithTLS().
		WithImage("cockroachdb/cockroach:v19.2.5").
		WithPVDataStore("1Gi", "standard" /* default storage class in KIND */)

	steps := Steps{
		{
			name: "creates a 1-node secure cluster",
			test: func(t *testing.T) {
				require.NoError(t, sb.Create(builder.Cr()))

				requireClusterToBeReadyEventually(t, sb, builder)
			},
		},
		{
			name: "upgrades the cluster to the next patch version",
			test: func(t *testing.T) {
				current := builder.Cr()
				require.NoError(t, sb.Get(current))

				current.Spec.Image.Name = "cockroachdb/cockroach:v19.2.6"
				require.NoError(t, sb.Update(current))

				requireClusterToBeReadyEventually(t, sb, builder)
				requireDbContainersToUseImage(t, sb, current)
			},
		},
	}

	steps.Run(t)
}

func TestUpgradesMajorVersion19to20(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	testLog := zapr.NewLogger(zaptest.NewLogger(t))

	actor.Log = testLog

	sb := testenv.NewDiffingSandbox(t, env)
	sb.StartManager(t, controller.InitClusterReconcilerWithLogger(testLog))

	builder := testutil.NewBuilder("crdb").WithNodeCount(1).WithTLS().
		WithImage("cockroachdb/cockroach:v19.2.6").
		WithPVDataStore("1Gi", "standard" /* default storage class in KIND */)

	steps := Steps{
		{
			name: "creates a 1-node secure cluster",
			test: func(t *testing.T) {
				require.NoError(t, sb.Create(builder.Cr()))

				requireClusterToBeReadyEventually(t, sb, builder)
			},
		},
		{
			name: "upgrades the cluster to the next minor version",
			test: func(t *testing.T) {
				current := builder.Cr()
				require.NoError(t, sb.Get(current))

				current.Spec.Image.Name = "cockroachdb/cockroach:v20.1.1"
				require.NoError(t, sb.Update(current))

				requireClusterToBeReadyEventually(t, sb, builder)
				requireDbContainersToUseImage(t, sb, current)
			},
		},
	}

	steps.Run(t)
}

func TestUpgradesMajorVersion19_1To19_2(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	testLog := zapr.NewLogger(zaptest.NewLogger(t))

	actor.Log = testLog

	sb := testenv.NewDiffingSandbox(t, env)
	sb.StartManager(t, controller.InitClusterReconcilerWithLogger(testLog))

	builder := testutil.NewBuilder("crdb").WithNodeCount(1).WithTLS().
		WithImage("cockroachdb/cockroach:v19.1.4").
		WithPVDataStore("1Gi", "standard" /* default storage class in KIND */)

	steps := Steps{
		{
			name: "creates a 1-node secure cluster",
			test: func(t *testing.T) {
				require.NoError(t, sb.Create(builder.Cr()))

				requireClusterToBeReadyEventually(t, sb, builder)
			},
		},
		{
			name: "upgrades the cluster to the next minor version",
			test: func(t *testing.T) {
				current := builder.Cr()
				require.NoError(t, sb.Get(current))

				current.Spec.Image.Name = "cockroachdb/cockroach:v19.2.1"
				require.NoError(t, sb.Update(current))

				requireClusterToBeReadyEventually(t, sb, builder)
				requireDbContainersToUseImage(t, sb, current)
			},
		},
	}

	steps.Run(t)
}

// this is giving us an error of no inbound stream connection (SQLSTATE XXUUU)
// intermitently.

// Test the new partioned upgrades
func TestParitionedUpgradesMajorVersion19to20(t *testing.T) {

	if doNotTestFlakes(t) {
		t.Log("This test is marked as a flake, not running test")
		return
	} else {
		t.Log("Running this test, although this test is flakey")
	}

	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	testLog := zapr.NewLogger(zaptest.NewLogger(t))

	actor.Log = testLog

	require.NoError(t, utilfeature.DefaultMutableFeatureGate.Set("PartitionedUpdate=true"))

	sb := testenv.NewDiffingSandbox(t, env)
	sb.StartManager(t, controller.InitClusterReconcilerWithLogger(testLog))

	builder := testutil.NewBuilder("crdb").WithNodeCount(3).WithTLS().
		WithImage("cockroachdb/cockroach:v19.2.6").
		WithPVDataStore("1Gi", "standard" /* default storage class in KIND */)

	steps := Steps{
		{
			name: "creates a 3-node secure cluster for partitioned update",
			test: func(t *testing.T) {
				require.NoError(t, sb.Create(builder.Cr()))

				requireClusterToBeReadyEventually(t, sb, builder)
			},
		},
		{
			name: "upgrades the cluster to the next minor version",
			test: func(t *testing.T) {
				current := builder.Cr()
				require.NoError(t, sb.Get(current))

				current.Spec.Image.Name = "cockroachdb/cockroach:v20.1.6"
				require.NoError(t, sb.Update(current))

				requireClusterToBeReadyEventually(t, sb, builder)
				requireDbContainersToUseImage(t, sb, current)
				// This value matches the WithImage value above, without patch
				requireDownGradeOptionSet(t, sb, builder, "19.2")
				requireDatabaseToFunction(t, sb, builder)
			},
		},
	}

	steps.Run(t)
	// Disable the feature flag
	require.NoError(t, utilfeature.DefaultMutableFeatureGate.Set("PartitionedUpdate=false"))
}

func TestDatabaseFunctionality(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	testLog := zapr.NewLogger(zaptest.NewLogger(t))
	actor.Log = testLog
	sb := testenv.NewDiffingSandbox(t, env)
	sb.StartManager(t, controller.InitClusterReconcilerWithLogger(testLog))
	builder := testutil.NewBuilder("crdb").WithNodeCount(3).WithTLS().
		WithImage("cockroachdb/cockroach:v20.1.7").
		WithPVDataStore("1Gi", "standard" /* default storage class in KIND */)
	steps := Steps{
		{
			name: "creates a 3-node secure cluster and tests db",
			test: func(t *testing.T) {
				require.NoError(t, sb.Create(builder.Cr()))
				requireClusterToBeReadyEventually(t, sb, builder)
				requireDatabaseToFunction(t, sb, builder)
			},
		},
	}
	steps.Run(t)
}

func TestDecommissionFunctionality(t *testing.T) {

	if doNotTestFlakes(t) {
		t.Log("This test is marked as a flake, not running test")
		return
	} else {
		t.Log("Running this test, although this test is flakey")
	}

	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	testLog := zapr.NewLogger(zaptest.NewLogger(t))
	actor.Log = testLog
	//Enable decommission feature gate
	require.NoError(t, utilfeature.DefaultMutableFeatureGate.Set("UseDecommission=true"))
	sb := testenv.NewDiffingSandbox(t, env)
	sb.StartManager(t, controller.InitClusterReconcilerWithLogger(testLog))
	builder := testutil.NewBuilder("crdb").WithNodeCount(4).WithTLS().
		WithImage("cockroachdb/cockroach:v20.1.7").
		WithPVDataStore("1Gi", "standard" /* default storage class in KIND */)
	steps := Steps{
		{
			name: "creates a 4-node secure cluster and tests db",
			test: func(t *testing.T) {
				require.NoError(t, sb.Create(builder.Cr()))
				requireClusterToBeReadyEventually(t, sb, builder)
			},
		},
		{
			name: "decommission a node",
			test: func(t *testing.T) {
				current := builder.Cr()
				require.NoError(t, sb.Get(current))

				current.Spec.Nodes = 3
				require.NoError(t, sb.Update(current))
				requireClusterToBeReadyEventually(t, sb, builder)
				requireDecommissionNode(t, sb, builder)
				requireDatabaseToFunction(t, sb, builder)
			},
		},
	}
	steps.Run(t)
	//Disable decommission feature gate
	require.NoError(t, utilfeature.DefaultMutableFeatureGate.Set("UseDecommission=false"))
}

func TestPVCResize(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	require.NoError(t, utilfeature.DefaultMutableFeatureGate.Set("ResizePVC=true"))
	testLog := zapr.NewLogger(zaptest.NewLogger(t))
	actor.Log = testLog
	sb := testenv.NewDiffingSandbox(t, env)
	sb.StartManager(t, controller.InitClusterReconcilerWithLogger(testLog))
	builder := testutil.NewBuilder("crdb").WithNodeCount(1).WithTLS().
		WithImage("cockroachdb/cockroach:v20.1.7").
		WithPVDataStore("1Gi", "standard" /* default storage class in KIND */)
	steps := Steps{
		{
			name: "creates a 3-node secure cluster db",
			test: func(t *testing.T) {
				require.NoError(t, sb.Create(builder.Cr()))
				requireClusterToBeReadyEventually(t, sb, builder)
			},
		},
		{
			name: "resize PVC",
			test: func(t *testing.T) {
				current := builder.Cr()
				require.NoError(t, sb.Get(current))
				current.Spec.DataStore.VolumeClaim.PersistentVolumeClaimSpec.Resources.Limits.Storage().Set(2048)

				require.NoError(t, sb.Update(current))
				requireClusterToBeReadyEventually(t, sb, builder)

				requirePVCToResize(t, sb, builder)

			},
		},
	}
	steps.Run(t)
	require.NoError(t, utilfeature.DefaultMutableFeatureGate.Set("ResizePVC=false"))
}

func doNotTestFlakes(t *testing.T) bool {
	if os.Getenv("TEST_FLAKES") != "" {
		t.Log("running flakey tests")
		return false
	}
	t.Log("not running flakey tests")
	return true
}
