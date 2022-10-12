package testhelpers

import (
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"
)

// AuthToken used to authenticate with the remote service
var AuthToken string

// SetAuthToken ...
func SetAuthToken(token string) {
	AuthToken = token
}

// GetAuthToken ...
func GetAuthToken() string {
	return AuthToken
}

// GetDefaultAccount ...
func GetDefaultAccount() string {
	return os.Getenv("DEFAULT_ACCOUNT")
}

// GetDefaultOrg ...
func GetDefaultOrg() string {
	return os.Getenv("DEFAULT_ORG")
}

// GetDefaultEnvironment ...
func GetDefaultEnvironment() string {
	return os.Getenv("DEFAULT_ENVIRONMENT")
}

// GetClientURL ...
func GetClientURL() string {
	return os.Getenv("CLIENT_URL")
}

// IsPlaformEnabled ...
func IsPlaformEnabled() bool {
	enabled, err := strconv.ParseBool(os.Getenv("IS_PLATFORM_ENABLED"))
	if err != nil {
		log.Warn("Couldn't parse IS_PLATFORM_ENABLED environment variable as bool")
	}
	return enabled
}

// GetUserAccessToken ...
func GetUserAccessToken() string {
	return os.Getenv("USER_ACCESS_TOKEN")
}

// GetPlatformBaseURL ...
func GetPlatformBaseURL() string {
	return os.Getenv("PLATFORM_BASE_URL")
}
