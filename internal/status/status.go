package status

import (
	"context"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1alpha1 "github.com/andreasgerstmayr/tempo-helm-operator/api/v1alpha1"
	"github.com/andreasgerstmayr/tempo-helm-operator/internal/manifestutils"
)

const (
	messageReady   = "All components are operational"
	messageFailed  = "Some Tempo components failed"
	messagePending = "Some Tempo components are pending on dependencies"
)

// ConfigurationError contains information about why the managed TempoStack has an invalid configuration.
type ConfigurationError struct {
	Reason  v1alpha1.ConditionReason
	Message string
}

func (e *ConfigurationError) Error() string {
	return fmt.Sprintf("invalid configuration: %s", e.Message)
}

func isPodReady(pod corev1.Pod) bool {
	for _, c := range pod.Status.ContainerStatuses {
		if !c.Ready {
			return false
		}
	}
	return true
}

func getPodsStatus(ctx context.Context, c client.Client, namespace string, name string, component string) (v1alpha1.PodStatusMap, error) {
	psm := v1alpha1.PodStatusMap{}
	opts := []client.ListOption{
		client.MatchingLabels(manifestutils.ComponentLabels(component, name)),
		client.InNamespace(namespace),
	}

	pods := &corev1.PodList{}
	err := c.List(ctx, pods, opts...)
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		phase := pod.Status.Phase
		if phase == corev1.PodRunning {
			// for the component status consider running, but not ready, pods as pending
			if !isPodReady(pod) {
				phase = corev1.PodPending
			}
		}
		psm[phase] = append(psm[phase], pod.Name)
	}

	return psm, nil
}

func getStatefulSetStatus(ctx context.Context, c client.Client, namespace string, name string, component string) (v1alpha1.PodStatusMap, error) {
	psm := v1alpha1.PodStatusMap{}

	opts := []client.ListOption{
		client.MatchingLabels(manifestutils.ComponentLabels(component, name)),
		client.InNamespace(namespace),
	}

	// After creation of a StatefulSet, but before the Pods are created, the list of Pods is empty
	// and therefore no Pod is in pending phase. However, this does not reflect the actual state,
	// therefore we additionally check if the StatefulSet has the required number of readyReplicas.
	//
	// This additional check also helps with Pods in terminating state, which otherwise would show up
	// as Pods with PodPhase = Running.
	stss := &appsv1.StatefulSetList{}
	err := c.List(ctx, stss, opts...)
	if err != nil {
		return nil, err
	}
	for _, sts := range stss.Items {
		if sts.Status.ReadyReplicas < ptr.Deref(sts.Spec.Replicas, 1) {
			psm[corev1.PodPending] = append(psm[corev1.PodPending], sts.Name)
			return psm, nil
		}
	}

	return getPodsStatus(ctx, c, namespace, name, component)
}

func getComponentsStatus(ctx context.Context, client client.Client, tempo v1alpha1.TempoMicroservices) (v1alpha1.ComponentStatus, error) {
	var err error
	components := v1alpha1.ComponentStatus{}

	components.Compactor, err = getPodsStatus(ctx, client, tempo.Namespace, tempo.Name, "compactor")
	if err != nil {
		return v1alpha1.ComponentStatus{}, fmt.Errorf("cannot get pod status: %w", err)
	}

	components.Distributor, err = getPodsStatus(ctx, client, tempo.Namespace, tempo.Name, "distributor")
	if err != nil {
		return v1alpha1.ComponentStatus{}, fmt.Errorf("cannot get pod status: %w", err)
	}

	components.Ingester, err = getStatefulSetStatus(ctx, client, tempo.Namespace, tempo.Name, "ingester")
	if err != nil {
		return v1alpha1.ComponentStatus{}, fmt.Errorf("cannot get pod status: %w", err)
	}

	components.Querier, err = getPodsStatus(ctx, client, tempo.Namespace, tempo.Name, "querier")
	if err != nil {
		return v1alpha1.ComponentStatus{}, fmt.Errorf("cannot get pod status: %w", err)
	}

	components.QueryFrontend, err = getPodsStatus(ctx, client, tempo.Namespace, tempo.Name, "query-frontend")
	if err != nil {
		return v1alpha1.ComponentStatus{}, fmt.Errorf("cannot get pod status: %w", err)
	}

	components.Observatorium, err = getPodsStatus(ctx, client, tempo.Namespace, tempo.Name, "observatorium")
	if err != nil {
		return v1alpha1.ComponentStatus{}, fmt.Errorf("cannot get pod status: %w", err)
	}

	return components, nil
}

func conditionStatus(active bool) metav1.ConditionStatus {
	if active {
		return metav1.ConditionTrue
	} else {
		return metav1.ConditionFalse
	}
}

// resetCondition disables the condition if it exists already (without changing any other field of the condition),
// otherwise creates a new disabled condition with a specified reason.
func resetCondition(conditions []metav1.Condition, conditionType v1alpha1.ConditionStatus, defaultReason v1alpha1.ConditionReason) metav1.Condition {
	existingCondition := meta.FindStatusCondition(conditions, string(conditionType))
	if existingCondition != nil {
		// do not modify the condition struct of the slice, otherwise
		// meta.SetStatusCondition() won't update the last transition time
		condition := existingCondition.DeepCopy()
		condition.Status = metav1.ConditionFalse
		return *condition
	} else {
		return metav1.Condition{
			Type:   string(conditionType),
			Reason: string(defaultReason),
			Status: metav1.ConditionFalse,
		}
	}
}

