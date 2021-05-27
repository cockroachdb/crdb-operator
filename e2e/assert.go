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
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"

	api "github.com/cockroachdb/cockroach-operator/apis/v1alpha1"
	"github.com/cockroachdb/cockroach-operator/pkg/database"
	"github.com/cockroachdb/cockroach-operator/pkg/kube"
	"github.com/cockroachdb/cockroach-operator/pkg/labels"
	"github.com/cockroachdb/cockroach-operator/pkg/resource"
	"github.com/cockroachdb/cockroach-operator/pkg/testutil"
	testenv "github.com/cockroachdb/cockroach-operator/pkg/testutil/env"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// RequireClusterToBeReadyEventuallyTimeout tests to see if a statefulset has started correctly and
// all of the pods are running.
func RequireClusterToBeReadyEventuallyTimeout(t *testing.T, sb testenv.DiffingSandbox, b testutil.ClusterBuilder, timeout time.Duration) {
	cluster := b.Cluster()

	err := wait.Poll(10*time.Second, timeout, func() (bool, error) {

		ss, err := fetchStatefulSet(sb, cluster.StatefulSetName())
		if err != nil {
			t.Logf("error fetching stateful set")
			return false, err
		}

		if ss == nil {
			t.Logf("stateful set is not found")
			return false, nil
		}

		if !statefulSetIsReady(ss) {
			t.Logf("stateful set is not ready")
			logPods(context.TODO(), ss, cluster, sb, t)
			return false, nil
		}
		return true, nil
	})

	if err != nil {
		ss, err := fetchStatefulSet(sb, cluster.StatefulSetName())
		if err != nil {
			t.Logf("error fetching stateful set and not logging pods")
		}

		if ss == nil {
			t.Logf("stateful set is not found and not logging pods")
		}

		if !statefulSetIsReady(ss) {
			t.Logf("stateful set is not ready logging pods")
		}
		if ss != nil {
			logErr := logPodLogs(context.TODO(), ss, cluster, sb, t)
			if logErr != nil {
				t.Logf("error logging the pods %v", logErr)
			}
		}
		err = errors.Wrapf(err, "error waiting for test")
	}
	require.NoError(t, err)
}

func requireDbContainersToUseImage(t *testing.T, sb testenv.DiffingSandbox, cr *api.CrdbCluster) {
	err := wait.Poll(10*time.Second, 400*time.Second, func() (bool, error) {
		pods, err := fetchPodsInStatefulSet(sb, labels.Common(cr).Selector())
		if err != nil {
			return false, err
		}

		if len(pods) < int(cr.Spec.Nodes) {
			return false, nil
		}

		res := testPodsWithPredicate(pods, func(p *corev1.Pod) bool {
			c, err := kube.FindContainer(resource.DbContainerName, &p.Spec)
			if err != nil {
				return false
			}

			return c.Image == cr.Spec.Image.Name
		})

		return res, nil
	})

	if err != nil {
		t.Logf("pods are not running correct container version")
	}

	require.NoError(t, err)
}

func clusterIsInitialized(t *testing.T, sb testenv.DiffingSandbox, name string) (bool, error) {
	expectedConditions := []api.ClusterCondition{
		{
			Type:   api.InitializedCondition,
			Status: metav1.ConditionFalse,
		},
	}

	actual := resource.ClusterPlaceholder(name)
	if err := sb.Get(actual); err != nil {
		t.Logf("failed to fetch current cluster status :(")
		return false, err
	}

	actualConditions := actual.Status.DeepCopy().Conditions

	// Reset condition time as it is not significant for the assertion
	var emptyTime metav1.Time
	for i := range actualConditions {
		actualConditions[i].LastTransitionTime = emptyTime
	}

	if !cmp.Equal(expectedConditions, actualConditions) {
		t.Logf("cluster is not initialized")
		return false, nil
	}

	return true, nil
}

