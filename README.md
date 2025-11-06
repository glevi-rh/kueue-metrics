# kueue-metrics

## Purpose of this repo
Currently, we have limited observability about [Tekton](https://tekton.dev/docs/) `PipelineRuns` statuses and the reasons behind them. It will help us to understand better the reasons of the failures and investigate the reasons behind a pending `PipelineRun` and if we can do something about it. Those are the statuses and reasons of a `PipelineRun`:
### Status: "Unknown"
* Pending
* Started
* Running
* Cancelled

### Status: "True"
* Succeeded
* Completed

### Status: "False"
* Failed
* Cancelled
* PipelineRunTimeout
* CreateRunFailed
* [Error Message] (e.g., `PipelineRunValidationFailed`)

For further information, you can go to [Tekton's monitoring execution status documentation](https://tekton.dev/docs/pipelines/pipelineruns/#monitoring-execution-status).

## Suggestions on how to implement
We have 3 suggestions with examples in this repo:

### Update/Add a Configmap to `kube-state-metrics`

Theoretically, we can add a custom resources state metrics to `openshift-monitoring` namespace by adding a dedicated configmap that will have the new metric. Here you can see [our current example of a PipelineRun metrics configmap](https://github.com/glevi-rh/kueue-metrics/blob/main/tekton_pipelines_metrics_configmap.yaml).

### Add a `PipelineRun` metric controller to [tekton-kueue](https://github.com/konflux-ci/tekton-kueue)

In order to monitor the `PipelineRun` objects and update the metrics, we can add a controller that does that. We'll need to add the new controller in [tekton-kueue/internal/controller/](https://github.com/konflux-ci/tekton-kueue/tree/main/internal/controller) folder, add a reference in [main.go](https://github.com/konflux-ci/tekton-kueue/blob/main/cmd/main.go), and add the new metric in [tekton-kueue metrics.go](https://github.com/konflux-ci/tekton-kueue/blob/main/internal/cel/metrics.go). Here you can see [our current example of a metrics controller](https://github.com/glevi-rh/kueue-metrics/blob/main/pipelinerun_metrics_controller.go).

### Create a dedicated [tekton-kueue](https://github.com/konflux-ci/tekton-kueue) Prometheus exporter

We can create a Prometheus exporter that will expose custom metrics of our choice. It will communicate directly with Kubernetes API to get the current data and it'll tell us the `PipelineRun` status and reason. Here you can see [our current example of a PipelineRun exporter](https://github.com/glevi-rh/kueue-metrics/blob/main/PipelineRun_exporter.go).


> NOTE: all the files were made with Gemini with some further refinement, we should not treat the examples as real implementation suggestions, if we decide to apply one of these approaches we should build something new.
