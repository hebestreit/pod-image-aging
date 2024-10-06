package controller

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"regexp"
	"strings"
)

const (
	domain = "pod-image-aging.hbst.io"
)

func ignorePod(pod *corev1.Pod) bool {
	if hasStatusAnnotation(pod) {
		return true
	}

	if pod.Annotations[getAnnotationKey("ignore")] == "true" {
		return true
	}

	if pod.ObjectMeta.DeletionTimestamp != nil {
		return true
	}

	if pod.Status.Phase != corev1.PodRunning {
		return true
	}

	return false
}

func hasStatusAnnotation(pod *corev1.Pod) bool {
	return pod.Annotations[getAnnotationKey("status")] != ""
}
func getAnnotationKey(path string) string {
	return fmt.Sprintf("%s/%s", domain, path)
}

// wildCardToRegexp converts a wildcard pattern to a regular expression pattern.
func wildCardToRegexp(pattern string) string {
	components := strings.Split(pattern, "*")
	if len(components) == 1 {
		// if len is 1, there are no *'s, return exact match pattern
		return "^" + pattern + "$"
	}
	var result strings.Builder
	for i, literal := range components {

		// Replace * with .*
		if i > 0 {
			result.WriteString(".*")
		}

		// Quote any regular expression meta characters in the
		// literal text.
		result.WriteString(regexp.QuoteMeta(literal))
	}
	return "^" + result.String() + "$"
}

func isImageInWildcardFilter(image string, wildcardFilters []string) bool {
	for _, wildcardFilter := range wildcardFilters {
		if result, _ := regexp.MatchString(wildCardToRegexp(wildcardFilter), image); result {
			return true
		}
	}
	return false
}
