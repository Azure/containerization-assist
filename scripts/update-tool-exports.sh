#!/bin/bash

# Update push-image
echo "export { pushImageTool, pushImage } from './tool';
export { pushImageSchema, type PushImageParams } from './schema';" > src/tools/push-image/index.ts

# Update tag-image  
echo "export { tagImageTool, tagImage } from './tool';
export { tagImageSchema, type TagImageParams } from './schema';" > src/tools/tag-image/index.ts

# Update generate-k8s-manifests
echo "export { generateK8sManifestsTool, generateK8sManifests } from './tool';
export { generateK8sManifestsSchema, type GenerateK8sManifestsParams } from './schema';" > src/tools/generate-k8s-manifests/index.ts

# Update verify-deployment
echo "export { verifyDeploymentTool, verifyDeployment } from './tool';
export { verifyDeploymentSchema, type VerifyDeploymentParams } from './schema';" > src/tools/verify-deployment/index.ts

echo "Exports updated!"