func clusterIsDecommissioned(t *testing.T, sb testenv.DiffingSandbox, name string) (bool, error) {
	expectedConditions := []api.ClusterCondition{
		{
			Type:   api.DecommissionCondition,
			Status: metav1.ConditionTrue,
		},
	}

	actual := resource.ClusterPlaceholder(name)
	if err := sb.Get(actual); err != nil {
		t.Logf("failed to fetch current cluster status :(")
		return false, err
	}

	actualConditions := actual.Status.DeepCopy().Conditions

	// Reset condition time as it is not significant for the assertion
	var emptyTime metav1.Time
	for i := range actualConditions {
		actualConditions[i].LastTransitionTime = emptyTime
	}
	if !cmp.Equal(expectedConditions, actualConditions) {
		t.Logf("cluster is not decomissioned")
		return false, nil
	}

	return true, nil
}

func fetchStatefulSet(sb testenv.DiffingSandbox, name string) (*appsv1.StatefulSet, error) {
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	if err := sb.Get(ss); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return ss, nil
}

func fetchPodsInStatefulSet(sb testenv.DiffingSandbox, labels map[string]string) ([]corev1.Pod, error) {
	var pods corev1.PodList

	if err := sb.List(&pods, labels); err != nil {
		return nil, err
	}

	return pods.Items, nil
}

func testPodsWithPredicate(pods []corev1.Pod, pred func(*corev1.Pod) bool) bool {
	for i := range pods {
		if !pred(&pods[i]) {
			return false
		}
	}

	return true
}

func statefulSetIsReady(ss *appsv1.StatefulSet) bool {
	return ss.Status.ReadyReplicas == ss.Status.Replicas
}

