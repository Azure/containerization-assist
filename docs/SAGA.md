# Saga Pattern and Its Implementation in Container Kit Workflows

## Understanding the Saga Pattern

The **Saga pattern** is a design approach for maintaining data consistency across a series of operations, especially in distributed or long-running systems. Instead of a single ACID transaction spanning multiple components, a saga breaks the work into a sequence of **local transactions (steps)**. Each step executes independently and, if it succeeds, the saga moves to the next step. If a step fails, the saga invokes predefined **compensating transactions** to undo the effects of all previously completed steps. This ensures the overall system returns to a consistent state even when some operations cannot complete. In essence, _“a saga is a sequence of local transactions... If a step in the sequence fails, the saga performs compensating transactions to undo the completed steps”_ .

**Structure:** In a saga, every step has an associated compensating action that can reverse its changes. Sagas can be implemented via **orchestration** (a central coordinator telling each step when to execute or rollback) or via **choreography** (steps emitting events to trigger the next step). The orchestration approach uses a central saga manager to coordinate the workflow and handle failures centrally . Choreography has no central controller—each service listens for events and acts, which can be harder to trace in complex scenarios . In either case, the saga maintains a log or state machine to track progress (e.g. “started”, “in progress”, “completed”, “compensated”).

**Benefits:**  
- **Failure Isolation and Consistency:** If one step fails, the entire process can be rolled back without affecting other sagas or global state .  
- **Resilience and Fault Tolerance:** With compensating actions defined for each step, the system handles runtime errors gracefully. No global locks or two-phase commit are needed; sagas achieve eventual consistency via retry and rollback logic.  
- **Long-Running Transaction Support:** Sagas allow workflows that span minutes or hours to proceed without holding resources indefinitely. Each step commits immediately, and only if a later step fails are earlier changes undone.  
- **Modularity and Clarity:** Each step explicitly declares its forward and rollback logic, making the overall transaction easier to understand and maintain.  
- **Adaptability:** In AI-driven systems, sagas provide a structured framework for automated retries—fail fast, rollback, analyze, and retry—without residual side effects.

## Illustrative Example of the Saga Pattern

Consider an **e-commerce order** workflow with three steps:  
1. **Reserve Product** – hold the item in inventory.  
2. **Charge Payment** – bill the customer’s card.  
3. **Ship Order** – initiate shipping.

If charging the payment (step 2) fails, the saga compensates step 1 (unreserve the product). If shipping (step 3) fails, it compensates steps 2 (refund payment) and 1 (release inventory) in reverse order, leaving the system as if no order was placed .

    // Pseudocode: E-commerce Order Saga (Orchestrator-style)
    func ProcessOrderSaga(order Order) error {
        // Step 1: Reserve the product
        err := ReserveProduct(order.item)
        if err != nil {
            return fmt.Errorf("order failed: cannot reserve product")
        }

        // Step 2: Charge payment
        err = ChargePayment(order.paymentInfo)
        if err != nil {
            CompensateReserveProduct(order.item)
            return fmt.Errorf("order failed: payment declined (reservation undone)")
        }

        // Step 3: Ship order
        err = ShipOrder(order)
        if err != nil {
            CompensateChargePayment(order.paymentInfo)
            CompensateReserveProduct(order.item)
            return fmt.Errorf("order failed: shipping error (refund & reservation cancelled)")
        }

        return nil  // Saga succeeded
    }

## Saga Pattern Usage in the Codebase (`pkg/mcp`)

1. **Core Types (`pkg/mcp/domain/saga`):**  
   - **`SagaStep` interface:**  
     ```go
     type SagaStep interface {
       Name() string
       Execute(ctx context.Context, data map[string]interface{}) error
       Compensate(ctx context.Context, data map[string]interface{}) error
       CanCompensate() bool
     }
     ```  
     Each step provides forward (`Execute`) and rollback (`Compensate`) logic, and indicates compensation capability .

   - **`SagaExecution`:**  
     Manages saga state, iterating steps and invoking `compensate()` on failure, which undoes completed steps in reverse order. States transition to *Completed*, *Compensated*, or *Aborted* depending on outcomes .

   - **`SagaCoordinator`:**  
     Starts sagas asynchronously (`StartSaga`), tracks active sagas, publishes start/completion events, and allows manual cancellation (`CancelSaga`) which triggers compensation like a failure .

2. **Workflow Middleware (`pkg/mcp/domain/workflow`):**  
   - **Step-Level Middleware:** Wraps `CompensatableStep`s in a `workflowSagaStepAdapter` that implements `SagaStep`, recording state before execution and invoking the step’s `Compensate` on rollback .  
   - **Workflow-Level Wrapper:** `WorkflowSagaMiddleware` starts a saga (assigning an ID and empty step list), enriches context, runs the workflow, and on error calls `CancelSaga` to rollback all executed steps . The `sagaDecorator` applies this wrapper to every orchestrator execution .

## Why the Saga Pattern Benefits This System

- **Resilience in Long-Running Workflows:** Ensures multi-step container build/deploy processes clean up on failure, avoiding orphaned resources.  
- **Isolation of Failures:** Contained within each saga instance; one failed workflow does not affect others.  
- **AI-Enhanced Recovery:** Clean rollback provides a fresh baseline for the AI error handler to analyze and retry safely .  
- **Modularity and Maintainability:** Clear separation of forward and rollback logic; middleware keeps core workflow code uncluttered.  
- **Operational Transparency:** Saga events/logs track each step’s outcome and any compensations, aiding debugging.

## Summary

The Saga pattern provides **all-or-nothing** semantics across distributed workflow steps. In Container Kit’s `pkg/mcp`, it ensures that complex, long-running operations either complete successfully or automatically clean up, enabling a robust, self-healing orchestration engine in modern cloud-native environments .

