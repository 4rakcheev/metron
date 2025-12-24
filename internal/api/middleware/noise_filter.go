package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// NoiseFilter filters out scanner/hacker noise from logs
// Returns true if the request should be filtered (not logged)
func NoiseFilter(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		method := c.Request.Method

		// Process request first
		c.Next()

		// Don't filter authenticated requests
		if c.GetBool("authenticated") {
			return
		}

		status := c.Writer.Status()

		// Filter out scanner noise:
		// 1. 404 on non-existing routes (scanners looking for vulnerabilities)
		// 2. Invalid HTTP methods on valid routes
		// 3. Common scanner paths
		if status == http.StatusNotFound && isScannerPath(path) {
			// Abort logging for scanner paths
			c.Set("skip_logging", true)
			return
		}

		// Filter out invalid methods
		if status == http.StatusMethodNotAllowed {
			c.Set("skip_logging", true)
			return
		}

		// Filter out requests to common scanner paths
		if isScannerPath(path) && status >= 400 {
			c.Set("skip_logging", true)
			return
		}

		// Log blocked request at debug level
		if c.GetBool("skip_logging") {
			logger.Debug("Scanner request filtered",
				"path", path,
				"method", method,
				"status", status,
				"client_ip", c.ClientIP())
		}
	}
}

// isScannerPath checks if a path is commonly used by scanners
func isScannerPath(path string) bool {
	scannerPaths := []string{
		"/admin",
		"/phpmyadmin",
		"/wp-admin",
		"/wp-login",
		"/.env",
		"/.git",
		"/config",
		"/backup",
		"/test",
		"/debug",
		"/.aws",
		"/console",
		"/api/v1/console",
		"/actuator",
		"/manager",
		"/cgi-bin",
		"/.well-known",
		"/robots.txt",
		"/favicon.ico",
		"/sitemap.xml",
	}

	lowercasePath := strings.ToLower(path)
	for _, scannerPath := range scannerPaths {
		if strings.HasPrefix(lowercasePath, scannerPath) {
			return true
		}
	}

	// Check for file extensions commonly probed by scanners
	scannerExtensions := []string{
		".php",
		".asp",
		".aspx",
		".jsp",
		".bak",
		".old",
		".sql",
		".zip",
		".tar",
		".gz",
	}

	for _, ext := range scannerExtensions {
		if strings.HasSuffix(lowercasePath, ext) {
			return true
		}
	}

	return false
}