func requireDownGradeOptionSet(t *testing.T, sb testenv.DiffingSandbox, b testutil.ClusterBuilder, version string) {
	sb.Mgr.GetConfig()
	podName := fmt.Sprintf("%s-0.%s", b.Cluster().Name(), b.Cluster().Name())
	conn := &database.DBConnection{
		Ctx:    context.TODO(),
		Client: sb.Mgr.GetClient(),
		Port:   b.Cluster().Spec().SQLPort,
		UseSSL: true,

		RestConfig:   sb.Mgr.GetConfig(),
		ServiceName:  podName,
		Namespace:    sb.Namespace,
		DatabaseName: "system",

		RunningInsideK8s:            false,
		ClientCertificateSecretName: b.Cluster().ClientTLSSecretName(),
		RootCertificateSecretName:   b.Cluster().NodeTLSSecretName(),
	}

	// Create a new database connection for the update.
	db, err := database.NewDbConnection(conn)
	require.NoError(t, err)
	defer db.Close()

	r := db.QueryRowContext(context.TODO(), "SHOW CLUSTER SETTING cluster.preserve_downgrade_option")
	var value string
	if err := r.Scan(&value); err != nil {
		t.Fatal(err)
	}

	if value == "" {
		t.Errorf("downgrade_option is empty and should be set to %s", version)
	}

	if value != value {
		t.Errorf("downgrade_option is not set to %s, but is set to %s", version, value)
	}

}
func requireDecommissionNode(t *testing.T, sb testenv.DiffingSandbox, b testutil.ClusterBuilder, numNodes int32) {
	cluster := b.Cluster()

	err := wait.Poll(10*time.Second, 400*time.Second, func() (bool, error) {
		sts, err := fetchStatefulSet(sb, cluster.StatefulSetName())
		if err != nil {
			t.Logf("statefulset is not found %v", err)
			return false, err
		}

		if sts == nil {
			t.Log("statefulset is not found")
			return false, nil
		}

		if !statefulSetIsReady(sts) {
			t.Log("statefulset is not ready")
			return false, nil
		}

		if numNodes != sts.Status.Replicas {
			t.Log("statefulset replicas do not match")
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		t.Log("timed out waiting for requiring deco'ed node")
	}
	require.NoError(t, err)
}

func requireDatabaseToFunction(t *testing.T, sb testenv.DiffingSandbox, b testutil.ClusterBuilder) {
	sb.Mgr.GetConfig()
	podName := fmt.Sprintf("%s-0.%s", b.Cluster().Name(), b.Cluster().Name())
	conn := &database.DBConnection{
		Ctx:    context.TODO(),
		Client: sb.Mgr.GetClient(),
		Port:   b.Cluster().Spec().SQLPort,
		UseSSL: true,

		RestConfig:   sb.Mgr.GetConfig(),
		ServiceName:  podName,
		Namespace:    sb.Namespace,
		DatabaseName: "system",

		RunningInsideK8s:            false,
		ClientCertificateSecretName: b.Cluster().ClientTLSSecretName(),
		RootCertificateSecretName:   b.Cluster().NodeTLSSecretName(),
	}

	// Create a new database connection for the update.
	db, err := database.NewDbConnection(conn)
	require.NoError(t, err)
	defer db.Close()

	if _, err := db.Exec("CREATE DATABASE test_db"); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Exec("USE test_db"); err != nil {
		t.Fatal(err)
	}

	// Create the "accounts" table.
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS accounts (id INT PRIMARY KEY, balance INT)"); err != nil {
		t.Fatal(err)
	}

	// Insert two rows into the "accounts" table.
	if _, err := db.Exec(
		"INSERT INTO accounts (id, balance) VALUES (1, 1000), (2, 250)"); err != nil {
		t.Fatal(err)
	}

	// Print out the balances.
	rows, err := db.Query("SELECT id, balance FROM accounts")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	t.Log("Initial balances:")
	for rows.Next() {
		var id, balance int
		if err := rows.Scan(&id, &balance); err != nil {
			t.Fatal(err)
		}
		t.Log("balances", id, balance)
	}

	countRows, err := db.Query("SELECT COUNT(*) as count FROM accounts")
	if err != nil {
		t.Fatal(err)
	}
	defer countRows.Close()
	count := getCount(t, countRows)
	if count != 2 {
		t.Fatal(fmt.Errorf("found incorrect number of rows.  Expected 2 got %v", count))
	}

	t.Log("finished testing database")
}

func getCount(t *testing.T, rows *sql.Rows) (count int) {
	for rows.Next() {
		err := rows.Scan(&count)
		if err != nil {
			t.Fatal(err)
		}
	}
	return count
}

func requirePVCToResize(t *testing.T, sb testenv.DiffingSandbox, b testutil.ClusterBuilder, quantity apiresource.Quantity) {
	cluster := b.Cluster()

	// TODO rewrite this
	err := wait.Poll(10*time.Second, 500*time.Second, func() (bool, error) {
		ss, err := fetchStatefulSet(sb, cluster.StatefulSetName())
		if err != nil {
			return false, err
		}

		if ss == nil {
			t.Logf("stateful set is not found")
			return false, nil
		}

		if !statefulSetIsReady(ss) {
			return false, nil
		}
		clientset, err := kubernetes.NewForConfig(sb.Mgr.GetConfig())
		require.NoError(t, err)

		resized, err := resizedPVCs(context.TODO(), ss, b.Cluster(), clientset, t, quantity)
		require.NoError(t, err)

		return resized, nil
	})

	if err != nil {
		t.Logf("timed out waiting for PVCs to resize")
	}
	require.NoError(t, err)
}

// test to see if all PVCs are resized
func resizedPVCs(ctx context.Context, sts *appsv1.StatefulSet, cluster *resource.Cluster,
	clientset *kubernetes.Clientset, t *testing.T, quantity apiresource.Quantity) (bool, error) {

	prefixes := make([]string, len(sts.Spec.VolumeClaimTemplates))
	pvcsToKeep := make(map[string]bool, int(*sts.Spec.Replicas)*len(sts.Spec.VolumeClaimTemplates))
	for j, pvct := range sts.Spec.VolumeClaimTemplates {
		prefixes[j] = fmt.Sprintf("%s-%s-", pvct.Name, sts.Name)

		for i := int32(0); i < *sts.Spec.Replicas; i++ {
			name := fmt.Sprintf("%s-%s-%d", pvct.Name, sts.Name, i)
			pvcsToKeep[name] = true
		}
	}

	selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
	if err != nil {
		return false, err
	}

	pvcs, err := clientset.CoreV1().PersistentVolumeClaims(cluster.Namespace()).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})

	if err != nil {
		return false, err
	}

	for _, pvc := range pvcs.Items {
		t.Logf("checking pvc %s", pvc.Name)
		// Resize PVCs that are still in use
		if pvcsToKeep[pvc.Name] {
			if !pvc.Spec.Resources.Requests.Storage().Equal(quantity) {
				return false, nil
			}
		}
	}

	return true, nil
}

