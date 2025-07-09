package runtime

import (
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/application/commands"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

func RegisterAllTools(registry interface{}) error {
	if reg, ok := registry.(interface {
		Register(name string, factory func() interface{}) error
	}); ok {
		tools := commands.GetRegisteredTools()
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
	tools := commands.GetRegisteredTools()
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	return names
}
func GetToolCount() int {
	tools := commands.GetRegisteredTools()
	return len(tools)
}
