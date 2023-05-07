package controller

import (
	"context"
	deployerv1 "deployment-operator/api/v1"
	"reflect"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

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
	var ownerRef metav1.OwnerReference

	Context("When creating a new AppDeployment resource", func() {
		var once sync.Once

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

				controller := true
				ownerRef = metav1.OwnerReference{
					APIVersion:         APIVersion,
					Kind:               Kind,
					Name:               Name,
					UID:                deployer.GetUID(),
					Controller:         &controller,
					BlockOwnerDeletion: &controller,
				}
			})
		})

		It("Should create a properly configured Deployment", func() {
			var deployment appsv1.Deployment

			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: Name, Namespace: Namespace}, &deployment)
			}).Should(Succeed())

			Expect(deployment.Spec.Selector).To(Equal(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": deployer.Name,
				},
			}))

			Expect(deployment.Spec.Replicas).To(Equal(intToInt32Ptr(Replicas)))

			Expect(deployment.Spec.Template.ObjectMeta).To(Equal(metav1.ObjectMeta{
				Labels: map[string]string{
					"app": deployer.Name,
				},
			}))

			terminationPeriod := int64(corev1.DefaultTerminationGracePeriodSeconds)
			Expect(deployment.Spec.Template.Spec).To(Equal(
				corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  defaultContainerName,
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

			Expect(deployment.OwnerReferences).To(Equal([]metav1.OwnerReference{ownerRef}))
		})

		It("Should create a properly configured Service", func() {
			var service corev1.Service

			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: Name, Namespace: Namespace}, &service)
			}).Should(Succeed())

			Expect(service.Spec.Selector).To(Equal(map[string]string{
				"app": deployer.Name,
			}))

			Expect(service.Spec.Ports).To(Equal([]corev1.ServicePort{
				{
					Name:       "http",
					Port:       int32(ServicePort),
					TargetPort: intstr.FromInt(ContainerPort),
					Protocol:   corev1.ProtocolTCP,
				},
			}))

			Expect(service.OwnerReferences).To(Equal([]metav1.OwnerReference{ownerRef}))
		})

		It("Should create a properly configured Ingress", func() {
			var ingress networkingv1.Ingress

			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: Name, Namespace: Namespace}, &ingress)
			}).Should(Succeed())

			Expect(ingress.ObjectMeta.Annotations).To(Equal(map[string]string{
				"cert-manager.io/cluster-issuer": ClusterIssuer,
			}))

			pathType := networkingv1.PathTypePrefix
			Expect(ingress.Spec.Rules).To(Equal([]networkingv1.IngressRule{
				{
					Host: Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: deployer.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: int32(ServicePort),
											},
										},
									},
								},
							},
						},
					},
				},
			}))

			Expect(ingress.Spec.TLS).To(Equal([]networkingv1.IngressTLS{
				{
					Hosts:      []string{Host},
					SecretName: Name,
				},
			}))

			Expect(ingress.OwnerReferences).To(Equal([]metav1.OwnerReference{ownerRef}))
		})
	})

	const (
		newReplicas      = 5
		newHost          = "test-new.com"
		newImage         = "ngingx-new:latest"
		newContainerPort = 81
		newServicePort   = 81
		newClusterIssuer = "test-issuer-new"
	)

	Context("When updating the AppDeployment resource", func() {
		var once sync.Once

		BeforeEach(func() {
			once.Do(func() {
				deployer.Spec = deployerv1.AppDeployerSpec{
					Replicas:      newReplicas,
					Host:          newHost,
					Image:         newImage,
					ContainerPort: newContainerPort,
					ServicePort:   newServicePort,
					ClusterIssuer: newClusterIssuer,
				}

				Expect(k8sClient.Update(ctx, deployer)).Should(Succeed())
			})
		})

		It("Should update the Deployment", func() {
			var deployment appsv1.Deployment

			// wait for the Deployment to be updated
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: Name, Namespace: Namespace}, &deployment)
				if err != nil {
					return false
				}

				return *deployment.Spec.Replicas == newReplicas
			}).Should(BeTrue())

			Expect(deployment.Spec.Selector).To(Equal(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": deployer.Name,
				},
			}))

			Expect(deployment.Spec.Template.ObjectMeta).To(Equal(metav1.ObjectMeta{
				Labels: map[string]string{
					"app": deployer.Name,
				},
			}))

			terminationPeriod := int64(corev1.DefaultTerminationGracePeriodSeconds)
			Expect(deployment.Spec.Template.Spec).To(Equal(
				corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  defaultContainerName,
							Image: newImage,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: int32(newContainerPort),
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

			Expect(deployment.OwnerReferences).To(Equal([]metav1.OwnerReference{ownerRef}))
		})

		It("Should update the Service", func() {
			var service corev1.Service

			// wait for the Service to be updated
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: Name, Namespace: Namespace}, &service)
				if err != nil {
					return false
				}

				return reflect.DeepEqual(service.Spec.Ports, []corev1.ServicePort{
					{
						Name:       "http",
						Port:       int32(newServicePort),
						TargetPort: intstr.FromInt(newContainerPort),
						Protocol:   corev1.ProtocolTCP,
					}})
			}).Should(BeTrue())

			Expect(service.Spec.Selector).To(Equal(map[string]string{
				"app": deployer.Name,
			}))

			Expect(service.OwnerReferences).To(Equal([]metav1.OwnerReference{ownerRef}))
		})

		It("Should update the Ingress", func() {
			var ingress networkingv1.Ingress

			// wait for the Ingress to be updated
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: Name, Namespace: Namespace}, &ingress)
				if err != nil {
					return false
				}

				return reflect.DeepEqual(ingress.Spec.TLS, []networkingv1.IngressTLS{
					{
						Hosts:      []string{newHost},
						SecretName: Name,
					}})
			}).Should(BeTrue())

			Expect(ingress.ObjectMeta.Annotations).To(Equal(map[string]string{
				"cert-manager.io/cluster-issuer": newClusterIssuer,
			}))

			pathType := networkingv1.PathTypePrefix
			Expect(ingress.Spec.Rules).To(Equal([]networkingv1.IngressRule{
				{
					Host: newHost,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: deployer.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: int32(newServicePort),
											},
										},
									},
								},
							},
						},
					},
				},
			}))

			Expect(ingress.OwnerReferences).To(Equal([]metav1.OwnerReference{ownerRef}))
		})
	})
})
