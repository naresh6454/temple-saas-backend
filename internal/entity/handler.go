package entity

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sharath018/temple-management-backend/internal/auth"
	"github.com/sharath018/temple-management-backend/middleware"
)

type Handler struct {
	Service *Service
}

// NewHandler creates a new Entity handler
func NewHandler(s *Service) *Handler {
	return &Handler{Service: s}
}

// Temple Admin → Create Temple (Triggers approval request)
// Temple Admin → Create Temple (Triggers approval request)
// Temple Admin → Create Temple (Triggers approval request)
func (h *Handler) CreateEntity(c *gin.Context) {
    var input Entity

    if err := c.ShouldBindJSON(&input); err != nil {
        log.Printf("Bind Error: %v", err)
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input", "details": err.Error()})
        return
    }

    // Validate required dropdown fields
    if input.TempleType == "" || input.State == "" || input.EstablishedYear == nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Temple Type, State, and Established Year are required"})
        return
    }

    // Get authenticated user
    user, exists := c.Get("user")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
        return
    }
    userObj := user.(auth.User)
    userID := userObj.ID
    
    // Check permissions explicitly to allow standard users
    if userObj.Role.RoleName != "superadmin" && 
       userObj.Role.RoleName != "templeadmin" && 
       userObj.Role.RoleName != "standarduser" {
        c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to create temples"})
        return
    }
    
    // For standard users, get the tenant ID from the X-Tenant-ID header
    createdBy := userID
    if userObj.Role.RoleName == "standarduser" {
        tenantIDHeader := c.GetHeader("X-Tenant-ID")
        if tenantIDHeader != "" {
            // Convert the header value to uint
            if tenantID, err := strconv.ParseUint(tenantIDHeader, 10, 64); err == nil {
                createdBy = uint(tenantID)
                log.Printf("Standard user %d creating temple as tenant %d", userID, createdBy)
            }
        }
    }
    
    // Set the creator ID directly
    input.CreatedBy = createdBy
    
    // 🆕 GET IP ADDRESS FOR AUDIT LOGGING
    ip := middleware.GetIPFromContext(c)

    if err := h.Service.CreateEntity(&input, userID, ip); err != nil {
        log.Printf("Service Error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create entity", "details": err.Error()})
        return
    }

    c.JSON(http.StatusAccepted, gin.H{"message": "Temple registration request submitted successfully"})
}

// Super Admin → View all temples, Temple Admin → View only their created temples
func (h *Handler) GetAllEntities(c *gin.Context) {
	// Get authenticated user
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	user, ok := userVal.(auth.User)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user object"})
		return
	}

	// Get access context
	accessContextVal, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing access context"})
		return
	}
	accessContext, ok := accessContextVal.(middleware.AccessContext)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access context"})
		return
	}

	var entities []Entity
	var err error

	// Check user role - if super admin, get all entities
	if user.Role.RoleName == "superadmin" {
		entities, err = h.Service.GetAllEntities()
	} else if user.Role.RoleName == "templeadmin" {
		// For temple admin, get only entities created by them
		if accessContext.DirectEntityID != nil {
			entities, err = h.Service.GetEntitiesByCreator(user.ID)
		} else {
			c.JSON(http.StatusForbidden, gin.H{"error": "No entity assigned to this admin"})
			return
		}
	} else if user.Role.RoleName == "standarduser" || user.Role.RoleName == "monitoringuser" {
		// For standarduser/monitoringuser, get only the entity they're assigned to
		if accessContext.AssignedEntityID != nil {
			entity, err := h.Service.GetEntityByID(int(*accessContext.AssignedEntityID))
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Assigned entity not found"})
				return
			}
			entities = []Entity{entity}
		} else {
			c.JSON(http.StatusForbidden, gin.H{"error": "No entity assigned to this user"})
			return
		}
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch temples", "details": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, entities)
}

// Anyone → View a specific temple by ID
func (h *Handler) GetEntityByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
		return
	}

	// Get access context
	accessContextVal, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing access context"})
		return
	}
	accessContext, ok := accessContextVal.(middleware.AccessContext)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access context"})
		return
	}

	// Check permissions
	user, _ := c.Get("user")
	userObj := user.(auth.User)
	
	// Allow access if:
	// 1. User is superadmin
	// 2. User is templeadmin and owns this entity
	// 3. User is standarduser/monitoringuser and is assigned to this entity
	entityIDUint := uint(id)
	hasAccess := userObj.Role.RoleName == "superadmin" ||
		(accessContext.DirectEntityID != nil && *accessContext.DirectEntityID == entityIDUint) ||
		(accessContext.AssignedEntityID != nil && *accessContext.AssignedEntityID == entityIDUint)

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this entity"})
		return
	}

	entity, err := h.Service.GetEntityByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Temple not found", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entity)
}

