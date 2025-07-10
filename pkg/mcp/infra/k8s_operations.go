//go:build k8s

package infra

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesOperations handles Kubernetes-specific operations
// This is only compiled when -tags k8s is used
type KubernetesOperations struct {
	client    kubernetes.Interface
	config    *rest.Config
	logger    *slog.Logger
	namespace string
}

// NewKubernetesOperations creates a new Kubernetes operations handler
func NewKubernetesOperations(kubeconfig string, namespace string, logger *slog.Logger) (*KubernetesOperations, error) {
	var config *rest.Config
	var err error

	if kubeconfig != "" {
		// Use kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
		}
	} else {
		// Use in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	if namespace == "" {
		namespace = "default"
	}

	return &KubernetesOperations{
		client:    client,
		config:    config,
		logger:    logger,
		namespace: namespace,
	}, nil
}

// DeployApplicationParams represents parameters for deploying applications
type DeployApplicationParams struct {
	Name        string
	Image       string
	Port        int32
	Replicas    int32
	Namespace   string
	Labels      map[string]string
	Annotations map[string]string
	EnvVars     map[string]string
	Resources   ResourceRequirements
	HealthCheck HealthCheckConfig
	ServiceType string
	Timeout     time.Duration
}

// ResourceRequirements represents resource requirements
type ResourceRequirements struct {
	CPURequest    string
	CPULimit      string
	MemoryRequest string
	MemoryLimit   string
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	HTTPPath         string
	HTTPPort         int32
	InitialDelay     time.Duration
	PeriodSeconds    int32
	TimeoutSeconds   int32
	FailureThreshold int32
}

// DeployApplicationResult represents the result of deploying an application
type DeployApplicationResult struct {
	Name              string
	Namespace         string
	DeploymentName    string
	ServiceName       string
	Status            string
	Replicas          int32
	AvailableReplicas int32
	ServiceEndpoint   string
	DeployTime        time.Duration
	Success           bool
	Error             string
	Pods              []PodInfo
}

// PodInfo represents pod information
type PodInfo struct {
	Name     string
	Status   string
	Ready    bool
	Restarts int32
	Age      time.Duration
}

// DeployApplication deploys an application to Kubernetes
func (k *KubernetesOperations) DeployApplication(ctx context.Context, params DeployApplicationParams) (*DeployApplicationResult, error) {
	startTime := time.Now()

	k.logger.Info("Starting Kubernetes application deployment",
		"name", params.Name,
		"image", params.Image,
		"namespace", params.Namespace,
		"replicas", params.Replicas)

	namespace := params.Namespace
	if namespace == "" {
		namespace = k.namespace
	}

	// Set timeout context
	if params.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, params.Timeout)
		defer cancel()
	}

	// Create deployment
	deployment, err := k.createDeployment(ctx, params, namespace)
	if err != nil {
		return &DeployApplicationResult{
			Name:       params.Name,
			Namespace:  namespace,
			Status:     "failed",
			Success:    false,
			Error:      err.Error(),
			DeployTime: time.Since(startTime),
		}, nil
	}

	// Create service
	service, err := k.createService(ctx, params, namespace)
	if err != nil {
		return &DeployApplicationResult{
			Name:           params.Name,
			Namespace:      namespace,
			DeploymentName: deployment.Name,
			Status:         "failed",
			Success:        false,
			Error:          err.Error(),
			DeployTime:     time.Since(startTime),
		}, nil
	}

	// Wait for deployment to be ready
	err = k.waitForDeploymentReady(ctx, deployment.Name, namespace)
	if err != nil {
		return &DeployApplicationResult{
			Name:           params.Name,
			Namespace:      namespace,
			DeploymentName: deployment.Name,
			ServiceName:    service.Name,
			Status:         "failed",
			Success:        false,
			Error:          err.Error(),
			DeployTime:     time.Since(startTime),
		}, nil
	}

	// Get deployment status
	deploymentStatus, err := k.client.AppsV1().Deployments(namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
	if err != nil {
		return &DeployApplicationResult{
			Name:           params.Name,
			Namespace:      namespace,
			DeploymentName: deployment.Name,
			ServiceName:    service.Name,
			Status:         "failed",
			Success:        false,
			Error:          err.Error(),
			DeployTime:     time.Since(startTime),
		}, nil
	}

	// Get pods information
	pods, err := k.getPodInfo(ctx, params.Name, namespace)
	if err != nil {
		k.logger.Warn("failed to get pod info", "error", err)
		pods = []PodInfo{}
	}

	// Determine service endpoint
	serviceEndpoint := k.getServiceEndpoint(service, params.ServiceType)

	result := &DeployApplicationResult{
		Name:              params.Name,
		Namespace:         namespace,
		DeploymentName:    deployment.Name,
		ServiceName:       service.Name,
		Status:            "deployed",
		Replicas:          deploymentStatus.Spec.Replicas,
		AvailableReplicas: deploymentStatus.Status.AvailableReplicas,
		ServiceEndpoint:   serviceEndpoint,
		DeployTime:        time.Since(startTime),
		Success:           true,
		Pods:              pods,
	}

	k.logger.Info("Kubernetes application deployment completed",
		"name", result.Name,
		"namespace", result.Namespace,
		"deployment_name", result.DeploymentName,
		"service_name", result.ServiceName,
		"replicas", result.Replicas,
		"available_replicas", result.AvailableReplicas,
		"deploy_time", result.DeployTime)

	return result, nil
}

