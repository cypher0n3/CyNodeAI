package cmd

func init() {
	registerStubListGetCmd(
		"nodes",
		"Node management (stub until orchestrator supports)",
		"List nodes",
		"Get node details",
		"/v1/nodes",
	)
}
