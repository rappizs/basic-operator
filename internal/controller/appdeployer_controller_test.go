package controller

import (
	"context"
	deployerv1 "deployment-operator/api/v1"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var once sync.Once

var _ = Describe("AppDeployer controller", func() {
	const (
		APIVersion    = "deployer.rappizs.com/v1"
		Kind          = "AppDeployer"
		Name          = "appdeployer-test"
		Namespace     = "default"
		Replicas      = 3
		Host          = "test.com"
		Image         = "ngingx:latest"
		ContainerPort = 80
		ServicePort   = 80
		ClusterIssuer = "test-issuer"
	)

	ctx := context.Background()

	var deployer *deployerv1.AppDeployer

	BeforeEach(func() {
		once.Do(func() {
			deployer = &deployerv1.AppDeployer{
				TypeMeta: metav1.TypeMeta{
					APIVersion: APIVersion,
					Kind:       Kind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      Name,
					Namespace: Namespace,
				},
				Spec: deployerv1.AppDeployerSpec{
					Replicas:      Replicas,
					Host:          Host,
					Image:         Image,
					ContainerPort: ContainerPort,
					ServicePort:   ServicePort,
					ClusterIssuer: ClusterIssuer,
				},
			}

			Expect(k8sClient.Create(ctx, deployer)).Should(Succeed())
		})
	})

	Context("When creating a new AppDeployment resource", func() {

		It("Should create a properly configured Deployment", func() {
			var deployment appsv1.Deployment

			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: Name, Namespace: Namespace}, &deployment)
			}).Should(Succeed())

			// Assert Deployment
			Expect(deployment.Spec.Selector.MatchLabels).To(Equal(map[string]string{
				"app": Name,
			}))

			Expect(deployment.Spec.Replicas).To(Equal(intToInt32Pointer(Replicas)))

			Expect(deployment.Spec.Template.ObjectMeta.Labels).To(Equal(map[string]string{
				"app": Name,
			}))

			terminationPeriod := int64(corev1.DefaultTerminationGracePeriodSeconds)

			Expect(deployment.Spec.Template.Spec).To(Equal(
				corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "base-container",
							Image: Image,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: int32(ContainerPort),
									Protocol:      corev1.ProtocolTCP,
								},
							},
							TerminationMessagePath:   corev1.TerminationMessagePathDefault,
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
							ImagePullPolicy:          corev1.PullAlways,
						},
					},
					SecurityContext:               &corev1.PodSecurityContext{},
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					SchedulerName:                 corev1.DefaultSchedulerName,
					TerminationGracePeriodSeconds: &terminationPeriod,
				}))

			controller := true

			Expect(deployment.OwnerReferences).To(Equal([]metav1.OwnerReference{
				{
					APIVersion:         APIVersion,
					Kind:               Kind,
					Name:               Name,
					UID:                deployer.GetUID(),
					Controller:         &controller,
					BlockOwnerDeletion: &controller,
				},
			}))

		})

		It("Should create a properly configured Service", func() {
			var service corev1.Service

			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: Name, Namespace: Namespace}, &service)
			}).Should(Succeed())
		})

		It("Should create a properly configured Ingress", func() {
			var ingress networkingv1.Ingress

			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: Name, Namespace: Namespace}, &ingress)
			}).Should(Succeed())

			Expect(k8sClient.Delete(ctx, deployer)).Should(Succeed())
		})
	})
})
