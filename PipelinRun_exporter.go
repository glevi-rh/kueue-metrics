package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	// Prometheus client
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	// Tekton and Kubernetes clients
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// --- Exporter Configuration ---

var (
	// A list of all known states to set inactive ones to 0
	allPossibleStates = []string{
		"Succeeded", "Failed", "Running", "PipelineRunPending",
		"PipelineRunCancelled", v1.PipelineRunReasonPending, "Unknown",
	}

	// Environment variable for the scrape interval
	scrapeIntervalEnvVar = "SCRAPE_INTERVAL"
)

// --- Prometheus Collector Implementation ---

// PipelineRunCollector implements the prometheus.Collector interface
type PipelineRunCollector struct {
	tektonClient tektonclientset.Interface
	prStatusDesc *prometheus.Desc // Stores the metric description
}

// NewPipelineRunCollector creates a new collector
func NewPipelineRunCollector(client tektonclientset.Interface) *PipelineRunCollector {
	return &PipelineRunCollector{
		tektonClient: client,
		// Create the metric description.
		prStatusDesc: prometheus.NewDesc(
			"tekton_kueue_pipelinerun_status",
			"The status of a PipelineRun, labeled by namespace, name, status, and build platform.",
			// These are the Prometheus labels
			[]string{"namespace", "name", "status", "build_platform"},
			nil, // No constant labels
		),
	}
}

// Describe sends the metric description to the Prometheus channel.
// It's part of the prometheus.Collector interface.
func (c *PipelineRunCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.prStatusDesc
}

// Collect is called by Prometheus *every time* the /metrics endpoint is scraped.
// This is where we talk to the API server and generate metrics on-the-fly.
func (c *PipelineRunCollector) Collect(ch chan<- prometheus.Metric) {
	log.Println("Collecting metrics from Kubernetes API...")

	// 1. LIST all PipelineRuns in all namespaces.
	// This is the "query" to our data source.
	prList, err := c.tektonClient.TektonV1().PipelineRuns("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error listing PipelineRuns: %v", err)
		return // Don't send any metrics if API call fails
	}

	// 2. Iterate over all PipelineRuns
	for _, pr := range prList.Items {
		// 3. Get the status and labels (using the helpers from below)
		currentStatus := getPipelineRunStatus(&pr)
		platformLabel := getBuildPlatformLabel(&pr)

		// 4. Create the "StateSet" metric
		// We loop over all known states and set the active one to 1 and inactive ones to 0.
		for _, state := range allPossibleStates {
			var value float64 // 0.0
			if state == currentStatus {
				value = 1.0 // 1.0 for the active state
			}

			// 5. Send the metric to the channel
			ch <- prometheus.MustNewConstMetric(
				c.prStatusDesc,      // The metric description
				prometheus.GaugeValue, // It's a Gauge
				value,               // The value (0 or 1)
				// The label values, in the same order as defined in the Desc
				pr.Namespace,
				pr.Name,
				state,
				platformLabel,
			)
		}
	}
	log.Printf("Collected metrics for %d PipelineRuns.", len(prList.Items))
}

// --- Helper Functions ---

// getPipelineRunStatus safely extracts the status string from the PipelineRun.
func getPipelineRunStatus(pr *v1.PipelineRun) string {
	if len(pr.Status.Conditions) > 0 {
		for _, c := range pr.Status.Conditions {
			if c.Type == v1.PipelineRunReasonSucceeded {
				return c.Reason
			}
		}
	}
	if pr.IsPending() {
		return v1.PipelineRunReasonPending
	}
	return "Unknown"
}

// getBuildPlatformLabel extracts the "build-platforms" param and formats it as a label.
func getBuildPlatformLabel(pr *v1.PipelineRun) string {
	for _, param := range pr.Spec.Params {
		if param.Name == "build-platforms" {
			// Join the array ["linux/amd64", "linux/arm64"]
			// into a single string "linux/amd64,linux/arm64"
			return strings.Join(param.Value.ArrayVal, ",")
		}
	}
	return "" // Return empty string if not found
}

// --- Main Function (like your example) ---

// KubeConfig loads Kubernetes config, prioritizing in-cluster, then fallback to kubeconfig file.
func KubeConfig() (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}
	log.Printf("InClusterConfig failed: %v. Trying kubeconfig...", err)

	// Fallback to kubeconfig file
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		kubeconfigPath = os.ExpandEnv("$HOME/.kube/config")
	}
	
	return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
}

func main() {
	// 1. Get Kubernetes Config (like your example gets DB config)
	config, err := KubeConfig()
	if err != nil {
		log.Fatalf("FATAL: Error loading Kubernetes config: %v", err)
	}

	// 2. Connect to the data source (Kube API)
	tektonClient, err := tektonclientset.NewForConfig(config)
	if err != nil {
		log.Fatalf("FATAL: Error creating Tekton clientset: %v", err)
	}
	log.Println("Successfully connected to the Kubernetes API.")

	// 3. Create a new Registry (like your example)
	reg := prometheus.NewRegistry()

	// 4. Create and Register the Collector
	// This is the key part. We register the *collector* itself, not global gauges.
	prCollector := NewPipelineRunCollector(tektonClient)
	reg.MustRegister(prCollector)

	// 5. Expose the registered metrics via HTTP (like your example)
	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	// 6. Start the server
	log.Println("Prometheus exporter starting on :9090/metrics ...")
	if err := http.ListenAndServe(":9090", nil); err != nil {
		log.Fatalf("FATAL: Error starting Prometheus HTTP server: %v", err)
	}
}
