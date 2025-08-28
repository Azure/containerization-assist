#!/bin/bash

# Error recovery hook for mcphost
# Instructs the LLM to continue when tool calls fail with suggestions for recovery

# Log hook execution start
echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR-RECOVERY HOOK: Starting execution" >> /tmp/hook_execution.log

# Read JSON input from stdin
input=$(cat)

# Log the input received for debugging
echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR-RECOVERY HOOK: Received input: $(echo "$input" | head -c 200)..." >> /tmp/hook_execution.log

# Check for various types of errors and conversation states
is_error=$(echo "$input" | jq -r '.error // empty')
tool_response=$(echo "$input" | jq -r '.tool_response // empty')
tool_name=$(echo "$input" | jq -r '.tool_name // empty')
conversation_state=$(echo "$input" | jq -r '.state // empty')
waiting_for_input=$(echo "$input" | jq -r '.waiting_for_input // empty')

# Log extracted values
echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR-RECOVERY HOOK: is_error='$is_error', tool_name='$tool_name', conversation_state='$conversation_state', waiting_for_input='$waiting_for_input'" >> /tmp/hook_execution.log

# Check if the conversation is waiting for user input
user_input_needed="false"
if [ "$waiting_for_input" = "true" ] || [ "$conversation_state" = "waiting" ]; then
    user_input_needed="true"
fi

# Log user input detection
echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR-RECOVERY HOOK: user_input_needed='$user_input_needed'" >> /tmp/hook_execution.log

# Also check for patterns that indicate the LLM is asking for user input
if echo "$input" | grep -qi "would you like me to\|should I\|do you want\|what would you like\|enter your prompt"; then
    user_input_needed="true"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR-RECOVERY HOOK: Detected user input pattern in text" >> /tmp/hook_execution.log
fi

# Check if tool response contains error indicators
if echo "$tool_response" | grep -qi "error\|failed\|exception\|invalid\|validation failed\|deployment.*failed"; then
    is_error="true"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR-RECOVERY HOOK: Detected error pattern in tool response" >> /tmp/hook_execution.log
fi

# If there's an error, inject recovery instructions
if [ -n "$is_error" ] && [ "$is_error" != "null" ] && [ "$is_error" != "false" ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR-RECOVERY HOOK: Processing error recovery" >> /tmp/hook_execution.log
    # Create recovery guidance based on the tool that failed
    recovery_message=""
    
    # Check for specific error patterns first
    if echo "$tool_response" | grep -qi "deployment validation failed"; then
        recovery_message="Deployment validation failed. Please investigate pod status with kubectl commands, check logs, and continue with troubleshooting or proceed to verify_deployment to get more detailed status."
    elif echo "$tool_name" | grep -qi "containerization-assist"; then
        recovery_message="Tool call failed. Please continue with the workflow by trying the suggested steps or using alternative approaches. Check the error message for specific guidance and retry with corrected parameters if needed."
    else
        recovery_message="The previous tool call encountered an error. Please analyze the error message, adjust your approach if necessary, and continue with the next logical step in the workflow."
    fi
    
    # Create modified response with recovery instructions
    output=$(echo "$input" | jq --arg recovery "$recovery_message" '
        .tool_response = (.tool_response // "") + "\n\n🔄 RECOVERY GUIDANCE: " + $recovery + " Continue with the next step in your workflow."
    ')
    
    echo "$output"
elif [ "$user_input_needed" = "true" ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR-RECOVERY HOOK: Processing user input auto-continue" >> /tmp/hook_execution.log
    # Inject auto-continue guidance when LLM is waiting for user input
    auto_continue_message="Please continue with the containerization workflow. Proceed with the next logical step based on the current situation."
    
    output=$(echo "$input" | jq --arg continue_msg "$auto_continue_message" '
        . + {"auto_continue": $continue_msg}
    ')
    
    echo "$output"
else
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR-RECOVERY HOOK: No errors detected, passing through unchanged" >> /tmp/hook_execution.log
    # No error detected, pass through unchanged
    echo "$input"
fi

echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR-RECOVERY HOOK: Execution completed" >> /tmp/hook_execution.log

# Always exit successfully to allow continuation
exit 0
