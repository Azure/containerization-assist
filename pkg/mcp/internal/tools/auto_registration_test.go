package tools

import (
	"fmt"
	"testing"
)

func TestAutoRegistrationAdapter(t *testing.T) {
	fmt.Println("🧪 Testing Auto-Registration System")
	fmt.Println("===================================")
	
	adapter := NewAutoRegistrationAdapter()
	
	readyTools := adapter.GetReadyToolNames()
	pendingTools := adapter.GetPendingToolNames()
	
	fmt.Printf("✅ Ready for auto-registration: %d tools\n", len(readyTools))
	for _, tool := range readyTools {
		fmt.Printf("   - %s\n", tool)
	}
	
	fmt.Printf("\n⏳ Pending interface migration: %d tools\n", len(pendingTools))
	for _, tool := range pendingTools {
		fmt.Printf("   - %s\n", tool)
	}
	
	fmt.Printf("\n📊 Total atomic tools discovered: %d\n", len(readyTools) + len(pendingTools))
	fmt.Println("🎯 Auto-registration system operational!")
	
	// Verify we have the expected number of tools
	if len(readyTools) != 11 {
		t.Errorf("Expected 11 ready tools, got %d", len(readyTools))
	}
	
	if len(pendingTools) != 0 {
		t.Errorf("Expected 0 pending tools, got %d", len(pendingTools))
	}
}