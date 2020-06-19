package provisioning

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	osconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	name = types.NamespacedName{Name: "baremetal"}

	// This is to be compliant with
	// https://github.com/openshift/cluster-version-operator/blob/b57ee63baf65f7cb6e95a8b2b304d88629cfe3c0/docs/dev/clusteroperator.md#what-should-an-operator-report-with-clusteroperator-custom-resource
	// When known hazardous states for upgrades are determined
	// specific "Upgradeable=False" status can be added with messages for how admins
	// can resolve it.
	operatorUpgradeable = setCOStatusCondition(osconfigv1.OperatorUpgradeable, osconfigv1.ConditionTrue, "", "")
)

// StatusReason is a MixedCaps string representing the reason for a
// status condition change.
type StatusReason string

const (
	clusterOperatorName = "baremetal"
	// OperatorDisabled reports when the primary function of the operator has been disabled.
	OperatorDisabled osconfigv1.ClusterStatusConditionType = "Disabled"

	// The default set of status change reasons.
	ReasonEmpty      StatusReason = ""
	ReasonSyncing    StatusReason = "SyncingResources"
	ReasonSyncFailed StatusReason = "SyncingFailed"
	ReasonDisabled   StatusReason = "UnsupportedPlatform"
)

// getInitStatusConditions returns the default set of status conditions for the
// ClusterOperator resource used on first creation of the ClusterOperator.
func setInitStatusConditions(co *osconfigv1.ClusterOperator) {
	// All conditions default to False with no message.
	conds := []osconfigv1.ClusterOperatorStatusCondition{
		setCOStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, "", ""),
		setCOStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionFalse, "", ""),
		setCOStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionFalse, "", ""),
		setCOStatusCondition(OperatorDisabled, osconfigv1.ConditionFalse, "", ""),
		operatorUpgradeable,
	}

	for _, cond := range conds {
		v1helpers.SetStatusCondition(&co.Status.Conditions, cond)
	}
	return
}

// createNewCO creates a new ClusterOperator and updates its status.
func createNewCO(c client.Client, namespace string, version string) (*osconfigv1.ClusterOperator, error) {
	new := &osconfigv1.ClusterOperator{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterOperator",
			APIVersion: "config.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name.Name,
		},
		Status: osconfigv1.ClusterOperatorStatus{
			RelatedObjects: []osconfigv1.ObjectReference{
				{
					Group:    "",
					Resource: "namespaces",
					Name:     namespace,
				},
			},
		},
	}

	operatorv1helpers.SetOperandVersion(&new.Status.Versions, osconfigv1.OperandVersion{Name: "operator", Version: version})

	err := c.Create(context.TODO(), new)
	if err != nil {
		return nil, err
	}

	setInitStatusConditions(new)

	if err = c.Update(context.TODO(), new); err != nil {
		glog.Error("Unable to add Status Conditions to the new ClusterOperator")
		return nil, err
	}
	current := v1helpers.FindStatusCondition(new.Status.Conditions, osconfigv1.OperatorUpgradeable)
	buf, err := yaml.Marshal(current)
	glog.Errorf("CO's Init status condition:\n %s", buf)

	return new, nil
}

// getExistingOrNewCO gets the existing CO, failing which it creates a new CO.
func getExistingOrNewCO(c client.Client, namespace string, version string) (*osconfigv1.ClusterOperator, error) {
	existing := &osconfigv1.ClusterOperator{ObjectMeta: metav1.ObjectMeta{Name: name.Name}}
	err := c.Get(context.TODO(), name, existing)

	if errors.IsNotFound(err) {
		glog.Errorf("Baremetal ClusterOperator does not exist, creating a new one.")
		return createNewCO(c, namespace, version)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get clusterOperator %q: %v", clusterOperatorName, err)
	}
	return existing, nil
}

func setCOStatusCondition(conditionType osconfigv1.ClusterStatusConditionType,
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

func updateCOStatusProgressing(c client.Client, targetNamespace string, targetVersions string) error {
	var isProgressing osconfigv1.ConditionStatus
	var message string

	// Find existing CO or create a new one
	// TODO: what should the version of the ClusterOperator be?
	co, err := getExistingOrNewCO(c, targetNamespace, targetVersions)
	if err != nil {
		glog.Errorf("Failed to get or create ClusterOperator: %v", err)
		return err
	}

	current := v1helpers.FindStatusCondition(co.Status.Conditions, osconfigv1.OperatorUpgradeable)
	if current != nil {
		buf, _ := yaml.Marshal(current)
		glog.Errorf("CO's current status condition:\n %s", buf)
	}

	currentVersions := co.Status.Versions

	if !reflect.DeepEqual(targetVersions, currentVersions) {
		glog.V(2).Info("Syncing status: progressing")
		// TODO Use K8s event recorder to report this state
		isProgressing = osconfigv1.ConditionTrue
		message = fmt.Sprintf("Progressing towards target version")
	} else {
		glog.V(2).Info("Syncing status: re-syncing")
		// TODO Use K8s event recorder to report this state
		isProgressing = osconfigv1.ConditionFalse
		message = fmt.Sprintf("Re-sync running")
	}
	conds := []osconfigv1.ClusterOperatorStatusCondition{
		setCOStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionFalse, "Starting up", "Before start of Metal3 deployment"),
		setCOStatusCondition(osconfigv1.OperatorProgressing, isProgressing, string(ReasonSyncing), message),
		setCOStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionFalse, "", ""),
		setCOStatusCondition(OperatorDisabled, osconfigv1.ConditionFalse, "", ""),
		operatorUpgradeable,
	}
	for _, cond := range conds {
		v1helpers.SetStatusCondition(&co.Status.Conditions, cond)
	}
	if err = c.Update(context.TODO(), co); err != nil {
		glog.Errorf("Could not update Status Condition Progressing to True for baremetal ClusterOperator")
		return err
	}

	current = v1helpers.FindStatusCondition(co.Status.Conditions, osconfigv1.OperatorProgressing)
	buf, err := yaml.Marshal(current)
	glog.Errorf("Reading back baremetal CO when in progress:\n %s", buf)
	return nil

}