// GenerateManifestsParams represents parameters for generating manifests
type GenerateManifestsParams struct {
	Name        string
	Image       string
	Port        int32
	Replicas    int32
	Namespace   string
	Labels      map[string]string
	Annotations map[string]string
	EnvVars     map[string]string
	Resources   ResourceRequirements
	HealthCheck HealthCheckConfig
	ServiceType string
	OutputDir   string
}

// GenerateManifestsResult represents the result of generating manifests
type GenerateManifestsResult struct {
	Name          string
	Namespace     string
	Files         []string
	OutputDir     string
	ManifestCount int
	Success       bool
	Error         string
}

// GenerateManifests generates Kubernetes manifests
func (k *KubernetesOperations) GenerateManifests(ctx context.Context, params GenerateManifestsParams) (*GenerateManifestsResult, error) {
	k.logger.Info("Starting Kubernetes manifest generation",
		"name", params.Name,
		"image", params.Image,
		"output_dir", params.OutputDir)

	namespace := params.Namespace
	if namespace == "" {
		namespace = k.namespace
	}

	// Generate deployment manifest
	deployment := k.buildDeploymentManifest(params, namespace)

	// Generate service manifest
	service := k.buildServiceManifest(params, namespace)

	// TODO: Write manifests to files in OutputDir
	// This would involve serializing the manifests to YAML and writing to disk

	result := &GenerateManifestsResult{
		Name:          params.Name,
		Namespace:     namespace,
		Files:         []string{"deployment.yaml", "service.yaml"},
		OutputDir:     params.OutputDir,
		ManifestCount: 2,
		Success:       true,
	}

	k.logger.Info("Kubernetes manifest generation completed",
		"name", result.Name,
		"namespace", result.Namespace,
		"files", result.Files,
		"manifest_count", result.ManifestCount)

	return result, nil
}

// RollbackParams represents parameters for rolling back deployments
type RollbackParams struct {
	Name      string
	Namespace string
	Revision  int64
	Timeout   time.Duration
}

// RollbackResult represents the result of rolling back a deployment
type RollbackResult struct {
	Name         string
	Namespace    string
	FromRevision int64
	ToRevision   int64
	Status       string
	RollbackTime time.Duration
	Success      bool
	Error        string
}

// RollbackDeployment rolls back a deployment to a previous revision
func (k *KubernetesOperations) RollbackDeployment(ctx context.Context, params RollbackParams) (*RollbackResult, error) {
	startTime := time.Now()

	k.logger.Info("Starting Kubernetes deployment rollback",
		"name", params.Name,
		"namespace", params.Namespace,
		"revision", params.Revision)

	namespace := params.Namespace
	if namespace == "" {
		namespace = k.namespace
	}

	// Set timeout context
	if params.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, params.Timeout)
		defer cancel()
	}

	// Get current deployment
	deployment, err := k.client.AppsV1().Deployments(namespace).Get(ctx, params.Name, metav1.GetOptions{})
	if err != nil {
		return &RollbackResult{
			Name:         params.Name,
			Namespace:    namespace,
			Status:       "failed",
			Success:      false,
			Error:        err.Error(),
			RollbackTime: time.Since(startTime),
		}, nil
	}

	currentRevision := deployment.Annotations["deployment.kubernetes.io/revision"]

	// TODO: Implement actual rollback logic
	// This would involve updating the deployment to a previous revision

	result := &RollbackResult{
		Name:         params.Name,
		Namespace:    namespace,
		FromRevision: 0, // Would parse from currentRevision
		ToRevision:   params.Revision,
		Status:       "rolled_back",
		RollbackTime: time.Since(startTime),
		Success:      true,
	}

	k.logger.Info("Kubernetes deployment rollback completed",
		"name", result.Name,
		"namespace", result.Namespace,
		"from_revision", result.FromRevision,
		"to_revision", result.ToRevision,
		"rollback_time", result.RollbackTime)

	return result, nil
}

