package cmd

func init() {
	registerStubSetGetCmd(
		"settings",
		"System settings (stub until orchestrator supports)",
		"/v1/settings",
		"/v1/settings",
	)
}
