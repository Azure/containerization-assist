/**
 * Built-in Knowledge Packs
 * All knowledge packs imported as JSON modules for reliable loading
 */

import azureContainerAppsPack from '../../knowledge/packs/azure-container-apps-pack.json';
import baseImagesPack from '../../knowledge/packs/base-images-pack.json';
import buildOptimization from '../../knowledge/packs/build-optimization.json';
import databasePack from '../../knowledge/packs/database-pack.json';
import dockerfileAdvanced from '../../knowledge/packs/dockerfile-advanced.json';
import dotnetBackgroundJobsPack from '../../knowledge/packs/dotnet-background-jobs-pack.json';
import dotnetBlazorPack from '../../knowledge/packs/dotnet-blazor-pack.json';
import dotnetEfCorePack from '../../knowledge/packs/dotnet-ef-core-pack.json';
import dotnetFramework48Pack from '../../knowledge/packs/dotnet-framework-48-pack.json';
import dotnetFrameworkPack from '../../knowledge/packs/dotnet-framework-pack.json';
import dotnetGrpcPack from '../../knowledge/packs/dotnet-grpc-pack.json';
import dotnetIdentityPack from '../../knowledge/packs/dotnet-identity-pack.json';
import dotnetMediatrPack from '../../knowledge/packs/dotnet-mediatr-pack.json';
import dotnetPack from '../../knowledge/packs/dotnet-pack.json';
import dotnetSignalrPack from '../../knowledge/packs/dotnet-signalr-pack.json';
import dotnetWorkerPack from '../../knowledge/packs/dotnet-worker-pack.json';
import goPack from '../../knowledge/packs/go-pack.json';
import javaPack from '../../knowledge/packs/java-pack.json';
import kubernetesDeployment from '../../knowledge/packs/kubernetes-deployment.json';
import kubernetesPack from '../../knowledge/packs/kubernetes-pack.json';
import nodejsPack from '../../knowledge/packs/nodejs-pack.json';
import phpPack from '../../knowledge/packs/php-pack.json';
import pythonPack from '../../knowledge/packs/python-pack.json';
import rubyPack from '../../knowledge/packs/ruby-pack.json';
import rustPack from '../../knowledge/packs/rust-pack.json';
import securityPack from '../../knowledge/packs/security-pack.json';
import securityRemediation from '../../knowledge/packs/security-remediation.json';
import starterPack from '../../knowledge/packs/starter-pack.json';

export interface BuiltInPack {
  name: string;
  data: unknown;
}

/**
 * All built-in knowledge packs
 * These are loaded as JSON modules at import time
 */
export const BUILTIN_PACKS: BuiltInPack[] = [
  { name: 'azure-container-apps-pack.json', data: azureContainerAppsPack },
  { name: 'base-images-pack.json', data: baseImagesPack },
  { name: 'build-optimization.json', data: buildOptimization },
  { name: 'database-pack.json', data: databasePack },
  { name: 'dockerfile-advanced.json', data: dockerfileAdvanced },
  { name: 'dotnet-background-jobs-pack.json', data: dotnetBackgroundJobsPack },
  { name: 'dotnet-blazor-pack.json', data: dotnetBlazorPack },
  { name: 'dotnet-ef-core-pack.json', data: dotnetEfCorePack },
  { name: 'dotnet-framework-48-pack.json', data: dotnetFramework48Pack },
  { name: 'dotnet-framework-pack.json', data: dotnetFrameworkPack },
  { name: 'dotnet-grpc-pack.json', data: dotnetGrpcPack },
  { name: 'dotnet-identity-pack.json', data: dotnetIdentityPack },
  { name: 'dotnet-mediatr-pack.json', data: dotnetMediatrPack },
  { name: 'dotnet-pack.json', data: dotnetPack },
  { name: 'dotnet-signalr-pack.json', data: dotnetSignalrPack },
  { name: 'dotnet-worker-pack.json', data: dotnetWorkerPack },
  { name: 'go-pack.json', data: goPack },
  { name: 'java-pack.json', data: javaPack },
  { name: 'kubernetes-deployment.json', data: kubernetesDeployment },
  { name: 'kubernetes-pack.json', data: kubernetesPack },
  { name: 'nodejs-pack.json', data: nodejsPack },
  { name: 'php-pack.json', data: phpPack },
  { name: 'python-pack.json', data: pythonPack },
  { name: 'ruby-pack.json', data: rubyPack },
  { name: 'rust-pack.json', data: rustPack },
  { name: 'security-pack.json', data: securityPack },
  { name: 'security-remediation.json', data: securityRemediation },
  { name: 'starter-pack.json', data: starterPack },
];