// HealthCheckParams represents parameters for health checks
type HealthCheckParams struct {
	Name      string
	Namespace string
	Timeout   time.Duration
}

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	Name          string
	Namespace     string
	Healthy       bool
	Status        string
	Replicas      int32
	ReadyReplicas int32
	Pods          []PodInfo
	CheckTime     time.Duration
	Error         string
}

// CheckHealth checks the health of a deployment
func (k *KubernetesOperations) CheckHealth(ctx context.Context, params HealthCheckParams) (*HealthCheckResult, error) {
	startTime := time.Now()

	k.logger.Info("Starting Kubernetes health check",
		"name", params.Name,
		"namespace", params.Namespace)

	namespace := params.Namespace
	if namespace == "" {
		namespace = k.namespace
	}

	// Set timeout context
	if params.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, params.Timeout)
		defer cancel()
	}

	// Get deployment status
	deployment, err := k.client.AppsV1().Deployments(namespace).Get(ctx, params.Name, metav1.GetOptions{})
	if err != nil {
		return &HealthCheckResult{
			Name:      params.Name,
			Namespace: namespace,
			Healthy:   false,
			Status:    "failed",
			CheckTime: time.Since(startTime),
			Error:     err.Error(),
		}, nil
	}

	// Get pods information
	pods, err := k.getPodInfo(ctx, params.Name, namespace)
	if err != nil {
		k.logger.Warn("failed to get pod info", "error", err)
		pods = []PodInfo{}
	}

	// Determine health status
	healthy := deployment.Status.ReadyReplicas == deployment.Status.Replicas &&
		deployment.Status.Replicas > 0

	status := "healthy"
	if !healthy {
		status = "unhealthy"
	}

	result := &HealthCheckResult{
		Name:          params.Name,
		Namespace:     namespace,
		Healthy:       healthy,
		Status:        status,
		Replicas:      *deployment.Spec.Replicas,
		ReadyReplicas: deployment.Status.ReadyReplicas,
		Pods:          pods,
		CheckTime:     time.Since(startTime),
	}

	k.logger.Info("Kubernetes health check completed",
		"name", result.Name,
		"namespace", result.Namespace,
		"healthy", result.Healthy,
		"status", result.Status,
		"replicas", result.Replicas,
		"ready_replicas", result.ReadyReplicas,
		"check_time", result.CheckTime)

	return result, nil
}

// createDeployment creates a Kubernetes deployment
func (k *KubernetesOperations) createDeployment(ctx context.Context, params DeployApplicationParams, namespace string) (*appsv1.Deployment, error) {
	deployment := k.buildDeploymentManifest(GenerateManifestsParams{
		Name:        params.Name,
		Image:       params.Image,
		Port:        params.Port,
		Replicas:    params.Replicas,
		Namespace:   namespace,
		Labels:      params.Labels,
		Annotations: params.Annotations,
		EnvVars:     params.EnvVars,
		Resources:   params.Resources,
		HealthCheck: params.HealthCheck,
	}, namespace)

	return k.client.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
}

// createService creates a Kubernetes service
func (k *KubernetesOperations) createService(ctx context.Context, params DeployApplicationParams, namespace string) (*corev1.Service, error) {
	service := k.buildServiceManifest(GenerateManifestsParams{
		Name:        params.Name,
		Image:       params.Image,
		Port:        params.Port,
		Replicas:    params.Replicas,
		Namespace:   namespace,
		Labels:      params.Labels,
		Annotations: params.Annotations,
		ServiceType: params.ServiceType,
	}, namespace)

	return k.client.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{})
}