func updateConditions(conditions *[]metav1.Condition, componentsStatus v1alpha1.ComponentStatus, reconcileError error) bool {
	isTerminalError := false

	// set PendingComponents condition if any pod of any component is in pending phase (or running but not ready)
	countPending := len(componentsStatus.Compactor[corev1.PodPending]) +
		len(componentsStatus.Distributor[corev1.PodPending]) +
		len(componentsStatus.Ingester[corev1.PodPending]) +
		len(componentsStatus.Querier[corev1.PodPending]) +
		len(componentsStatus.QueryFrontend[corev1.PodPending]) +
		len(componentsStatus.Observatorium[corev1.PodPending])
	pending := metav1.Condition{
		Type:    string(v1alpha1.ConditionPending),
		Reason:  string(v1alpha1.ReasonPendingComponents),
		Message: messagePending,
		Status:  conditionStatus(countPending > 0),
	}

	// set ConfigurationError condition if the reconcile function returned a ConfigurationError
	var configurationError metav1.Condition
	var cerr *ConfigurationError
	if errors.As(reconcileError, &cerr) {
		configurationError = metav1.Condition{
			Type:    string(v1alpha1.ConditionConfigurationError),
			Reason:  string(cerr.Reason),
			Message: cerr.Message,
			Status:  metav1.ConditionTrue,
		}
		isTerminalError = true
	} else {
		configurationError = resetCondition(*conditions, v1alpha1.ConditionConfigurationError, v1alpha1.ReasonInvalidStorageConfig)
	}

	// set Failed condition if the reconcile function returned any error other than ConfigurationError,
	// or if any pod of any component is in failed phase
	countFailed := len(componentsStatus.Compactor[corev1.PodFailed]) +
		len(componentsStatus.Distributor[corev1.PodFailed]) +
		len(componentsStatus.Ingester[corev1.PodFailed]) +
		len(componentsStatus.Querier[corev1.PodFailed]) +
		len(componentsStatus.QueryFrontend[corev1.PodFailed]) +
		len(componentsStatus.Observatorium[corev1.PodFailed])
	countUnknown := len(componentsStatus.Compactor[corev1.PodUnknown]) +
		len(componentsStatus.Distributor[corev1.PodUnknown]) +
		len(componentsStatus.Ingester[corev1.PodUnknown]) +
		len(componentsStatus.Querier[corev1.PodUnknown]) +
		len(componentsStatus.QueryFrontend[corev1.PodUnknown]) +
		len(componentsStatus.Observatorium[corev1.PodUnknown])
	var failed metav1.Condition
	if reconcileError != nil && cerr == nil {
		failed = metav1.Condition{
			Type:    string(v1alpha1.ConditionFailed),
			Reason:  string(v1alpha1.ReasonFailedReconciliation),
			Message: reconcileError.Error(),
			Status:  metav1.ConditionTrue,
		}
	} else if countFailed > 0 || countUnknown > 0 {
		failed = metav1.Condition{
			Type:    string(v1alpha1.ConditionFailed),
			Reason:  string(v1alpha1.ReasonFailedComponents),
			Message: messageFailed,
			Status:  metav1.ConditionTrue,
		}
	} else {
		failed = resetCondition(*conditions, v1alpha1.ConditionFailed, v1alpha1.ReasonFailedComponents)
	}

	// set Ready condition if all above conditions are false
	ready := metav1.Condition{
		Type:    string(v1alpha1.ConditionReady),
		Reason:  string(v1alpha1.ReasonReady),
		Message: messageReady,
		Status: conditionStatus(
			pending.Status == metav1.ConditionFalse &&
				failed.Status == metav1.ConditionFalse &&
				configurationError.Status == metav1.ConditionFalse,
		),
	}

	meta.SetStatusCondition(conditions, pending)
	meta.SetStatusCondition(conditions, configurationError)
	meta.SetStatusCondition(conditions, failed)
	meta.SetStatusCondition(conditions, ready)
	return isTerminalError
}

func patchStatus(ctx context.Context, c client.Client, original v1alpha1.TempoMicroservices, status v1alpha1.TempoMicroservicesStatus) error {
	patch := client.MergeFrom(&original)
	updated := original.DeepCopy()
	updated.Status = status
	return c.Status().Patch(ctx, updated, patch)
}

// HandleStatus updates the .status field of a TempoMicroservices CR
// Status Conditions API conventions: https://github.com/kubernetes/community/blob/c04227d209633696ad49d7f4546fc8cfd9c660ab/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties
func HandleStatus(ctx context.Context, client client.Client, tempo v1alpha1.TempoMicroservices, reconcileError error) error {
	var err error
	log := ctrl.LoggerFrom(ctx)
	status := *tempo.Status.DeepCopy()

	status.Components, err = getComponentsStatus(ctx, client, tempo)
	if err != nil {
		log.Error(err, "could not get status of each component")
	}

	isTerminalError := updateConditions(&status.Conditions, status.Components, reconcileError)
	if isTerminalError {
		// wrap error in reconcile.TerminalError to indicate human intervention is required
		// and the request should not be requeued.
		reconcileError = reconcile.TerminalError(reconcileError)
	}

	err = patchStatus(ctx, client, tempo, status)
	if err != nil {
		return err
	}

	return reconcileError
}
