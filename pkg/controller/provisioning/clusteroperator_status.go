package provisioning

import (
	"os"
        "reflect"
	"fmt"

        "github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/errors"
	"github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
        osconfigv1 "github.com/openshift/api/config/v1"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StatusReason represents the reason for a status change.
// It is always expected to be a MixedCaps string.
type StatusReason string

// The default set of status change reasons.
const (
        ReasonEmpty      StatusReason = ""
        ReasonSyncing    StatusReason = "SyncingResources"
        ReasonSyncFailed StatusReason = "SyncingFailed"
)

const (
        clusterOperatorName = "cluster-baremetal"
)

var (
        // This is to be compliant with
        // https://github.com/openshift/cluster-version-operator/blob/b57ee63baf65f7cb6e95a8b2b304d88629cfe3c0/docs/dev/clusteroperator.md#what-should-an-operator-report-with-clusteroperator-custom-resource
        // When known hazardous states for upgrades are determined
        // specific "Upgradeable=False" status can be added with messages for how admins
        // can resolve it.
        operatorUpgradeable = newClusterOperatorStatusCondition(osconfigv1.OperatorUpgradeable, osconfigv1.ConditionTrue, "", "")
)

func newClusterOperatorStatusCondition(conditionType osconfigv1.ClusterStatusConditionType,
        conditionStatus osconfigv1.ConditionStatus, reason string,
        message string) osconfigv1.ClusterOperatorStatusCondition {
        return osconfigv1.ClusterOperatorStatusCondition{
                Type:               conditionType,
                Status:             conditionStatus,
                LastTransitionTime: metav1.Now(),
                Reason:             reason,
                Message:            message,
        }
}

// statusProgressing sets the Progressing condition to True, with the given
// reason and message, and sets the upgradeable condition to True.  It does not
// modify any existing Available or Degraded conditions.
func (cbo *CBO) statusProgressing() error {
        desiredVersions := os.Getenv("OPERATOR_VERSION")

	// TODO Making an assumption here that the Cluster Operator already exists
	// Check to see if we need to check if the ClusterOperator already exists
	// and create one if it doesn't
        currentVersions, err := cbo.getCurrentVersions()

        if err != nil {
                glog.Errorf("Error getting operator current versions: %v", err)
                return err
        }
        var isProgressing osconfigv1.ConditionStatus
	
        co, err := cbo.getOrCreateClusterOperator()
        if err != nil {
              	glog.Errorf("Failed to get or create Cluster Operator: %v", err)
               	return err
        }

        var message string
        if !reflect.DeepEqual(desiredVersions, currentVersions) {
                glog.V(2).Info("Syncing status: progressing")
		// TODO Use K8s event recorder to report this state
                isProgressing = osconfigv1.ConditionTrue
        } else {
                glog.V(2).Info("Syncing status: re-syncing")
		// TODO Use K8s event recorder to report this state
                isProgressing = osconfigv1.ConditionFalse
        }

        conds := []osconfigv1.ClusterOperatorStatusCondition{
                newClusterOperatorStatusCondition(osconfigv1.OperatorProgressing, isProgressing, string(ReasonSyncing), message),
                operatorUpgradeable,
        }

        return cbo.updateStatus(co, conds)
	return nil
}

// getClusterOperator returns the current ClusterOperator.
func (cbo *CBO) getClusterOperator() (*osconfigv1.ClusterOperator, error) {
        return cbo.osClient.ConfigV1().ClusterOperators().
                Get(clusterOperatorName, metav1.GetOptions{})
}

// defaultStatusConditions returns the default set of status conditions for the
// ClusterOperator resource used on first creation of the ClusterOperator.
func (cbo *CBO) defaultStatusConditions() []osconfigv1.ClusterOperatorStatusCondition {
        // All conditions default to False with no message.
        return []osconfigv1.ClusterOperatorStatusCondition{
                newClusterOperatorStatusCondition(
                        osconfigv1.OperatorProgressing,
                        osconfigv1.ConditionFalse,
                        "", "",
                ),
                newClusterOperatorStatusCondition(
                        osconfigv1.OperatorDegraded,
                        osconfigv1.ConditionFalse,
                        "", "",
                ),
                newClusterOperatorStatusCondition(
                        osconfigv1.OperatorAvailable,
                        osconfigv1.ConditionFalse,
                        "", "",
                ),
        }
}


// defaultClusterOperator returns the default ClusterOperator resource with
// default values for related objects and status conditions.
func (cbo *CBO) defaultClusterOperator() *osconfigv1.ClusterOperator {
        return &osconfigv1.ClusterOperator{
		TypeMeta: metav1.TypeMeta{
                	Kind:       "ClusterOperator",
                        APIVersion: "config.openshift.io/v1",
                },
                ObjectMeta: metav1.ObjectMeta{
                        Name: clusterOperatorName,
                },
                Status: osconfigv1.ClusterOperatorStatus{
                        Conditions:     cbo.defaultStatusConditions(),
                        RelatedObjects: []osconfigv1.ObjectReference{
                        	{
                                	Group:    "",
                                        Resource: "namespaces",
                                        Name:     cbo.namespace,
                                },
			},
                },
        }
}


// createClusterOperator creates the ClusterOperator and updates its status.
func (cbo *CBO) createClusterOperator() (*osconfigv1.ClusterOperator, error) {
        defaultCO := cbo.defaultClusterOperator()

        co, err := cbo.osClient.ConfigV1().ClusterOperators().Create(defaultCO)
        if err != nil {
                return nil, err
        }

        co.Status = defaultCO.Status

        return cbo.osClient.ConfigV1().ClusterOperators().UpdateStatus(co)
}

// getOrCreateClusterOperator fetches the current ClusterOperator or creates a
// default one if not found -- ensuring the related objects list is current.
func (cbo *CBO) getOrCreateClusterOperator() (*osconfigv1.ClusterOperator, error) {
        existing, err := cbo.getClusterOperator()

        if errors.IsNotFound(err) {
                glog.Infof("ClusterOperator does not exist, creating a new one.")
                return cbo.createClusterOperator()
        }

        if err != nil {
                return nil, fmt.Errorf("failed to get clusterOperator %q: %v", clusterOperatorName, err)
        }
	return existing, nil
}

func (cbo *CBO) getCurrentVersions() ([]osconfigv1.OperandVersion, error) {
        co, err := cbo.getOrCreateClusterOperator()
	if err != nil {
            	return nil, err
        }
        return co.Status.Versions, nil

}

//syncStatus applies the new condition to the mao ClusterOperator object.
func (cbo *CBO) updateStatus(co *osconfigv1.ClusterOperator, conds []osconfigv1.ClusterOperatorStatusCondition) error {
        for _, c := range conds {
                v1helpers.SetStatusCondition(&co.Status.Conditions, c)
        }

        _, err := cbo.osClient.ConfigV1().ClusterOperators().UpdateStatus(co)
        return err
}
