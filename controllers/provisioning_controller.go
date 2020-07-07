/*

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

package controllers

import (
	"context"
        "os"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metal3iov1alpha1 "github.com/cluster-baremetal-operator-alt/api/v1alpha1"

        "github.com/go-logr/logr"
        "k8s.io/apimachinery/pkg/runtime"
        ctrl "sigs.k8s.io/controller-runtime"
        "sigs.k8s.io/controller-runtime/pkg/client"

        openshiftv1alpha1 "github.com/cluster-baremetal-operator-alt/api/v1alpha1"

        appsv1 "k8s.io/api/apps/v1"
        corev1 "k8s.io/api/core/v1"
        "k8s.io/apimachinery/pkg/api/errors"
        "k8s.io/apimachinery/pkg/runtime"
        "k8s.io/apimachinery/pkg/types"
        appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
        "sigs.k8s.io/controller-runtime/pkg/client"
        "sigs.k8s.io/controller-runtime/pkg/controller"
        "sigs.k8s.io/controller-runtime/pkg/handler"
        logf "sigs.k8s.io/controller-runtime/pkg/log"
        "sigs.k8s.io/controller-runtime/pkg/manager"
        "sigs.k8s.io/controller-runtime/pkg/reconcile"
        "sigs.k8s.io/controller-runtime/pkg/source"

        osconfigv1 "github.com/openshift/api/config/v1"
        osoperatorv1 "github.com/openshift/api/operator/v1"
        osclientset "github.com/openshift/client-go/config/clientset/versioned"
        metal3v1alpha1 "github.com/openshift/cluster-baremetal-operator/pkg/apis/metal3/v1alpha1"
        "github.com/openshift/library-go/pkg/operator/events"
        "github.com/openshift/library-go/pkg/operator/resource/resourceapply"
        "github.com/openshift/library-go/pkg/operator/resource/resourcemerge"

)

var log = logf.Log.WithName("controller_provisioning")
var componentNamespace = "openshift-machine-api"
var componentName = "cluster-baremetal-operator"

// OperatorConfig contains configuration for the metal3 Deployment
type OperatorConfig struct {
        TargetNamespace      string
        BaremetalControllers BaremetalControllers
}

type BaremetalControllers struct {
        BaremetalOperator         string
        Ironic                    string
        IronicInspector           string
        IronicIpaDownloader       string
        IronicMachineOsDownloader string
        IronicStaticIpManager     string
}

// ProvisioningReconciler reconciles a Provisioning object
type ProvisioningReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
        // that reads objects from the cache and writes to the apiserver
        client     client.Client

        appsClient *appsclientv1.AppsV1Client
        osClient   osclientset.Interface
        scheme     *runtime.Scheme
        log        logr.Logger
        config     *OperatorConfig

        // Track latest generation of our resources in memory, which means
        // we will re-apply on restart of the operator.
        // TODO: persist these to CR using operator.openshift.io OperatorStatus
        // TODO : Do we need this? It was not part of auto generated code
        //generations []osoperatorv1.GenerationStatus
}

// +kubebuilder:rbac:groups=metal3.io.,resources=provisionings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io.,resources=provisionings/status,verbs=get;update;patch

func (r *ProvisioningReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx = context.Background()
	log = r.Log.WithValues("provisioning", req.NamespacedName)

        infra := &osconfigv1.Infrastructure{}
        err := r.client.Get(ctx, types.NamespacedName{Name: "cluster"}, infra)
        if err != nil {
                log.Info("Unable to determine Platform that the Operator is running on.")
                return ctrl.Result{}, err
        }

        // Disable ourselves on platforms other than bare metal
        if infra.Status.Platform != osconfigv1.BareMetalPlatformType {
                err = updateCOStatusDisabled(r.client, r.osClient, r.config.TargetNamespace, os.Getenv("OPERATOR_VERSION"))
                if err != nil {
                        return ctrl.Result{}, err
                }
                // We're disabled; don't requeue
                return ctrl.Result{}, nil
        }

        err = updateCOStatusProgressing(r.client, r.osClient, r.config.TargetNamespace, os.Getenv("OPERATOR_VERSION"))
        if err != nil {
                return ctrl.Result{}, err
        }

        // provisioning.metal3.io is a singleton
        if req.Name != baremetalProvisioningCR {
                log.Info("Ignoring Provisioning.metal3.io without default name")
                return ctrl.Result{}, nil
        }

        // Fetch the Provisioning instance
        instance := &metal3v1alpha1.Provisioning{}
        err = r.client.Get(ctx, request.NamespacedName, instance)
        if err != nil {
                if errors.IsNotFound(err) {
                        // Request object not found, could have been deleted after reconcile request.
                        // Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
                        // Return and don't requeue
                        return ctrl.Result{}, nil
                }
                // Error reading the object - requeue the request.
                return ctrl.Result{}, err
        }

        // Create a Secret needed for the Metal3 deployment
        foundSecret := &corev1.Secret{}
        err = r.client.Get(ctx, types.NamespacedName{Name: baremetalSecretName, Namespace: r.config.TargetNamespace}, foundSecret)
        if err != nil && errors.IsNotFound(err) {
                // Secret does not already exist. So, create one.
                secret := createMariadbPasswordSecret(r.config)
                reqLogger.Info("Creating a new Maridb password secret", "Secret.Namespace", secret.Namespace, "Deployment.Name", secret.Name)
                err := r.client.Create(ctx, secret)
                if err != nil {
                        return ctx.Result{}, err
                }
        } else if err != nil {
                return ctx.Result{}, err
        }

        // Define a new Deployment object
        deployment := newMetal3Deployment(r.config, getBaremetalProvisioningConfig(instance))
        expectedGeneration := resourcemerge.ExpectedDeploymentGeneration(deployment, r.generations)
        _, updated, err := resourceapply.ApplyDeployment(r.appsClient, events.NewLoggingEventRecorder(componentName), deployment, expectedGeneration, false)
        if err != nil {
                if err = updateCOStatusDegraded(r.client, r.osClient, r.config.TargetNamespace, os.Getenv("OPERATOR_VERSION")); err != nil {
                        reqLogger.Info("Unable to set baremetal ClusterOperator status to Degraded.")
                }
                return reconcile.Result{}, err
        } else if updated {
                reqLogger.Info("Successfully created or updated Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
                resourcemerge.SetDeploymentGeneration(&r.generations, deployment)
        } else {
                reqLogger.Info("Skip reconcile: Deployment already up to date", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
        }

        if err = updateCOStatusAvailable(r.client, r.osClient, r.config.TargetNamespace, os.Getenv("OPERATOR_VERSION")); err != nil {
                reqLogger.Info("Unable to set baremetal ClusterOperator status to Available.")
        }

        // Success; don't requeue
	return ctrl.Result{}, nil
}

func (r *ProvisioningReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&metal3iov1alpha1.Provisioning{}).
		Complete(r)
}