func logPods(ctx context.Context, sts *appsv1.StatefulSet, cluster *resource.Cluster,
	sb testenv.DiffingSandbox, t *testing.T) error {
	// create a new clientset to talk to k8s
	clientset, err := kubernetes.NewForConfig(sb.Mgr.GetConfig())
	if err != nil {
		return err
	}

	// the LableSelector I thought worked did not
	// so I just get all of the Pods in a NS
	options := metav1.ListOptions{
		//LabelSelector: "app=" + cluster.StatefulSetName(),
	}

	// Get all pods
	podList, err := clientset.CoreV1().Pods(sts.Namespace).List(ctx, options)
	if err != nil {
		return err
	}

	if len(podList.Items) == 0 {
		t.Log("no pods found")
	}

	// Print out pretty into on the Pods
	for _, podInfo := range (*podList).Items {
		t.Logf("pods-name=%v\n", podInfo.Name)
		t.Logf("pods-status=%v\n", podInfo.Status.Phase)
		t.Logf("pods-condition=%v\n", podInfo.Status.Conditions)
		/*
			// TODO if pod is running but not ready for some period get pod logs
			if kube.IsPodReady(&podInfo) {
				t.Logf("pods-condition=%v\n", podInfo.Status.Conditions)
			}
		*/
	}

	return nil
}

func logPodLogs(ctx context.Context, sts *appsv1.StatefulSet, cluster *resource.Cluster,
	sb testenv.DiffingSandbox, t *testing.T) error {
	// create a new clientset to talk to k8s
	clientset, err := kubernetes.NewForConfig(sb.Mgr.GetConfig())
	if err != nil {
		return err
	}

	// the LableSelector I thought worked did not
	// so I just get all of the Pods in a NS
	options := metav1.ListOptions{
		//LabelSelector: "app=" + cluster.StatefulSetName(),
	}

	// Get all pods
	podList, err := clientset.CoreV1().Pods(sts.Namespace).List(ctx, options)
	if err != nil {
		return err
	}

	if len(podList.Items) == 0 {
		t.Log("no pods found")
	}

	// Print out pretty into on the Pods
	for _, podInfo := range (*podList).Items {
		t.Logf("pods-name=%v\n", podInfo.Name)
		t.Logf("pods-status=%v\n", podInfo.Status.Phase)
		t.Logf("pods-condition=%v\n", podInfo.Status.Conditions)
		/*
			// TODO if pod is running but not ready for some period get pod logs
			if kube.IsPodReady(&podInfo) {
				t.Logf("pods-condition=%v\n", podInfo.Status.Conditions)
			}
		*/
		log, err := getPodLog(ctx, podInfo.Name, sts.Namespace, clientset)

		if err != nil {
			return err
		}

		t.Log(log)
	}

	return nil
}

func getPodLog(ctx context.Context, podName string, namespace string, clientset kubernetes.Interface) (string, error) {

	// This func will print out the pod logs
	// This is code that is used by version checker and we should probably refactor
	// this and move it into kube package.
	// But right now it is untested
	podLogOpts := corev1.PodLogOptions{}
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &podLogOpts)

	podLogs, err := req.Stream(ctx)
	if err != nil {
		msg := "error in opening stream"
		return "", errors.Wrapf(err, msg)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		msg := "error in copying stream"
		return "", errors.Wrapf(err, msg)
	}
	return buf.String(), nil
}
