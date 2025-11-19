import * as path from 'path';
import { ContainerAssistToolNames } from "./models";
import { PlanStep, PlanStepBulletType } from './planStep';

const dependenciesCheckStep = new PlanStep("Check containerization pre-requisites:", PlanStepBulletType.Numbered);
dependenciesCheckStep.addSteps([
  "Ensure Docker is installed and running."
]);

export function populateGetContainerizationPlanPrompts(
  workspaceFolder: string, 
  servicePaths: string[],
  containerAssistToolNames: ContainerAssistToolNames
): string
{
  const relativeServicePaths = servicePaths.map(servicePath => {
    const relative = path.relative(workspaceFolder, servicePath);
    const isServicePathInWorkspace = !relative.startsWith('..') && !path.isAbsolute(relative);

    // throw if servicePath not in workspaceFolder
    if (!isServicePathInWorkspace) {
      throw new Error(`Service path ${servicePath} is not within workspace folder ${workspaceFolder}. Service paths must be full paths.`);
    }

    return relative;
  });

  const relativeServicePathsString = relativeServicePaths.map(p => `- ${p} (${path.join(workspaceFolder, p).replace(/\\/g, "/")})`).join('\n');

  const containerizationStep = new PlanStep("Steps to containerize the project", PlanStepBulletType.Numbered);
  containerizationStep.addStep(dependenciesCheckStep);

  const repoScanStep = new PlanStep(`Scan the repository using tool '${containerAssistToolNames.analyzeRepo}'.`, PlanStepBulletType.Numbered);
  containerizationStep.addStep(repoScanStep);

  const codeLocalReadiness = new PlanStep("Check code is ready to run in a local container:", PlanStepBulletType.Numbered);
  codeLocalReadiness.addSteps([
    "Ensure the application reads configuration from environment variables. Avoid hardcoding configuration values.",
    "Review the application's dependencies that must be hosted in the cloud, and list these for the user."
  ]);
  containerizationStep.addStep(codeLocalReadiness);

  const dockerFileCreationStep = new PlanStep("Create a Dockerfile for each project:", PlanStepBulletType.Numbered);
  dockerFileCreationStep.addStep(new PlanStep(`Use tool '${containerAssistToolNames.generateDockerfile}' to generate the Dockerfile.`, PlanStepBulletType.Numbered));
  containerizationStep.addStep(dockerFileCreationStep);

  const dockerBuildStep = new PlanStep("Build the Docker images for each project:", PlanStepBulletType.Numbered);

  if (containerAssistToolNames.buildImage !== undefined && containerAssistToolNames.scanImage !== undefined && containerAssistToolNames.tagImage !== undefined) {
    dockerBuildStep.addSteps([
      `Use '${containerAssistToolNames.buildImage}' to build the image.`,
      `Use '${containerAssistToolNames.scanImage}' to scan the built image for any vulnerabilities.`,
      `Use '${containerAssistToolNames.tagImage}' to tag the image appropriately after fixing any vulnerabilities.`
    ]);
  } else {
    dockerBuildStep.addSteps([
      "Use docker build command to build the Docker images (do not run). Tag the images with the proper version."
    ]);
  }
  containerizationStep.addStep(dockerBuildStep);

  const successSummaryStep = new PlanStep("Summarize the successful containerization", PlanStepBulletType.Numbered);
  successSummaryStep.addSteps([
    "List out the dockerfiles created for each project.",
    "Describe to the user the code edits required to successfully containerize the project."
  ]);
  containerizationStep.addStep(successSummaryStep);

  return `
{Agent should fill in and polish the markdown template below to generate a containerization plan for the project. Then save it to '.azure/containerization-plan.copilotmd' file. Don't add extra validation steps! Don't change the tool name!}

# **Containerization Plan**
## **Goal**
Setup Dockerfiles for the project to run inside of containers. Do not set up docker-compose for running locally as it is not required.

## **List of services to be containerized**
{For each service listed, provide a simple description such as language, entrypoint, dependencies, etc.}
${relativeServicePathsString}

## **Execution Step**
> **Below are the steps for Copilot to follow; ask Copilot to update or execute this plan.**
${containerizationStep.renderIntoMarkdown()}
`;
}