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
	"testing"
	"time"

	"github.com/cockroachdb/cockroach-operator/pkg/actor"
	"github.com/cockroachdb/cockroach-operator/pkg/controller"
	"github.com/cockroachdb/cockroach-operator/pkg/testutil"
	testenv "github.com/cockroachdb/cockroach-operator/pkg/testutil/env"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TODO parallel seems to be buggy.  Not certain why, but we need to figure out if running with the operator
// deployed in the cluster helps
// We may have a threadsafe problem where one test starts messing with another test
var parallel = *flag.Bool("parallel", false, "run tests in parallel")

// run the pvc test
var pvc = flag.Bool("pvc", false, "run pvc test")

// TODO should we make this an atomic that is created by evn pkg?
var env *testenv.ActiveEnv

// TestMain wraps the unit tests. Set TEST_DO_NOT_USE_KIND evnvironment variable to any value
// if you do not want this test to start a k8s cluster using kind.
func TestMain(m *testing.M) {
	e := testenv.CreateActiveEnvForTest([]string{"..", ".."})
	env = e.Start()
	code := testenv.RunCode(m, e)
	os.Exit(code)
}

func TestCreatesInSecureCluster(t *testing.T) {

	// Test Creating a insecure cluster
	// No actions on the cluster just create it and
	// tear it down.

	if parallel {
		t.Parallel()
	}
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	testLog := zapr.NewLogger(zaptest.NewLogger(t))

	actor.Log = testLog

	sb := testenv.NewDiffingSandbox(t, env)
	sb.StartManager(t, controller.InitClusterReconcilerWithLogger(testLog))

	builder := testutil.NewBuilder("crdb").WithNodeCount(3).
		WithImage("cockroachdb/cockroach:v21.1.6").
		WithPVDataStore("1Gi", "standard" /* default storage class in KIND */)

	steps := testutil.Steps{
		{
			Name: "creates 3-node insecure cluster",
			Test: func(t *testing.T) {
				require.NoError(t, sb.Create(builder.Cr()))

				testutil.RequireClusterToBeReadyEventuallyTimeout(t, sb, builder, 500*time.Second)
				testutil.RequireDatabaseToFunction(t, sb, builder)
				t.Log("Done with basic cluster")
			},
		},
	}
	steps.Run(t)
}

func TestCreatesSecureCluster(t *testing.T) {

	// Test Creating a secure cluster
	// No actions on the cluster just create it and
	// tear it down.

	if parallel {
		t.Parallel()
	}
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	testLog := zapr.NewLogger(zaptest.NewLogger(t))

	actor.Log = testLog

	sb := testenv.NewDiffingSandbox(t, env)
	sb.StartManager(t, controller.InitClusterReconcilerWithLogger(testLog))

	builder := testutil.NewBuilder("crdb").WithNodeCount(3).WithTLS().
		WithImage("cockroachdb/cockroach:v20.2.10").
		WithPVDataStore("1Gi", "standard" /* default storage class in KIND */)

	steps := testutil.Steps{
		{
			Name: "creates 3-node secure cluster",
			Test: func(t *testing.T) {
				require.NoError(t, sb.Create(builder.Cr()))

				testutil.RequireClusterToBeReadyEventuallyTimeout(t, sb, builder, 500*time.Second)
				testutil.RequireDatabaseToFunction(t, sb, builder)
				t.Log("Done with basic cluster")
			},
		},
	}
	steps.Run(t)
}
