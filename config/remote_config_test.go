package config

const (
	account               = "account"
	org                   = "org"
	project               = "project"
	environmentIdentifier = "env"
	defaultAPIKey         = "key1"
	defaultEnvironmentID  = "0000-0000-0000-0000-0000"
	// this jwt base64 encodes the defaultEnvironmentID, environmentIdentifier and project
	validJWT = "header.eyJlbnZpcm9ubWVudCI6IjAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMCIsImVudmlyb25tZW50SWRlbnRpZmllciI6ImVudiIsInByb2plY3RJZGVudGlmaWVyIjoicHJvamVjdCIsImNsdXN0ZXJJZGVudGlmaWVyIjoiMiJ9.signature"
	validKey = "valid_key"
)
