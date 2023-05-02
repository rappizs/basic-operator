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

// ImageDeployerReconciler reconciles a ImageDeployer object
type ImageDeployerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=deployer.rappizs.com,resources=imagedeployers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=deployer.rappizs.com,resources=imagedeployers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=deployer.rappizs.com,resources=imagedeployers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ImageDeployer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ImageDeployerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	deployer := &deployerv1.ImageDeployer{}
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

	return ctrl.Result{}, nil
}

func (r *ImageDeployerReconciler) getDeployment(ctx context.Context, name, namespace string) (*appsv1.Deployment, error) {
	var deployment appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &deployment)

	return &deployment, err
}

func (r *ImageDeployerReconciler) getService(ctx context.Context, name, namespace string) (*corev1.Service, error) {
	var deployment corev1.Service
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &deployment)

	return &deployment, err
}

func (r *ImageDeployerReconciler) deleteDeployment(ctx context.Context, name, namespace string) error {
	deployment, err := r.getDeployment(ctx, name, namespace)
	if err != nil {
		return err
	}

	return r.Delete(ctx, deployment)
}

func (r *ImageDeployerReconciler) createDeployment(ctx context.Context, deployer *deployerv1.ImageDeployer) error {
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
			Replicas: intToInt32Pointer(spec.Replicas),
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

func (r *ImageDeployerReconciler) createService(ctx context.Context, deployer *deployerv1.ImageDeployer) error {
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

// SetupWithManager sets up the controller with the Manager.
func (r *ImageDeployerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deployerv1.ImageDeployer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

func intToInt32Pointer(value int) *int32 {
	value32 := int32(value)
	return &value32
}
