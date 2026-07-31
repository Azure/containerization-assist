# Agent-eval multi-fixture run reports

These are the self-contained HTML gradient reports for the multi-fixture
agent-eval sweeps referenced in
[`../summer-2026-work-summary.md`](../summer-2026-work-summary.md). Open any
`runN.html` in a browser — each is standalone (no server needed).

Every sweep runs a matrix of **fixtures × models × levels × reps**. The three
levels are always:

| Level | Label | What it is |
|---|---|---|
| `bare` | bare | Baseline prompt only. No CA skill, no CA MCP. The control. |
| `mcp` | CA MCP | Baseline + CA `aks-loop` procedural prompt + CA MCP tools. |
| `skills` | CA skills | Baseline + CA `deploy-to-aks` skill bundle + CA MCP tools (real-world delivery). |

All fixtures are Java modernization repos under `test/fixtures/legacy-java/`.

| Run | Date (UTC) | Cells | Models | Fixtures |
|---|---|---:|---|---|
| [run1.html](run1.html) | 2026-06-26 | 36 | gpt-4o, gpt-4.1, gpt-4.1-mini | ejb-ant-monolith, spring-boot-rest-api, spring-mvc-war, wildfly-container-quickstart |
| [run2.html](run2.html) | 2026-06-27 | 108 | gpt-4o, gpt-4.1, gpt-4.1-mini | bob-food-app-frontend, spring-boot-rest-api, spring-mvc-war, wildfly-container-quickstart |
| [run3.html](run3.html) | 2026-06-28 | 108 | gpt-4o, gpt-4.1, gpt-5.4 | bob-food-app-frontend, spring-boot-rest-api, spring-mvc-war, wildfly-container-quickstart |
| [run4.html](run4.html) | 2026-06-28 | 108 | gpt-4o, gpt-4.1, gpt-5.4 | bob-food-app-frontend, spring-boot-rest-api, spring-mvc-war, wildfly-container-quickstart |
| [run5.html](run5.html) | 2026-07-02 | 81 | gpt-4o, gpt-4.1, gpt-5.4 | ejb-ant-monolith, spring-boot-rest-api, spring-mvc-war |
| [run6.html](run6.html) | 2026-07-09 | 27 | gpt-4o, gpt-4.1, gpt-5.4 | ejb-ant-monolith, spring-boot-rest-api, spring-mvc-war |

Notes:

- **run1** is the earliest full fleet sweep (gpt-4-family only), kept as the
  baseline reference point.
- **run3 / run4** are two reps of the same gpt-5.4 fleet matrix on the same day.
- **run6** is the smaller "deploy-nudge" fleet (27 cells) — the run where the
  harness injects a nudge if a cell stops before `verifyDeploy`.
- The findings drawn from these runs are written up in section 5 of
  [`../summer-2026-work-summary.md`](../summer-2026-work-summary.md).
