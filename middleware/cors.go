package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	// CRITICAL: Cannot use AllowAllOrigins with AllowCredentials
	// When AllowCredentials=true, must specify explicit origins (not wildcard *)
	// Otherwise browsers will refuse to send cookies
	config.AllowOrigins = []string{
		"http://localhost:3000",
		"http://127.0.0.1:3000",
		"http://localhost:5173", // Vite dev server
		"http://127.0.0.1:5173",
	}
	config.AllowCredentials = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"*"}
	return cors.New(config)
}