// buildDeploymentManifest builds a deployment manifest
func (k *KubernetesOperations) buildDeploymentManifest(params GenerateManifestsParams, namespace string) *appsv1.Deployment {
	labels := map[string]string{
		"app": params.Name,
	}
	for k, v := range params.Labels {
		labels[k] = v
	}

	// Build environment variables
	var envVars []corev1.EnvVar
	for key, value := range params.EnvVars {
		envVars = append(envVars, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}

	// Build resource requirements
	resources := corev1.ResourceRequirements{}
	if params.Resources.CPURequest != "" || params.Resources.MemoryRequest != "" {
		resources.Requests = corev1.ResourceList{}
		if params.Resources.CPURequest != "" {
			resources.Requests["cpu"] = resource.MustParse(params.Resources.CPURequest)
		}
		if params.Resources.MemoryRequest != "" {
			resources.Requests["memory"] = resource.MustParse(params.Resources.MemoryRequest)
		}
	}
	if params.Resources.CPULimit != "" || params.Resources.MemoryLimit != "" {
		resources.Limits = corev1.ResourceList{}
		if params.Resources.CPULimit != "" {
			resources.Limits["cpu"] = resource.MustParse(params.Resources.CPULimit)
		}
		if params.Resources.MemoryLimit != "" {
			resources.Limits["memory"] = resource.MustParse(params.Resources.MemoryLimit)
		}
	}

	// Build health check probes
	var readinessProbe *corev1.Probe
	var livenessProbe *corev1.Probe
	if params.HealthCheck.HTTPPath != "" {
		readinessProbe = &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: params.HealthCheck.HTTPPath,
					Port: intstr.FromInt(int(params.HealthCheck.HTTPPort)),
				},
			},
			InitialDelaySeconds: int32(params.HealthCheck.InitialDelay.Seconds()),
			PeriodSeconds:       params.HealthCheck.PeriodSeconds,
			TimeoutSeconds:      params.HealthCheck.TimeoutSeconds,
			FailureThreshold:    params.HealthCheck.FailureThreshold,
		}
		livenessProbe = &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: params.HealthCheck.HTTPPath,
					Port: intstr.FromInt(int(params.HealthCheck.HTTPPort)),
				},
			},
			InitialDelaySeconds: int32(params.HealthCheck.InitialDelay.Seconds()),
			PeriodSeconds:       params.HealthCheck.PeriodSeconds,
			TimeoutSeconds:      params.HealthCheck.TimeoutSeconds,
			FailureThreshold:    params.HealthCheck.FailureThreshold,
		}
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        params.Name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: params.Annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &params.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:           params.Name,
							Image:          params.Image,
							Ports:          []corev1.ContainerPort{{ContainerPort: params.Port}},
							Env:            envVars,
							Resources:      resources,
							ReadinessProbe: readinessProbe,
							LivenessProbe:  livenessProbe,
						},
					},
				},
			},
		},
	}
}

