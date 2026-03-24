package database

// Shared test fixture strings for *string fields in database package tests.
var (
	testPrefEmpty          = ""
	testPrefWhitespace     = "   "
	testPrefJSONString     = `"hello"`
	testPrefNumber         = "42"
	testPrefJSONObject     = `{"a":1}`
	testPrefInvalidJSON    = `{invalid`
	testSkillContentUpdate = "# Changed"
	testSystemSettingVal   = "v"
	testSystemSettingUser  = "u"
)