func updateCOStatusAvailable(c client.Client, targetNamespace string, targetVersions string) error {
	message := "Metal3 pod available"
	reason := "Metal3DeployComplete"

	// Find existing CO or create a new one
	// TODO: what should the version of the ClusterOperator be?
	co, err := getExistingOrNewCO(c, targetNamespace, targetVersions)
	if err != nil {
		glog.Errorf("Failed to get or create ClusterOperator: %v", err)
		return err
	}

	conds := []osconfigv1.ClusterOperatorStatusCondition{
		setCOStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionTrue, reason, message),
		setCOStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, "", ""),
		setCOStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionFalse, "", ""),
		setCOStatusCondition(OperatorDisabled, osconfigv1.ConditionFalse, "", ""),
		operatorUpgradeable,
	}
	for _, cond := range conds {
		v1helpers.SetStatusCondition(&co.Status.Conditions, cond)
	}
	if err = c.Update(context.TODO(), co); err != nil {
		glog.Errorf("Could not update Status Condition Available to True for baremetal ClusterOperator")
		return err
	}

	current := v1helpers.FindStatusCondition(co.Status.Conditions, osconfigv1.OperatorAvailable)
	buf, err := yaml.Marshal(current)
	glog.Errorf("Reading back baremetal CO when Available:\n %s", buf)
	return nil
}

func updateCOStatusDegraded(c client.Client, targetNamespace string, targetVersions string) error {
	message := "Operator set to degraded"
	reason := "Metal3DeployFailed"

	// Find existing CO or create a new one
	// TODO: what should the version of the ClusterOperator be?
	co, err := getExistingOrNewCO(c, targetNamespace, targetVersions)
	if err != nil {
		glog.Errorf("Failed to get or create ClusterOperator: %v", err)
		return err
	}

	conds := []osconfigv1.ClusterOperatorStatusCondition{
		setCOStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionFalse, "", ""),
		setCOStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, "", ""),
		setCOStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionTrue, reason, message),
		setCOStatusCondition(OperatorDisabled, osconfigv1.ConditionFalse, "", ""),
		operatorUpgradeable,
	}
	for _, cond := range conds {
		v1helpers.SetStatusCondition(&co.Status.Conditions, cond)
	}
	if err = c.Update(context.TODO(), co); err != nil {
		glog.Errorf("Could not update Status Condition Degraded to True for baremetal ClusterOperator")
		return err
	}

	current := v1helpers.FindStatusCondition(co.Status.Conditions, osconfigv1.OperatorDegraded)
	buf, _ := yaml.Marshal(current)
	glog.Errorf("Reading back baremetal CO when Degraded:\n %s", buf)
	return nil
}

func updateCOStatusDisabled(c client.Client, targetNamespace string, targetVersions string) error {
	message := "Operator is non functional"
	reason := "UnsupportedPlatform"

	// Find existing CO or create a new one
	// TODO: what should the version of the ClusterOperator be?
	co, err := getExistingOrNewCO(c, targetNamespace, targetVersions)
	if err != nil {
		glog.Errorf("Failed to get or create ClusterOperator: %v", err)
		return err
	}

	conds := []osconfigv1.ClusterOperatorStatusCondition{
		setCOStatusCondition(osconfigv1.OperatorAvailable, osconfigv1.ConditionFalse, "", ""),
		setCOStatusCondition(osconfigv1.OperatorProgressing, osconfigv1.ConditionFalse, "", ""),
		setCOStatusCondition(osconfigv1.OperatorDegraded, osconfigv1.ConditionFalse, "", ""),
		setCOStatusCondition(OperatorDisabled, osconfigv1.ConditionTrue, reason, message),
		operatorUpgradeable,
	}
	for _, cond := range conds {
		v1helpers.SetStatusCondition(&co.Status.Conditions, cond)
	}
	if err = c.Update(context.TODO(), co); err != nil {
		glog.Errorf("Could not update Status Condition Degraded to True for baremetal ClusterOperator")
		return err
	}

	current := v1helpers.FindStatusCondition(co.Status.Conditions, OperatorDisabled)
	buf, _ := yaml.Marshal(current)
	glog.Errorf("Reading back baremetal CO when Disabled:\n %s", buf)
	return nil
}
