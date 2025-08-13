package integration

// K8sDeploymentYAML is a minimal Deployment used by integration tests.
const K8sDeploymentYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  namespace: default
spec:
  selector:
    matchLabels:
      app: app
  replicas: 1
  template:
    metadata:
      labels:
        app: app
    spec:
      containers:
      - name: app
        image: localhost:5001/app:latest
        ports:
        - containerPort: 8080
`

// K8sServiceYAML is a minimal Service used by integration tests.
const K8sServiceYAML = `apiVersion: v1
kind: Service
metadata:
  name: app
  namespace: default
spec:
  selector:
    app: app
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
`

// BasicK8sManifests returns the shared minimal Deployment and Service manifests.
func BasicK8sManifests() map[string]interface{} {
	return map[string]interface{}{
		"deployment.yaml": K8sDeploymentYAML,
		"service.yaml":    K8sServiceYAML,
	}
}

// K8sIngressYAML is a minimal Ingress used by integration tests.
const K8sIngressYAML = `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app
  namespace: default
spec:
  rules:
  - http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: app
            port:
              number: 80
`

// BasicK8sManifestsWithIngress returns Deployment, Service, and Ingress manifests.
func BasicK8sManifestsWithIngress() map[string]interface{} {
	return map[string]interface{}{
		"deployment.yaml": K8sDeploymentYAML,
		"service.yaml":    K8sServiceYAML,
		"ingress.yaml":    K8sIngressYAML,
	}
}
