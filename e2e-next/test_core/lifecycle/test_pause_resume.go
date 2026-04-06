package lifecycle

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/loft-sh/vcluster/e2e-next/constants"
	"github.com/loft-sh/vcluster/e2e-next/labels"
	"github.com/loft-sh/vcluster/pkg/cli/find"
	"github.com/loft-sh/vcluster/pkg/util/random"
)

var _ = Describe("Tenant cluster pause and resume", labels.Core, labels.PR, func() {
	Context("pause and resume a running tenant cluster", Ordered, func() {
		// Ordered because resume depends on the pause from the prior spec.
		var (
			suffix      string
			clusterName string
			namespace   string
			hostClient  kubernetes.Interface
		)

		BeforeAll(func(ctx context.Context) {
			suffix = random.String(6)
			clusterName = "e2e-pause-resume-" + suffix
			namespace = clusterName
			hostClient = hostKubeClient()
			createAndWaitForReady(ctx, hostClient, clusterName, namespace)
		})

		AfterAll(func(ctx context.Context) {
			_, err := runVClusterCmd(ctx, "delete", clusterName, "-n", namespace, "--delete-namespace")
			Expect(err).To(Succeed())
		})

		It("should pause the tenant cluster", func(ctx context.Context) {
			By("Pausing the tenant cluster", func() {
				_, err := runVClusterCmd(ctx, "pause", clusterName, "-n", namespace)
				Expect(err).To(Succeed())
			})

			By("Verifying all pods are gone", func() {
				Eventually(func(g Gomega, ctx context.Context) {
					pods, err := hostClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
						LabelSelector: "app=vcluster,release=" + clusterName,
					})
					g.Expect(err).To(Succeed())
					g.Expect(pods.Items).To(BeEmpty(),
						"tenant cluster %s should have 0 pods after pause, got %d", clusterName, len(pods.Items))
				}).WithContext(ctx).WithPolling(constants.PollingInterval).WithTimeout(constants.PollingTimeoutLong).Should(Succeed())
			})

			By("Verifying list shows Paused status", func() {
				Eventually(func(g Gomega, ctx context.Context) {
					entries, err := listVClusters(ctx, namespace)
					g.Expect(err).To(Succeed())
					found := findByName(entries, clusterName)
					g.Expect(found).NotTo(BeNil(), "tenant cluster %s not found in list", clusterName)
					g.Expect(found.Status).To(Equal(string(find.StatusPaused)),
						"tenant cluster %s has status %s, expected %s", clusterName, found.Status, find.StatusPaused)
				}).WithContext(ctx).WithPolling(constants.PollingInterval).WithTimeout(constants.PollingTimeout).Should(Succeed())
			})
		})

		It("should resume the tenant cluster", func(ctx context.Context) {
			By("Resuming the tenant cluster", func() {
				_, err := runVClusterCmd(ctx, "resume", clusterName, "-n", namespace)
				Expect(err).To(Succeed())
			})

			By("Waiting for tenant cluster to be ready", func() {
				waitForVClusterReady(ctx, hostClient, clusterName, namespace)
			})

			By("Verifying list shows Running status", func() {
				Eventually(func(g Gomega, ctx context.Context) {
					entries, err := listVClusters(ctx, namespace)
					g.Expect(err).To(Succeed())
					found := findByName(entries, clusterName)
					g.Expect(found).NotTo(BeNil(), "tenant cluster %s not found in list", clusterName)
					g.Expect(found.Status).To(Equal(string(find.StatusRunning)),
						"tenant cluster %s has status %s, expected %s", clusterName, found.Status, find.StatusRunning)
				}).WithContext(ctx).WithPolling(constants.PollingInterval).WithTimeout(constants.PollingTimeoutLong).Should(Succeed())
			})
		})
	})

	Context("pause and resume a scaled-down tenant cluster", Ordered, func() {
		// Ordered because each spec depends on state from the prior spec:
		// create → scale down → pause → resume.
		var (
			suffix      string
			clusterName string
			namespace   string
			hostClient  kubernetes.Interface
		)

		BeforeAll(func(ctx context.Context) {
			suffix = random.String(6)
			clusterName = "e2e-pause-resume-sd-" + suffix
			namespace = clusterName
			hostClient = hostKubeClient()
			createAndWaitForReady(ctx, hostClient, clusterName, namespace)
			scaleDownVCluster(ctx, hostClient, clusterName, namespace)
		})

		AfterAll(func(ctx context.Context) {
			_, err := runVClusterCmd(ctx, "delete", clusterName, "-n", namespace, "--delete-namespace")
			Expect(err).To(Succeed())
		})

		It("should pause a scaled-down tenant cluster", func(ctx context.Context) {
			By("Pausing the scaled-down tenant cluster", func() {
				_, err := runVClusterCmd(ctx, "pause", clusterName, "-n", namespace)
				Expect(err).To(Succeed())
			})

			By("Verifying paused annotations are set on the StatefulSet", func() {
				sts, err := hostClient.AppsV1().StatefulSets(namespace).Get(ctx, clusterName, metav1.GetOptions{})
				Expect(err).To(Succeed())
				Expect(sts.Annotations).To(HaveKeyWithValue("loft.sh/paused", "true"),
					"StatefulSet %s/%s should have loft.sh/paused=true annotation", namespace, clusterName)
				Expect(sts.Annotations).To(HaveKeyWithValue("loft.sh/paused-replicas", "1"),
					"StatefulSet %s/%s should have loft.sh/paused-replicas=1 annotation", namespace, clusterName)
			})
		})

		It("should resume the tenant cluster after pause", func(ctx context.Context) {
			By("Resuming the tenant cluster", func() {
				_, err := runVClusterCmd(ctx, "resume", clusterName, "-n", namespace)
				Expect(err).To(Succeed())
			})

			By("Waiting for tenant cluster to be ready", func() {
				waitForVClusterReady(ctx, hostClient, clusterName, namespace)
			})

			By("Verifying StatefulSet has correct replica count", func() {
				sts, err := hostClient.AppsV1().StatefulSets(namespace).Get(ctx, clusterName, metav1.GetOptions{})
				Expect(err).To(Succeed())
				Expect(sts.Spec.Replicas).NotTo(BeNil())
				Expect(*sts.Spec.Replicas).To(Equal(int32(1)),
					"StatefulSet %s/%s should have 1 replica, got %d", namespace, clusterName, *sts.Spec.Replicas)
			})

			By("Verifying list shows Running status", func() {
				Eventually(func(g Gomega, ctx context.Context) {
					entries, err := listVClusters(ctx, namespace)
					g.Expect(err).To(Succeed())
					found := findByName(entries, clusterName)
					g.Expect(found).NotTo(BeNil(), "tenant cluster %s not found in list", clusterName)
					g.Expect(found.Status).To(Equal(string(find.StatusRunning)),
						"tenant cluster %s has status %s, expected %s", clusterName, found.Status, find.StatusRunning)
				}).WithContext(ctx).WithPolling(constants.PollingInterval).WithTimeout(constants.PollingTimeoutLong).Should(Succeed())
			})
		})
	})
})
