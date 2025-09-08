package middleware

import (
	"net/http"
	"strconv"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sharath018/temple-management-backend/config"
	"github.com/sharath018/temple-management-backend/internal/auth"
)

// AuthMiddleware handles JWT authentication and sets up access context
func AuthMiddleware(cfg *config.Config, authSvc auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid Authorization header"})
			return
		}

		tokenStr := parts[1]
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTAccessSecret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
			return
		}

		userIDFloat, ok := claims["user_id"].(float64)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user_id missing in token"})
			return
		}

		userID := uint(userIDFloat)
		user, err := authSvc.GetUserByID(userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}

		// Set user in context
		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Set("claims", claims)

		// Extract entity ID from path if available
		entityIDFromPath := ExtractEntityIDFromPath(c)

		// FIXED: Proper tenant and entity resolution
		assignedTenantID := ResolveTenantID(c, user, claims)
		
		// Create access context with enhanced devotee handling
		accessContext := ResolveAccessContextWithDevotee(c, user, assignedTenantID, entityIDFromPath)
		c.Set("access_context", accessContext)

		c.Next()
	}
}

// ExtractEntityIDFromPath attempts to extract an entity ID from URL paths like /entity/123/...
func ExtractEntityIDFromPath(c *gin.Context) *uint {
	path := c.Request.URL.Path
	if strings.Contains(path, "/entity/") {
		parts := strings.Split(path, "/")
		for i, part := range parts {
			if part == "entity" && i+1 < len(parts) {
				entityIDFromPath, err := strconv.ParseUint(parts[i+1], 10, 32)
				if err == nil {
					uintID := uint(entityIDFromPath)
					return &uintID
				}
			}
		}
	}
	return nil
}

// FIXED: Simplified tenant ID resolution
func ResolveTenantID(c *gin.Context, user auth.User, claims jwt.MapClaims) *uint {
	// Priority 1: URL parameter (for specific tenant access)
	if tenantIDParam := c.Param("id"); tenantIDParam != "" && tenantIDParam != "all" {
		if tid, err := strconv.ParseUint(tenantIDParam, 10, 32); err == nil {
			id := uint(tid)
			return &id
		}
	}

	// Priority 2: Query parameter
	if tenantQuery := c.Query("tenant_id"); tenantQuery != "" && tenantQuery != "all" {
		if tid, err := strconv.ParseUint(tenantQuery, 10, 32); err == nil {
			id := uint(tid)
			return &id
		}
	}

	// Priority 3: Header (X-Tenant-ID)
	if tenantHeader := c.GetHeader("X-Tenant-ID"); tenantHeader != "" && tenantHeader != "all" {
		if tid, err := strconv.ParseUint(tenantHeader, 10, 32); err == nil {
			id := uint(tid)
			return &id
		}
	}

	// Priority 4: JWT token assigned_tenant_id
	if assignedTenantIDFloat, exists := claims["assigned_tenant_id"]; exists {
		if tenantID, ok := assignedTenantIDFloat.(float64); ok && tenantID > 0 {
			id := uint(tenantID)
			return &id
		}
	}

	// For standarduser and monitoringuser, they must have an assigned tenant
	if user.Role.RoleName == "standarduser" || user.Role.RoleName == "monitoringuser" {
		// Return their own entity ID as the assigned tenant
		return user.EntityID
	}

	return nil
}

// ResolveAccessContextWithDevotee creates an access context with special handling for devotee roles
func ResolveAccessContextWithDevotee(c *gin.Context, user auth.User, assignedTenantID, entityIDFromPath *uint) AccessContext {
	// Start with basic access context
	accessContext := AccessContext{
		UserID:   user.ID,
		RoleName: user.Role.RoleName,
	}

	// Handle permissions and entity access based on role
	switch user.Role.RoleName {
	case RoleSuperAdmin:
		accessContext.PermissionType = "full"
		accessContext.AssignedEntityID = assignedTenantID
		
	case RoleTempleAdmin:
		accessContext.PermissionType = "full"
		accessContext.DirectEntityID = user.EntityID
		
	// Modified part in ResolveAccessContextWithDevotee function
case RoleStandardUser:
    accessContext.PermissionType = "full"
    // Ensure we set the assigned tenant ID for entity access
    if assignedTenantID != nil {
        accessContext.AssignedEntityID = assignedTenantID
        // Log assigned tenant ID for debugging
        fmt.Printf("Standard user assigned to tenant ID: %d\n", *assignedTenantID)
    } else {
        fmt.Printf("WARNING: Standard user has no assigned tenant ID\n")
    }
		
	case RoleMonitoringUser:
		accessContext.PermissionType = "readonly"
		accessContext.AssignedEntityID = assignedTenantID
		
	case RoleDevotee, RoleVolunteer:
		// Devotees and volunteers get readonly access
		accessContext.PermissionType = "readonly"
		
		// Try multiple approaches to find the entity ID, in order of priority:
		if entityIDFromPath != nil {
			// 1. Entity ID from URL path (e.g., /entity/123/devotee/dashboard)
			accessContext.AssignedEntityID = entityIDFromPath
		} else if entityHeader := c.GetHeader("X-Entity-ID"); entityHeader != "" {
			// 2. Entity ID from X-Entity-ID header
			if eid, err := strconv.ParseUint(entityHeader, 10, 32); err == nil {
				id := uint(eid)
				accessContext.AssignedEntityID = &id
			}
		} else if assignedTenantID != nil {
			// 3. Tenant ID (often the same as entity ID)
			accessContext.AssignedEntityID = assignedTenantID
		} else if user.EntityID != nil {
			// 4. User's own entity ID
			accessContext.DirectEntityID = user.EntityID
		}
	}

	return accessContext
}