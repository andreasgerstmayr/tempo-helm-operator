package controller

import (
	"context"
	"encoding/json"
	"fmt"

	helmclient "github.com/operator-framework/helm-operator-plugins/pkg/client"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	tempov1alpha1 "github.com/andreasgerstmayr/tempo-helm-operator/api/v1alpha1"
)

// TempoMicroservicesReconciler reconciles a TempoMicroservices object
type TempoMicroservicesReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	ActionConfigGetter helmclient.ActionConfigGetter
	ActionClientGetter helmclient.ActionClientGetter
}

//+kubebuilder:rbac:groups=tempo.grafana.com,resources=tempomicroservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tempo.grafana.com,resources=tempomicroservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tempo.grafana.com,resources=tempomicroservices/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *TempoMicroservicesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	tempo := tempov1alpha1.TempoMicroservices{}
	if err := r.Get(ctx, req.NamespacedName, &tempo); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "unable to fetch TempoMicroservices")
			return ctrl.Result{}, fmt.Errorf("could not fetch TempoMicroservices: %w", err)
		}

		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, nil
	}

	chart, err := loader.Load("helm-charts/tempo-distributed")
	if err != nil {
		return ctrl.Result{}, err
	}

	var vals chartutil.Values
	err = json.Unmarshal(tempo.Spec.Values.Raw, &vals)
	if err != nil {
		return ctrl.Result{}, err
	}

	// merge values from CR with default values of chart
	vals, err = chartutil.CoalesceValues(chart, vals)
	if err != nil {
		return ctrl.Result{}, err
	}

	manifests, err := r.renderHelmChart(chart, &tempo, vals)
	if err != nil {
		return ctrl.Result{}, err
	}

	mtlsEnabled, _ := vals.PathValue("server.mtls.enabled")
	if mtlsEnabled == true {
		certs, err := createCerts(ctx, r.Client, tempo)
		if err != nil {
			return ctrl.Result{}, err
		}
		manifests = append(manifests, certs...)
	}

	err = reconcileManagedObjects(context.Background(), r.Client, &tempo, r.Scheme, manifests, map[types.UID]client.Object{})
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TempoMicroservicesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tempov1alpha1.TempoMicroservices{}).
		Complete(r)
}
