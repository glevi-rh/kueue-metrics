### This should be added in https://github.com/konflux-ci/tekton-kueue/tree/main/internal/controller as a *pipelinerun_metrics_controller*

package controller

import (
	"context"

	"github.com/konflux-ci/tekton-queue/internal/cel" 

	tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PipelineRunMetricsReconciler reconciles a PipelineRun object to update metrics
type PipelineRunMetricsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tekton.dev,resources=pipelineruns,verbs=get;list;watch

// Reconcile is the main loop that updates metrics for PipelineRuns
func (r *PipelineRunMetricsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. Fetch the PipelineRun object
	var pr tekv1.PipelineRun
	if err := r.Get(ctx, req.NamespacedName, &pr); err != nil {
		if errors.IsNotFound(err) {
			// Object was deleted. This is our hook to clean up metrics.
			logger.Info("PipelineRun not found, deleting metrics", "pipelinerun", req.NamespacedName)
			
			// We can't pass the object, but we can pass its name and namespace
			// to DeletePartialMatch (which is what DeletePipelineRunStatusMetric uses)
			// Let's create a helper in cel/pipelinerun_metrics.go
			
			// --- CLEANUP METRIC ---
			cel.DeletePipelineRunStatusMetricByNamespacedName(req.NamespacedName)
			
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// 2. Check for Deletion (Finalizer Pattern)
	// This is the *other* deletion hook. It runs when the object is
	// "terminating" but still exists.
	if !pr.ObjectMeta.DeletionTimestamp.IsZero() {
		// PipelineRun is being deleted.
		logger.Info("PipelineRun is deleting, removing metrics", "pipelinerun", pr.Name)
		
		// --- CLEANUP METRIC ---
		cel.DeletePipelineRunStatusMetric(&pr)
		
		return ctrl.Result{}, nil
	}

	// 3. Update the metric for the current state (Create/Update)
	// This will be called on every change, including status updates.
	
	// --- UPDATE METRIC ---
	cel.UpdatePipelineRunStatusMetric(&pr)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineRunMetricsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tekv1.PipelineRun{}).
		Complete(r)
}
