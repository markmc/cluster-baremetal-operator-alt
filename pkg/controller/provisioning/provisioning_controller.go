package provisioning

import (
	"context"
	"os"

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
	metal3v1alpha1 "github.com/openshift/cluster-baremetal-operator/pkg/apis/metal3/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
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

// Add creates a new Provisioning Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileProvisioning{
		client:     mgr.GetClient(),
		appsClient: appsclientv1.NewForConfigOrDie(mgr.GetConfig()),
		osClient:   osclientset.NewForConfigOrDie(mgr.GetConfig()),
		scheme:     mgr.GetScheme(),
		config: &OperatorConfig{
			TargetNamespace: componentNamespace,
			BaremetalControllers: BaremetalControllers{
				BaremetalOperator:         os.Getenv("BAREMETAL_IMAGE"),
				Ironic:                    os.Getenv("IRONIC_IMAGE"),
				IronicInspector:           os.Getenv("IRONIC_INSPECTOR_IMAGE"),
				IronicIpaDownloader:       os.Getenv("IRONIC_IPA_DOWNLOADER_IMAGE"),
				IronicMachineOsDownloader: os.Getenv("IRONIC_MACHINE_OS_DOWNLOADER_IMAGE"),
				IronicStaticIpManager:     os.Getenv("IRONIC_STATIC_IP_MANAGER_IMAGE"),
			},
		},
	}

}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("provisioning-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Provisioning
	err = c.Watch(&source.Kind{Type: &metal3v1alpha1.Provisioning{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to our Deployment and Secret and requeue the owner Provisioning
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &metal3v1alpha1.Provisioning{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &metal3v1alpha1.Provisioning{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileProvisioning implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileProvisioning{}

// ReconcileProvisioning reconciles a Provisioning object
type ReconcileProvisioning struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	appsClient *appsclientv1.AppsV1Client
	osClient   osclientset.Interface
	scheme     *runtime.Scheme
	config     *OperatorConfig

	// Track latest generation of our resources in memory, which means
	// we will re-apply on restart of the operator.
	// TODO: persist these to CR using operator.openshift.io OperatorStatus
	generations []osoperatorv1.GenerationStatus
}

// Reconcile reads that state of the cluster for a Provisioning object and makes changes based on the state read
// and what is in the Provisioning.Spec
func (r *ReconcileProvisioning) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Provisioning")

	infra := &osconfigv1.Infrastructure{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, infra)
	if err != nil {
                reqLogger.Info("Unable to determine Platform that the Operator is running on.")
		return reconcile.Result{}, err
	}

	// Disable ourselves on platforms other than bare metal
	if infra.Status.Platform != osconfigv1.BareMetalPlatformType {
		err = updateCOStatusDisabled(r.client, r.osClient, r.config.TargetNamespace, os.Getenv("OPERATOR_VERSION"))
		if err != nil {
			return reconcile.Result{}, err
		}
		// We're disabled; don't requeue
		return reconcile.Result{}, nil
	}

	err = updateCOStatusProgressing(r.client, r.osClient, r.config.TargetNamespace, os.Getenv("OPERATOR_VERSION"))
	if err != nil {
		return reconcile.Result{}, err
	}

        // provisioning.metal3.io is a singleton
        if request.Name != baremetalProvisioningCR {
                reqLogger.Info("Ignoring Provisioning.metal3.io without default name")
                return reconcile.Result{}, nil
        }

	// Fetch the Provisioning instance
	instance := &metal3v1alpha1.Provisioning{}
	err = r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Create a Secret needed for the Metal3 deployment
	foundSecret := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: baremetalSecretName, Namespace: r.config.TargetNamespace}, foundSecret)
	if err != nil && errors.IsNotFound(err) {
		// Secret does not already exist. So, create one.
		secret := createMariadbPasswordSecret(r.config)
		reqLogger.Info("Creating a new Maridb password secret", "Secret.Namespace", secret.Namespace, "Deployment.Name", secret.Name)
		err := r.client.Create(context.TODO(), secret)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Define a new Deployment object
	deployment := newMetal3Deployment(r.config, getBaremetalProvisioningConfig(instance))
	expectedGeneration := resourcemerge.ExpectedDeploymentGeneration(deployment, r.generations)
	_, updated, err := resourceapply.ApplyDeployment(r.appsClient, events.NewLoggingEventRecorder(componentName), deployment, expectedGeneration, false)
	if err != nil {
		return reconcile.Result{}, err
	} else if updated {
		reqLogger.Info("Successfully created or updated Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
		resourcemerge.SetDeploymentGeneration(&r.generations, deployment)
	} else {
		reqLogger.Info("Skip reconcile: Deployment already up to date", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
	}

	if err != nil {
		_ = updateCOStatusDegraded(r.client, r.osClient, r.config.TargetNamespace, os.Getenv("OPERATOR_VERSION"))
		return reconcile.Result{}, err
	}

	err = updateCOStatusAvailable(r.client, r.osClient, r.config.TargetNamespace, os.Getenv("OPERATOR_VERSION"))
	// Success; don't requeue
	return reconcile.Result{}, nil
}
