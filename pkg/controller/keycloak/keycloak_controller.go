package keycloak

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/controller"

	keycloakv1alpha1 "github.com/keycloak/keycloak-operator/pkg/apis/keycloak/v1alpha1"
	"github.com/keycloak/keycloak-operator/pkg/common"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_keycloak")

const (
	RequeueDelaySeconds = 30
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Keycloak Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	client := mgr.GetClient()

	return &ReconcileKeycloak{
		client:  client,
		scheme:  mgr.GetScheme(),
		context: ctx,
		cancel:  cancel,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("keycloak-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Some resource types may not exist on the cluster when this operator starts.
	// This autodetect background ticker continually checks if the resource type exists and setup the controller watch if it does
	if d, err := common.NewAutoDetect(mgr, c); err != nil {
		log.Error(err, "failed to start the background process to auto-detect the operator capabilities")
	} else {
		d.Start()
	}

	// Watch for changes to primary resource Keycloak
	err = c.Watch(&source.Kind{Type: &keycloakv1alpha1.Keycloak{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileKeycloak implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileKeycloak{}

// ReconcileKeycloak reconciles a Keycloak object
type ReconcileKeycloak struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client  client.Client
	scheme  *runtime.Scheme
	context context.Context
	cancel  context.CancelFunc
}

// Reconcile reads that state of the cluster for a Keycloak object and makes changes based on the state read
// and what is in the Keycloak.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileKeycloak) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Keycloak")

	// Fetch the Keycloak instance
	instance := &keycloakv1alpha1.Keycloak{}
	err := r.client.Get(r.context, request.NamespacedName, instance)
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

	// Read current state
	currentState := common.NewClusterState()
	err = currentState.Read(r.context, instance, r.client)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Get Action to reconcile current state into desired state
	reconciler := NewKeycloakReconciler()
	desiredState, err := reconciler.Reconcile(currentState, instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	actionRunner := common.NewClusterActionRunner(r.context, r.client, r.scheme, instance)
	err = actionRunner.RunAll(desiredState)
	if err != nil {
		return reconcile.Result{}, err
	}

	log.Info("desired cluster state met")
	return reconcile.Result{RequeueAfter: RequeueDelaySeconds * time.Second}, nil
}
