package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"time"
)

const (
	metricsPrefix = "pod_image_aging"
)

// Define Prometheus metrics
var (
	oldestImageSeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: fmt.Sprintf("%s_oldest_seconds", metricsPrefix),
			Help: "The number of seconds since the oldest image in the namespace was created",
		},
		[]string{"namespace"},
	)
	youngestImageSeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: fmt.Sprintf("%s_youngest_seconds", metricsPrefix),
			Help: "The number of seconds since the youngest image in the namespace was created",
		},
		[]string{"namespace"},
	)
	averageImageSeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: fmt.Sprintf("%s_average_seconds", metricsPrefix),
			Help: "The average number of seconds since the images in the namespace were created",
		},
		[]string{"namespace"},
	)
)

func init() {
	ctrlmetrics.Registry.MustRegister(oldestImageSeconds, youngestImageSeconds, averageImageSeconds)
}

func UpdateMetrics(c client.Client, namespace string, log logr.Logger) error {
	pods := &corev1.PodList{}
	err := c.List(context.TODO(), pods, client.InNamespace(namespace))
	if err != nil {
		log.Error(err, "Failed to list pods")
		return err
	}

	var imageCreationDates []time.Time
	for _, pod := range pods.Items {
		if !hasStatusAnnotation(&pod) {
			continue
		}

		status := &StatusAnnotation{}
		if err := json.Unmarshal([]byte(pod.Annotations[getAnnotationKey("status")]), status); err != nil {
			return err
		}

		for _, container := range status.Containers {
			createdDate, err := time.Parse(time.RFC3339, container.CreatedAt)
			if err == nil {
				imageCreationDates = append(imageCreationDates, createdDate)
			}
		}
	}

	if len(imageCreationDates) > 0 {
		now := time.Now()
		oldest := now.Sub(imageCreationDates[0]).Seconds()
		youngest := oldest
		var total float64

		for _, date := range imageCreationDates {
			seconds := now.Sub(date).Seconds()
			if seconds > oldest {
				oldest = seconds
			}
			if seconds < youngest {
				youngest = seconds
			}
			total += seconds
		}

		avg := total / float64(len(imageCreationDates))

		// Update the metrics
		oldestImageSeconds.WithLabelValues(namespace).Set(oldest)
		youngestImageSeconds.WithLabelValues(namespace).Set(youngest)
		averageImageSeconds.WithLabelValues(namespace).Set(avg)
	}

	return nil
}
