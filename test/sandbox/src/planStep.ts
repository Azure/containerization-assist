export enum PlanStepBulletType {
  Numbered,
  Bulleted
}

export class PlanStep {
  private bulletType: PlanStepBulletType;
  private stepName: string;
  private children: PlanStep[];

  constructor(stepName: string, bulletType: PlanStepBulletType) {
    this.stepName = stepName;
    this.children = [];
    this.bulletType = bulletType;
  }

  public addStep(planStep: PlanStep | string): PlanStep {
    if (typeof planStep === "string") {
      planStep = new PlanStep(planStep, this.bulletType);
    }

    this.children.push(planStep);
    return planStep;
  }

  public addSteps(planSteps: (PlanStep | string)[]) {
    for (const step of planSteps) {
      if (typeof step === "string") {
        this.children.push(new PlanStep(step, this.bulletType));
      } else {
        this.children.push(step);
      }
    }
  }

  public render(): string[] {
    const lines: string[] = [];

    const hasChildren: boolean = this.children.length > 0;

    lines.push(`${this.stepName}${hasChildren && !this.stepName.endsWith(":") ? ":" : ""}`);

    for (let i = 0; i < this.children.length; i++) {
      const child = this.children[i].render();

      lines.push(...child.map((line, index) => {
        if (index === 0) {
          const heading = this.bulletType === PlanStepBulletType.Numbered ? `${i+1}.` : `-`;

          return `${heading} ${line}`; // add a number to the heading line
        }

        return `    ${line}`; // add indentation for child lines
      }));
    }

    return lines;
  }

  public renderIntoMarkdown(): string {
    return this.render().join("\n");
  }
}