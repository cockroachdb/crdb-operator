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

package upgradessha256

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

// TestUpgradesMinorVersion tests a minor version bump
func TestUpgradesMinorVersion(t *testing.T) {

	// We are testing a Minor Version Upgrade with
	// partition update
	// Going from v20.2.8 to v20.2.9

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
	//set related image env var in sha256 format
	os.Setenv("RELATED_IMAGE_COCKROACH_v20_2_8", "cockroachdb/cockroach@sha256:162d653fe76cc6f7a9800ce1de40f03fd80467ee937f782630bd404c92e2a277")
	os.Setenv("RELATED_IMAGE_COCKROACH_v20_2_9", "cockroachdb/cockroach@sha256:d32411676b1c6583257a40818a6038ca7f906fe883b2ad1b1eea3986dd33526c")

	builder := testutil.NewBuilder("crdb").WithNodeCount(3).WithTLS().
		WithCockroachDBVersion("v20.2.8").
		WithPVDataStore("1Gi", "standard" /* default storage class in KIND */)

	steps := testutil.Steps{
		{
			Name: "creates a 3-nodes secure cluster",
			Test: func(t *testing.T) {
				require.NoError(t, sb.Create(builder.Cr()))
				testutil.RequireClusterToBeReadyEventuallyTimeout(t, sb, builder, 500*time.Second)
			},
		},
		{
			Name: "upgrades the cluster to the next patch version",
			Test: func(t *testing.T) {
				current := builder.Cr()
				require.NoError(t, sb.Get(current))

				current.Spec.CockroachDBVersion = "v20.2.9"
				require.NoError(t, sb.Update(current))

				testutil.RequireClusterToBeReadyEventuallyTimeout(t, sb, builder, 500*time.Second)
				testutil.RequireDbContainersToUseImage(t, sb, current)
				t.Log("Done with upgrade")
			},
		},
	}

	steps.Run(t)
	//clean
	os.Unsetenv("RELATED_IMAGE_COCKROACH_v20_2_8")
	os.Unsetenv("RELATED_IMAGE_COCKROACH_v20_2_9")
}

// TestUpgradesMajorVersion20to21 tests major Version Upgrade
func TestUpgradesMajorVersion20to21(t *testing.T) {

	// We are testing a Major Version Upgrade with
	// partition update
	// Going from v20.2.10 to v21.1.0 using related images in sha256 image format
	// and cockroachDBVersion field.

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
	//related images must be in sha256 format
	os.Setenv("RELATED_IMAGE_COCKROACH_v21_1_1", "cockroachdb/cockroach@sha256:7c84559a33db90b52f8179c904818525e45852b683bd6272f61dcf54c103f5b1")
	os.Setenv("RELATED_IMAGE_COCKROACH_v20_2_10", "cockroachdb/cockroach@sha256:a1ef571ff3b47b395084d2f29abbc7706be36a826a618a794697d90a03615ada")
	builder := testutil.NewBuilder("crdb").WithNodeCount(3).WithTLS().
		WithCockroachDBVersion("v20.2.10").
		WithPVDataStore("1Gi", "standard" /* default storage class in KIND */)

	steps := testutil.Steps{
		{
			Name: "creates a 3-nodes secure cluster",
			Test: func(t *testing.T) {
				require.NoError(t, sb.Create(builder.Cr()))
				testutil.RequireClusterToBeReadyEventuallyTimeout(t, sb, builder, 500*time.Second)
			},
		},
		{
			Name: "upgrades the cluster to the next patch version using related image in sha256 format",
			Test: func(t *testing.T) {
				current := builder.Cr()
				require.NoError(t, sb.Get(current))

				current.Spec.CockroachDBVersion = "v21.1.1"
				require.NoError(t, sb.Update(current))

				testutil.RequireClusterToBeReadyEventuallyTimeout(t, sb, builder, 500*time.Second)
				testutil.RequireDbContainersToUseImage(t, sb, current)
				t.Log("Done with upgrade")
			},
		},
	}

	steps.Run(t)
	//clean
	os.Unsetenv("RELATED_IMAGE_COCKROACH_v21_1_1")
	os.Unsetenv("RELATED_IMAGE_COCKROACH_v20_2_10")
}

// TestUpgradesMajorVersion20_1To20_2 is another major version upgrade
func TestUpgradesMajorVersion20_1To20_2(t *testing.T) {

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
	//set related image env var in sha256 format 
	os.Setenv("RELATED_IMAGE_COCKROACH_v20_2_10", "cockroachdb/cockroach@sha256:a1ef571ff3b47b395084d2f29abbc7706be36a826a618a794697d90a03615ada")
	os.Setenv("RELATED_IMAGE_COCKROACH_v20_1_16", "cockroachdb/cockroach@sha256:73edc4b4b473d0461de39092a8e4b1939b5c4edc557d0a5666de07a7290d70d8")

	builder := testutil.NewBuilder("crdb").WithNodeCount(3).WithTLS().
		WithCockroachDBVersion("v20.1.16").
		WithPVDataStore("1Gi", "standard" /* default storage class in KIND */)

	steps := testutil.Steps{
		{
			Name: "creates a 3-node secure cluster",
			Test: func(t *testing.T) {
				require.NoError(t, sb.Create(builder.Cr()))

				testutil.RequireClusterToBeReadyEventuallyTimeout(t, sb, builder, 500*time.Second)
			},
		},
		{
			Name: "upgrades the cluster to the next major version",
			Test: func(t *testing.T) {
				current := builder.Cr()
				require.NoError(t, sb.Get(current))

				current.Spec.CockroachDBVersion = "v20.2.10"
				require.NoError(t, sb.Update(current))
				testutil.RequireClusterToBeReadyEventuallyTimeout(t, sb, builder, 500*time.Second)
				testutil.RequireDbContainersToUseImage(t, sb, current)
				t.Log("Done with major upgrade")
			},
		},
	}

	steps.Run(t)
	//clean
	os.Unsetenv("RELATED_IMAGE_COCKROACH_v20_2_10")
	os.Unsetenv("RELATED_IMAGE_COCKROACH_v20_1_16")

}
