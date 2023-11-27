package testhelpers

import (
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"
)

// AuthToken used to authenticate with the remote service
var AuthToken string
var ProxyAuthToken string

// SetAuthToken ...
func SetAuthToken(token string) {
	AuthToken = token
}

// GetAuthToken ...
func GetAuthToken() string {
	return AuthToken
}

// SetProxyAuthToken ...
func SetProxyAuthToken(token string) {
	ProxyAuthToken = token
}

func GetProxyAuthToken() string {
	return ProxyAuthToken
}

// GetDefaultAccount ...
func GetDefaultAccount() string {
	return os.Getenv("DEFAULT_ACCOUNT")
}

// GetDefaultOrg ...
func GetDefaultOrg() string {
	return os.Getenv("DEFAULT_ORG")
}

// GetSecondaryOrg ...
func GetSecondaryOrg() string {
	return os.Getenv("SECONDARY_ORG_IDENTIFIER")
}

// GetDefaultEnvironment ...
func GetDefaultEnvironment() string {
	return os.Getenv("DEFAULT_ENVIRONMENT")
}

// GetSecondaryEnvironment ...
func GetSecondaryEnvironment() string {
	return os.Getenv("SECONDARY_ENVIRONMENT")
}

// GetClientURL ...
func GetClientURL() string {
	return os.Getenv("CLIENT_URL")
}

// GetClientURL ...
func GetAdminURL() string {
	return os.Getenv("ADMIN_URL")
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

// GetProxyKeyIdentifier ...
func GetProxyKeyIdentifier() string {
	return os.Getenv("PROXY_KEY_IDENTIFIER")
}