// Temple Admin → Update existing temple
func (h *Handler) UpdateEntity(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
		return
	}

	// Get access context
	accessContextVal, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing access context"})
		return
	}
	accessContext, ok := accessContextVal.(middleware.AccessContext)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access context"})
		return
	}

	// Check write permissions
	if !accessContext.CanWrite() {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have write permissions"})
		return
	}

	var input Entity
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("Update Bind Error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input", "details": err.Error()})
		return
	}

	// Get authenticated user for audit logging
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := user.(auth.User).ID

	// Check if entity ID matches accessible entity
	entityIDUint := uint(id)
	hasAccess := (accessContext.DirectEntityID != nil && *accessContext.DirectEntityID == entityIDUint) ||
		(accessContext.AssignedEntityID != nil && *accessContext.AssignedEntityID == entityIDUint)

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to update this entity"})
		return
	}

	input.ID = uint(id)
	input.UpdatedAt = time.Now()

	// 🆕 GET IP ADDRESS FOR AUDIT LOGGING
	ip := middleware.GetIPFromContext(c)

	if err := h.Service.UpdateEntity(input, userID, ip); err != nil {
		log.Printf("Update Error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update temple", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Temple updated successfully"})
}

// Super Admin → Delete a temple
func (h *Handler) DeleteEntity(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
		return
	}

	// Get authenticated user for audit logging
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := user.(auth.User).ID

	// 🆕 GET IP ADDRESS FOR AUDIT LOGGING
	ip := middleware.GetIPFromContext(c)

	if err := h.Service.DeleteEntity(id, userID, ip); err != nil {
		log.Printf("Delete Error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete temple", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Temple deleted successfully"})
}

// Temple Admin → Get devotees by entity
func (h *Handler) GetDevoteesByEntity(c *gin.Context) {
	entityIDParam := c.Param("id")
	entityIDUint, err := strconv.ParseUint(entityIDParam, 10, 64)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
		return
	}

	// Get access context
	accessContextVal, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing access context"})
		return
	}
	accessContext, ok := accessContextVal.(middleware.AccessContext)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access context"})
		return
	}

	// Check permissions
	entityID := uint(entityIDUint)
	hasAccess := (accessContext.DirectEntityID != nil && *accessContext.DirectEntityID == entityID) ||
		(accessContext.AssignedEntityID != nil && *accessContext.AssignedEntityID == entityID)

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to devotees for this entity"})
		return
	}

	// ✅ Fetch devotees for the given entity
	devotees, err := h.Service.GetDevotees(uint(entityIDUint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch devotees"})
		return
	}

	c.JSON(http.StatusOK, devotees)
}

// Temple Admin → Get devotee statistics for entity
func (h *Handler) GetDevoteeStats(c *gin.Context) {
	entityIDStr := c.Param("id")
	entityIDUint, err := strconv.ParseUint(entityIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
		return
	}

	// Get access context
	accessContextVal, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing access context"})
		return
	}
	accessContext, ok := accessContextVal.(middleware.AccessContext)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access context"})
		return
	}

	// Check permissions
	entityID := uint(entityIDUint)
	hasAccess := (accessContext.DirectEntityID != nil && *accessContext.DirectEntityID == entityID) ||
		(accessContext.AssignedEntityID != nil && *accessContext.AssignedEntityID == entityID)

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to devotee stats for this entity"})
		return
	}

	stats, err := h.Service.GetDevoteeStats(entityID)
	if err != nil {
		log.Printf("Error fetching devotee stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch devotee stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// Temple Admin → Update devotee membership status
// PATCH /entities/:entityID/devotees/:userID/status
func (h *Handler) UpdateDevoteeMembershipStatus(c *gin.Context) {
	entityIDUint, err := strconv.ParseUint(c.Param("entityID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
		return
	}

	// Get access context
	accessContextVal, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing access context"})
		return
	}
	accessContext, ok := accessContextVal.(middleware.AccessContext)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access context"})
		return
	}

	// Check write permissions
	if !accessContext.CanWrite() {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have write permissions"})
		return
	}

	// Check entity access
	entityID := uint(entityIDUint)
	hasAccess := (accessContext.DirectEntityID != nil && *accessContext.DirectEntityID == entityID) ||
		(accessContext.AssignedEntityID != nil && *accessContext.AssignedEntityID == entityID)

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to manage devotees for this entity"})
		return
	}

	userID, err := strconv.ParseUint(c.Param("userID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required,oneof=active inactive"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.Service.MembershipService.UpdateMembershipStatus(uint(userID), uint(entityIDUint), req.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not update status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Membership status updated successfully"})
}

// Temple Admin → Dashboard Summary
// GET /entities/dashboard-summary
func (h *Handler) GetDashboardSummary(c *gin.Context) {
	// Get access context
	accessContextVal, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing access context"})
		return
	}
	accessContext, ok := accessContextVal.(middleware.AccessContext)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access context"})
		return
	}

	// Get the accessible entity ID
	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No accessible entity"})
		return
	}

	// Call service
	summary, err := h.Service.GetDashboardSummary(*entityID)
	if err != nil {
		log.Printf("Dashboard Summary Error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch dashboard summary"})
		return
	}

	c.JSON(http.StatusOK, summary)
}