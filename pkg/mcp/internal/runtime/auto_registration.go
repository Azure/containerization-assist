package runtime

import (
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/core"
	errors "github.com/Azure/container-kit/pkg/mcp/internal"
)

func RegisterAllTools(registry interface{}) error {
	if reg, ok := registry.(interface {
		Register(name string, factory func() interface{}) error
	}); ok {
		tools := core.GetRegisteredTools()
		for name, factory := range tools {
			if err := reg.Register(name, func() interface{} { return factory() }); err != nil {
				return errors.NewError().Message(fmt.Sprintf("failed to register tool %s", name)).Cause(err).Build()
			}
		}
		return nil
	}
	return errors.NewError().Messagef("registry does not implement required Register method").WithLocation().Build()
}
func GetAllToolNames() []string {
	return core.GetRegisteredToolNames()
}
func GetToolCount() int {
	tools := core.GetRegisteredTools()
	return len(tools)
}
