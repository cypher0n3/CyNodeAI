package cmd

func init() {
	registerStubListCmd(
		"audit",
		"Audit logs (stub until orchestrator supports)",
		"List audit events",
		"/v1/audit",
	)
}
