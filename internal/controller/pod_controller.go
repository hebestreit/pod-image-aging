/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/types"
	"github.com/go-logr/logr"
	"github.com/hebestreit/pod-image-aging/internal/cache"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	types2 "k8s.io/apimachinery/pkg/types"
	"regexp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"slices"
	"strings"
	"time"
)

const (
	domain = "pod-image-aging.hbst.io"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Cache  *cache.Cache
	Opts   *Opts
}

type Opts struct {
	IncludeNamespacesFilter string
	ExcludeNamespacesFilter string
	IncludeImagesFilter     string
	ExcludeImagesFilter     string
	CacheExpiration         time.Duration
	DockerAuthConfigPath    string
}

type StatusAnnotation struct {
	Containers     []Container `json:"containers,omitempty"`
	InitContainers []Container `json:"initContainers,omitempty"`
}

type Container struct {
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	pod := &corev1.Pod{}
	if err := r.Get(ctx, req.NamespacedName, pod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if pod.Annotations[getAnnotationKey("status")] != "" ||
		pod.Annotations[getAnnotationKey("ignore")] == "true" ||
		pod.ObjectMeta.DeletionTimestamp != nil ||
		pod.Status.Phase != corev1.PodRunning {
		return ctrl.Result{}, nil
	}

	opts := r.Opts
	includeNamespaces := strings.Split(opts.IncludeNamespacesFilter, ",")
	excludeNamespaces := strings.Split(opts.ExcludeNamespacesFilter, ",")
	includeImages := strings.Split(opts.IncludeImagesFilter, ",")
	excludeImages := strings.Split(opts.ExcludeImagesFilter, ",")

	if (opts.IncludeNamespacesFilter != "" && !slices.Contains(includeNamespaces, pod.Namespace)) || (opts.ExcludeNamespacesFilter != "" && slices.Contains(excludeNamespaces, pod.Namespace)) {
		return ctrl.Result{}, nil
	}

	nodeName := pod.Spec.NodeName
	if nodeName == "" {
		return ctrl.Result{}, nil
	}

	var node corev1.Node
	if err := r.Client.Get(ctx, client.ObjectKey{Name: nodeName}, &node); err != nil {
		return ctrl.Result{}, err
	}

	var containers []Container
containerStatuses:
	for _, container := range pod.Status.ContainerStatuses {
		if (opts.IncludeImagesFilter != "" && !isImageInWildcardFilter(container.Image, includeImages)) ||
			(opts.ExcludeImagesFilter != "" && isImageInWildcardFilter(container.Image, excludeImages)) {
			continue containerStatuses
		}

		imageCreated, err := getImageCreatedAt(ctx, r.Cache, l, container, node, opts)
		if err != nil {
			return ctrl.Result{}, err
		}

		containers = append(containers, Container{
			Name:      container.Name,
			CreatedAt: imageCreated.Format(time.RFC3339),
		})
	}

	var initContainers []Container
initContainerStatuses:
	for _, container := range pod.Status.InitContainerStatuses {
		if (opts.IncludeImagesFilter != "" && !isImageInWildcardFilter(container.Image, includeImages)) ||
			(opts.ExcludeImagesFilter != "" && isImageInWildcardFilter(container.Image, excludeImages)) {
			continue initContainerStatuses
		}

		imageCreated, err := getImageCreatedAt(ctx, r.Cache, l, container, node, opts)
		if err != nil {
			return ctrl.Result{}, err
		}

		initContainers = append(initContainers, Container{
			Name:      container.Name,
			CreatedAt: imageCreated.Format(time.RFC3339),
		})
	}

	if len(containers) == 0 && len(initContainers) == 0 {
		return ctrl.Result{}, nil
	}

	jsonString, err := json.Marshal(StatusAnnotation{Containers: containers, InitContainers: initContainers})
	if err != nil {
		return ctrl.Result{}, err
	}

	patchData, err := json.Marshal(
		metav1.PartialObjectMetadata{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					getAnnotationKey("status"): string(jsonString),
				},
			},
		},
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Client.Patch(ctx, pod, client.RawPatch(types2.StrategicMergePatchType, patchData)); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc:  func(e event.CreateEvent) bool { return false },
			UpdateFunc:  func(e event.UpdateEvent) bool { return true },
			DeleteFunc:  func(e event.DeleteEvent) bool { return false },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		}).
		// TODO check if this can be scaled in a single worker and how horizontally
		//WithOptions(controller.Options{
		//	MaxConcurrentReconciles: 5,
		//}).
		Complete(r)
}

func inspectImage(ctx context.Context, container *corev1.ContainerStatus, os, architecture string, dockerAuthConfigPath string) (*types.ImageInspectInfo, error) {
	sysCtx := &types.SystemContext{
		ArchitectureChoice:       architecture,
		OSChoice:                 os,
		DockerCompatAuthFilePath: dockerAuthConfigPath,
	}

	imageName := container.ImageID
	if strings.Contains(imageName, "@sha256:") && strings.Contains(imageName, ":") {
		// Split the image name by '@' to separate the digest
		parts := strings.Split(imageName, "@")
		// Split the first part by ':' to remove the tag
		nameParts := strings.Split(parts[0], ":")
		// Reconstruct the image name without the tag
		if len(nameParts) > 1 {
			imageName = nameParts[0] + "@" + parts[1]
		}
	}

	ref, err := docker.ParseReference("//" + imageName)
	if err != nil {
		return nil, fmt.Errorf("error parsing image reference %s: %w", imageName, err)
	}

	img, err := ref.NewImage(ctx, sysCtx)
	if err != nil {
		return nil, fmt.Errorf("error creating image %s: %w", imageName, err)
	}
	defer img.Close()

	imgInspect, err := img.Inspect(ctx)
	if err != nil {
		return nil, fmt.Errorf("error inspecting image %s: %w", imageName, err)
	}

	return imgInspect, nil
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

func getImageCreatedAt(ctx context.Context, cache *cache.Cache, l logr.Logger, container corev1.ContainerStatus, node corev1.Node, opts *Opts) (*time.Time, error) {
	imageCreated, found := cache.Get(container.ImageID)
	if found {
		l.Info("Using cached image creation date", "Name", container.Name, "ImageID", container.ImageID, "Created", imageCreated)
		return imageCreated, nil
	}

	l.Info("Inspecting image", "Name", container.Name, "ImageID", container.ImageID)
	imgInspect, err := inspectImage(ctx, &container, node.Labels["kubernetes.io/os"], node.Labels["kubernetes.io/arch"], opts.DockerAuthConfigPath)
	if err != nil {
		return nil, err
	}

	if imgInspect.Created.IsZero() {
		return nil, fmt.Errorf("image creation date is zero")
	}

	l.Info("Image inspected", "Name", container.Name, "ImageID", container.ImageID, "Created", imgInspect.Created)

	cache.Set(container.ImageID, *imgInspect.Created, &opts.CacheExpiration)
	return imgInspect.Created, nil
}
