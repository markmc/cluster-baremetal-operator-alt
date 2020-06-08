package provisioning

import (
	"context"
	"os"
	"time"

	"github.com/golang/glog"
	configv1 "github.com/openshift/api/config/v1"
	osoperatorv1 "github.com/openshift/api/operator/v1"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/client-go/kubernetes"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
	"k8s.io/client-go/tools/record"
	appslisterv1 "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	appsinformersv1 "k8s.io/client-go/informers/apps/v1"	
)

// CBO defines the cluster baremetal operator.
type CBO struct {
        namespace, name string

        imagesFile string
        config     string

        kubeClient    kubernetes.Interface
        osClient      osclientset.Interface
        eventRecorder record.EventRecorder

        syncHandler func(ic string) error

        deployLister       appslisterv1.DeploymentLister
        deployListerSynced cache.InformerSynced

        // queue only ever has one item, but it has nice error handling backoff/retry semantics
        queue           workqueue.RateLimitingInterface
        operandVersions []configv1.OperandVersion

        generations []osoperatorv1.GenerationStatus
}

// New returns a new cluster baremetal operator.
func New(
        namespace, name string,
        imagesFile string,

        config string,

        deployInformer appsinformersv1.DeploymentInformer,

        kubeClient kubernetes.Interface,
        osClient osclientset.Interface,

        recorder record.EventRecorder,
) *CBO {
	operandVersions := []configv1.OperandVersion{}
        if releaseVersion := os.Getenv("RELEASE_VERSION"); len(releaseVersion) > 0 {
                operandVersions = append(operandVersions, configv1.OperandVersion{Name: "operator", Version: releaseVersion})
        }

	cbo := &CBO{
                namespace:       namespace,
                name:            name,
                imagesFile:      imagesFile,
                kubeClient:      kubeClient,
                osClient:        osClient,
                eventRecorder:   recorder,
                queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "clusterbaremetaloperator"),
                operandVersions: operandVersions,
        }

	// TODO : Figure out if and how we want to manage the event handler queue
	//deployInformer.Informer().AddEventHandler(cbo.eventHandlerDeployments())

	cbo.config = config
	cbo.syncHandler = sync

        cbo.deployLister = deployInformer.Lister()
        cbo.deployListerSynced = deployInformer.Informer().HasSynced

        return cbo
}

func sync(key string) error {
        startTime := time.Now()
        glog.V(4).Infof("Started syncing Cluster Baremetal Operator %q (%v)", key, startTime)
        defer func() {
                glog.V(4).Infof("Finished syncing Cluster Baremetal Operator %q (%v)", key, time.Since(startTime))
        }()

        //return syncClusterOperator(r.client, r.config.TargetNamespace, os.Getenv("OPERATOR_VERSION"), true, false)
	return nil
}

func syncClusterOperator(c client.Client, targetNamespace string, version string, done, disabled bool) error {
	name := types.NamespacedName{Name: "baremetal"}

	co := &configv1.ClusterOperator{}
	err := c.Get(context.Background(), name, co)
	if err != nil {
		// Not found - create the resource
		if errors.IsNotFound(err) {
			co = &configv1.ClusterOperator{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterOperator",
					APIVersion: "config.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: name.Name,
				},
				Status: configv1.ClusterOperatorStatus{
					RelatedObjects: []configv1.ObjectReference{
						{
							Group:    "",
							Resource: "namespaces",
							Name:     targetNamespace,
						},
					},
				},
			}
			err = c.Create(context.TODO(), co)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	prevConditions := co.Status.Conditions
	co.Status.Conditions = updateConditions(prevConditions, done, disabled)

	operatorv1helpers.SetOperandVersion(&co.Status.Versions, configv1.OperandVersion{Name: "operator", Version: version})

	if conditionsEquals(co.Status.Conditions, prevConditions) {
		return nil
	}

	err = c.Update(context.TODO(), co)
	if err != nil {
		return err
	}

	return nil
}

// OperatorDisabled reports when the primary function of the operator has been disabled.
const OperatorDisabled configv1.ClusterStatusConditionType = "Disabled"

func updateConditions(conditions []configv1.ClusterOperatorStatusCondition, done, disabled bool) []configv1.ClusterOperatorStatusCondition {
	// FIXME: actually implement the expected semantics of these conditions
	conditions = []configv1.ClusterOperatorStatusCondition{
		{
			Type:   configv1.OperatorAvailable,
			Status: configv1.ConditionFalse,
		}, {
			Type:   configv1.OperatorProgressing,
			Status: configv1.ConditionFalse,
		}, {
			Type:   configv1.OperatorDegraded,
			Status: configv1.ConditionFalse,
		}, {
			Type:   OperatorDisabled,
			Status: configv1.ConditionFalse,
		},
	}
	if done {
		conditions[0].Status = configv1.ConditionTrue
	} else {
		conditions[1].Status = configv1.ConditionTrue
	}
	if disabled {
		conditions[3].Status = configv1.ConditionTrue
	}
	return conditions
}

func conditionsEquals(conditions []configv1.ClusterOperatorStatusCondition, prev []configv1.ClusterOperatorStatusCondition) bool {
	// FIXME: only update when something has changed
	return false
}
