package cmd

func init() {
	registerStubListCmd(
		"creds",
		"Credential management (stub until orchestrator supports)",
		"List credentials metadata",
		"/v1/creds",
	)
}