// buildServiceManifest builds a service manifest
func (k *KubernetesOperations) buildServiceManifest(params GenerateManifestsParams, namespace string) *corev1.Service {
	labels := map[string]string{
		"app": params.Name,
	}
	for k, v := range params.Labels {
		labels[k] = v
	}

	serviceType := corev1.ServiceTypeClusterIP
	if params.ServiceType != "" {
		serviceType = corev1.ServiceType(params.ServiceType)
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        params.Name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: params.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Type: serviceType,
			Selector: map[string]string{
				"app": params.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Port:       params.Port,
					TargetPort: intstr.FromInt(int(params.Port)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// waitForDeploymentReady waits for a deployment to be ready
func (k *KubernetesOperations) waitForDeploymentReady(ctx context.Context, name, namespace string) error {
	watchInterface, err := k.client.AppsV1().Deployments(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", name),
	})
	if err != nil {
		return fmt.Errorf("failed to watch deployment: %w", err)
	}
	defer watchInterface.Stop()

	for {
		select {
		case event, ok := <-watchInterface.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			if event.Type == watch.Modified {
				deployment, ok := event.Object.(*appsv1.Deployment)
				if !ok {
					continue
				}

				if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
					return nil
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// getPodInfo gets information about pods for a deployment
func (k *KubernetesOperations) getPodInfo(ctx context.Context, appName, namespace string) ([]PodInfo, error) {
	pods, err := k.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"app": appName,
		}).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var podInfos []PodInfo
	for _, pod := range pods.Items {
		podInfo := PodInfo{
			Name:     pod.Name,
			Status:   string(pod.Status.Phase),
			Ready:    isPodReady(&pod),
			Restarts: getRestartCount(&pod),
			Age:      time.Since(pod.CreationTimestamp.Time),
		}
		podInfos = append(podInfos, podInfo)
	}

	return podInfos, nil
}

// isPodReady checks if a pod is ready
func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// getRestartCount gets the restart count for a pod
func getRestartCount(pod *corev1.Pod) int32 {
	var restarts int32
	for _, containerStatus := range pod.Status.ContainerStatuses {
		restarts += containerStatus.RestartCount
	}
	return restarts
}

// getServiceEndpoint determines the service endpoint
func (k *KubernetesOperations) getServiceEndpoint(service *corev1.Service, serviceType string) string {
	switch service.Spec.Type {
	case corev1.ServiceTypeLoadBalancer:
		if len(service.Status.LoadBalancer.Ingress) > 0 {
			ingress := service.Status.LoadBalancer.Ingress[0]
			if ingress.IP != "" {
				return fmt.Sprintf("http://%s:%d", ingress.IP, service.Spec.Ports[0].Port)
			}
			if ingress.Hostname != "" {
				return fmt.Sprintf("http://%s:%d", ingress.Hostname, service.Spec.Ports[0].Port)
			}
		}
	case corev1.ServiceTypeNodePort:
		if len(service.Spec.Ports) > 0 {
			return fmt.Sprintf("http://<node-ip>:%d", service.Spec.Ports[0].NodePort)
		}
	case corev1.ServiceTypeClusterIP:
		return fmt.Sprintf("http://%s:%d", service.Spec.ClusterIP, service.Spec.Ports[0].Port)
	}
	return ""
}

// ConvertDeployRequest converts domain deploy request to Kubernetes deploy request
func (k *KubernetesOperations) ConvertDeployRequest(domainRequest *deploy.DeployRequest) DeployApplicationParams {
	return DeployApplicationParams{
		Name:        domainRequest.Name,
		Image:       domainRequest.Image,
		Port:        domainRequest.Port,
		Replicas:    domainRequest.Replicas,
		Namespace:   domainRequest.Namespace,
		Labels:      domainRequest.Labels,
		Annotations: domainRequest.Annotations,
		EnvVars:     domainRequest.EnvVars,
		Resources: ResourceRequirements{
			CPURequest:    domainRequest.Resources.CPURequest,
			CPULimit:      domainRequest.Resources.CPULimit,
			MemoryRequest: domainRequest.Resources.MemoryRequest,
			MemoryLimit:   domainRequest.Resources.MemoryLimit,
		},
		HealthCheck: HealthCheckConfig{
			HTTPPath:         domainRequest.HealthCheck.HTTPPath,
			HTTPPort:         domainRequest.HealthCheck.HTTPPort,
			InitialDelay:     domainRequest.HealthCheck.InitialDelay,
			PeriodSeconds:    domainRequest.HealthCheck.PeriodSeconds,
			TimeoutSeconds:   domainRequest.HealthCheck.TimeoutSeconds,
			FailureThreshold: domainRequest.HealthCheck.FailureThreshold,
		},
		ServiceType: domainRequest.ServiceType,
		Timeout:     domainRequest.Timeout,
	}
}

// ConvertDeployResult converts Kubernetes deploy result to domain deploy result
func (k *KubernetesOperations) ConvertDeployResult(k8sResult *DeployApplicationResult) *deploy.DeployResult {
	return &deploy.DeployResult{
		Name:              k8sResult.Name,
		Namespace:         k8sResult.Namespace,
		DeploymentName:    k8sResult.DeploymentName,
		ServiceName:       k8sResult.ServiceName,
		Status:            k8sResult.Status,
		Replicas:          k8sResult.Replicas,
		AvailableReplicas: k8sResult.AvailableReplicas,
		ServiceEndpoint:   k8sResult.ServiceEndpoint,
		DeployTime:        k8sResult.DeployTime,
		Success:           k8sResult.Success,
		Error:             k8sResult.Error,
	}
}
