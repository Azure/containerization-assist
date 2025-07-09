package core

// initializeConsolidatedTools creates and returns all consolidated command tools
// Currently disabled due to import cycle issues between commands/services/state packages
func (s *serverImpl) initializeConsolidatedTools() []interface{} {
	s.logger.Info("Consolidated tools disabled due to import cycle - using legacy tools")
	return make([]interface{}, 0)
}

// registerAllConsolidatedTools registers all consolidated tools with gomcp
func (s *serverImpl) registerAllConsolidatedTools() {
	s.logger.Info("Consolidated tools registration skipped - using legacy gomcp tools")
}
