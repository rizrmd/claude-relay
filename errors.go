package clauderelay

import (
	"errors"
	"strings"
)

// Common errors
var (
	ErrNotAuthenticated = errors.New("claude is not authenticated")
	ErrInvalidToken     = errors.New("invalid authentication token")
	ErrAuthRequired     = errors.New("authentication required")
)

// IsAuthenticationError checks if an error is related to authentication issues
func IsAuthenticationError(err error) bool {
	if err == nil {
		return false
	}
	
	// Check for specific authentication errors
	if errors.Is(err, ErrNotAuthenticated) || 
	   errors.Is(err, ErrInvalidToken) || 
	   errors.Is(err, ErrAuthRequired) {
		return true
	}
	
	// Check error message for authentication-related keywords
	errStr := strings.ToLower(err.Error())
	authKeywords := []string{
		"not authenticated",
		"authentication required",
		"invalid token",
		"invalid api key",
		"unauthorized",
		"auth failed",
		"please authenticate",
		"login required",
		"session expired",
	}
	
	for _, keyword := range authKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}
	
	// Check if it's a generic failure that might be auth-related
	// (exit status 1 from claude often means auth issues)
	if strings.Contains(errStr, "exit status 1") && 
	   strings.Contains(errStr, "claude") {
		// This might be an auth issue, but we're not certain
		return true
	}
	
	return false
}