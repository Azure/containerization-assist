#!/bin/bash

# Script to add context parameters to session methods in command files

FILES=(
    "pkg/mcp/application/commands/build_consolidated.go"
    "pkg/mcp/application/commands/scan_consolidated.go"
    "pkg/mcp/application/commands/deploy_consolidated.go"
)

for file in "${FILES[@]}"; do
    echo "Processing $file..."
    
    # Update getSessionWorkspace method signature
    sed -i 's/func (cmd \*Consolidated\w*Command) getSessionWorkspace(sessionID string) (string, error) {/func (cmd *Consolidated\w*Command) getSessionWorkspace(ctx context.Context, sessionID string) (string, error) {/g' "$file"
    
    # Update GetSessionMetadata calls
    sed -i 's/cmd\.sessionState\.GetSessionMetadata(sessionID)/cmd.sessionState.GetSessionMetadata(ctx, sessionID)/g' "$file"
    
    # Update getSessionWorkspace calls
    sed -i 's/cmd\.getSessionWorkspace(\([^)]*\)\.SessionID)/cmd.getSessionWorkspace(ctx, \1.SessionID)/g' "$file"
    
    # Update updateSessionState method signature
    sed -i 's/func (cmd \*Consolidated\w*Command) updateSessionState(sessionID string, result/func (cmd *Consolidated\w*Command) updateSessionState(ctx context.Context, sessionID string, result/g' "$file"
    
    # Update UpdateSessionData calls
    sed -i 's/cmd\.sessionState\.UpdateSessionData(sessionID, stateUpdate)/cmd.sessionState.UpdateSessionData(ctx, sessionID, stateUpdate)/g' "$file"
    
    # Update updateSessionState calls
    sed -i 's/cmd\.updateSessionState(\([^,]*\)\.SessionID, /cmd.updateSessionState(ctx, \1.SessionID, /g' "$file"
done

echo "Done!"