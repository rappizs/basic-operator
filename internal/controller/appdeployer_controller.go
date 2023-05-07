/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	deployerv1 "deployment-operator/api/v1"
)

// AppDeployerReconciler reconciles a AppDeployer object
type AppDeployerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=deployer.rappizs.com,resources=appdeployers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=deployer.rappizs.com,resources=appdeployers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=deployer.rappizs.com,resources=appdeployers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services/status,verbs=get
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AppDeployer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *AppDeployerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	deployer := &deployerv1.AppDeployer{}
	err := r.Get(ctx, req.NamespacedName, deployer)
	if err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	err = r.createDeployment(ctx, deployer)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.createService(ctx, deployer)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.createIngress(ctx, deployer)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *AppDeployerReconciler) getDeployment(ctx context.Context, name, namespace string) (*appsv1.Deployment, error) {
	var deployment appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &deployment)

	return &deployment, err
}

func (r *AppDeployerReconciler) getService(ctx context.Context, name, namespace string) (*corev1.Service, error) {
	var service corev1.Service
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &service)

	return &service, err
}

func (r *AppDeployerReconciler) getIngress(ctx context.Context, name, namespace string) (*networkingv1.Ingress, error) {
	var ingress networkingv1.Ingress
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &ingress)

	return &ingress, err
}

func (r *AppDeployerReconciler) createDeployment(ctx context.Context, deployer *deployerv1.AppDeployer) error {
	log := log.FromContext(ctx)
	spec := deployer.Spec

	newDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployer.Name,
			Namespace: deployer.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": deployer.Name,
				},
			},
			Replicas: intToInt32Ptr(spec.Replicas),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": deployer.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "base-container",
							Image: spec.Image,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: int32(spec.ContainerPort),
								},
							},
						},
					},
				},
			},
		},
	}

	currDeployment, err := r.getDeployment(ctx, deployer.Name, deployer.Namespace)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		err = controllerutil.SetControllerReference(deployer, newDeployment, r.Scheme)
		if err != nil {
			return err
		}

		log.Info("Creating deployment")
		return r.Create(ctx, newDeployment)
	}

	if *currDeployment.Spec.Replicas != *newDeployment.Spec.Replicas {
		currDeployment.Spec.Replicas = newDeployment.Spec.Replicas
		log.Info("Updating deployment")
		return r.Update(ctx, currDeployment)
	}

	return nil
}

func (r *AppDeployerReconciler) createService(ctx context.Context, deployer *deployerv1.AppDeployer) error {
	log := log.FromContext(ctx)
	spec := deployer.Spec

	newService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployer.Name,
			Namespace: deployer.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": deployer.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       int32(spec.ServicePort),
					TargetPort: intstr.FromInt(spec.ContainerPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	currService, err := r.getService(ctx, deployer.Name, deployer.Namespace)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		err = controllerutil.SetControllerReference(deployer, &newService, r.Scheme)
		if err != nil {
			return err
		}

		log.Info("Creating service")
		return r.Create(ctx, &newService)
	}

	if !reflect.DeepEqual(currService.Spec.Ports, newService.Spec.Ports) {
		currService.Spec.Ports = newService.Spec.Ports

		log.Info("Updating service")
		return r.Update(ctx, currService)
	}

	return nil
}

func (r *AppDeployerReconciler) createIngress(ctx context.Context, deployer *deployerv1.AppDeployer) error {
	log := log.FromContext(ctx)
	spec := deployer.Spec

	pathType := networkingv1.PathTypePrefix
	newIngress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployer.Name,
			Namespace: deployer.Namespace,
			Annotations: map[string]string{
				"cert-manager.io/cluster-issuer": spec.ClusterIssuer,
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: spec.Host,
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
												Number: int32(spec.ServicePort),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      []string{spec.Host},
					SecretName: deployer.Name,
				},
			},
		},
	}

	currIngress, err := r.getIngress(ctx, deployer.Name, deployer.Namespace)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		err = controllerutil.SetControllerReference(deployer, &newIngress, r.Scheme)
		if err != nil {
			return err
		}

		log.Info("Creating ingress")
		return r.Create(ctx, &newIngress)
	}

	if currIngress.Spec.Rules[0].Host != spec.Host {
		currIngress.Spec.Rules = newIngress.Spec.Rules
		currIngress.Spec.TLS = newIngress.Spec.TLS

		log.Info("Updating ingress")
		return r.Update(ctx, currIngress)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppDeployerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deployerv1.AppDeployer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}

func intToInt32Ptr(value int) *int32 {
	value32 := int32(value)
	return &value32
}
