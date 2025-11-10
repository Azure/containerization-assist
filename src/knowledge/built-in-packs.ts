/**
 * Built-in Knowledge Packs
 * All knowledge packs imported as TypeScript modules for reliable loading
 */

import azureContainerAppsPack from '../../knowledge/packs/azure-container-apps-pack';
import baseImagesPack from '../../knowledge/packs/base-images-pack';
import buildOptimization from '../../knowledge/packs/build-optimization';
import databasePack from '../../knowledge/packs/database-pack';
import dockerfileAdvanced from '../../knowledge/packs/dockerfile-advanced';
import dotnetBackgroundJobsPack from '../../knowledge/packs/dotnet-background-jobs-pack';
import dotnetBlazorPack from '../../knowledge/packs/dotnet-blazor-pack';
import dotnetEfCorePack from '../../knowledge/packs/dotnet-ef-core-pack';
import dotnetFramework48Pack from '../../knowledge/packs/dotnet-framework-48-pack';
import dotnetFrameworkPack from '../../knowledge/packs/dotnet-framework-pack';
import dotnetGrpcPack from '../../knowledge/packs/dotnet-grpc-pack';
import dotnetIdentityPack from '../../knowledge/packs/dotnet-identity-pack';
import dotnetMediatrPack from '../../knowledge/packs/dotnet-mediatr-pack';
import dotnetPack from '../../knowledge/packs/dotnet-pack';
import dotnetSignalrPack from '../../knowledge/packs/dotnet-signalr-pack';
import dotnetWorkerPack from '../../knowledge/packs/dotnet-worker-pack';
import goPack from '../../knowledge/packs/go-pack';
import javaPack from '../../knowledge/packs/java-pack';
import kubernetesDeployment from '../../knowledge/packs/kubernetes-deployment';
import kubernetesPack from '../../knowledge/packs/kubernetes-pack';
import nodejsPack from '../../knowledge/packs/nodejs-pack';
import phpPack from '../../knowledge/packs/php-pack';
import pythonPack from '../../knowledge/packs/python-pack';
import rubyPack from '../../knowledge/packs/ruby-pack';
import rustPack from '../../knowledge/packs/rust-pack';
import securityPack from '../../knowledge/packs/security-pack';
import securityRemediation from '../../knowledge/packs/security-remediation';
import starterPack from '../../knowledge/packs/starter-pack';

export interface BuiltInPack {
  name: string;
  data: unknown;
}

/**
 * All built-in knowledge packs
 * These are loaded as TypeScript modules at build time
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
