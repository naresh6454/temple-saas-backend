package seva

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sharath018/temple-management-backend/internal/auth"
	"github.com/sharath018/temple-management-backend/internal/auditlog"
	"github.com/sharath018/temple-management-backend/middleware"
)

type Handler struct {
	service  Service
	auditSvc auditlog.Service
}

func NewHandler(service Service, auditSvc auditlog.Service) *Handler {
	return &Handler{
		service:  service,
		auditSvc: auditSvc,
	}
}

// ========================= REQUEST STRUCTS =============================

type CreateSevaRequest struct {
	Name              string    `json:"name" binding:"required"`
	SevaType          string    `json:"seva_type" binding:"required"`
	Description       string    `json:"description"`
	Price             float64   `json:"price"`
	Date              string    `json:"date"`
	StartTime         string    `json:"start_time"`
	EndTime           string    `json:"end_time"`
	Duration          int       `json:"duration"`
	MaxBookingsPerDay int       `json:"max_bookings_per_day"`
}

type UpdateSevaRequest struct {
	Name              string    `json:"name"`
	SevaType          string    `json:"seva_type"`
	Description       string    `json:"description"`
	Price             float64   `json:"price"`
	Date              string    `json:"date"`
	StartTime         string    `json:"start_time"`
	EndTime           string    `json:"end_time"`
	Duration          int       `json:"duration"`
	MaxBookingsPerDay int       `json:"max_bookings_per_day"`
	IsActive          *bool     `json:"is_active"`
}

type BookSevaRequest struct {
	SevaID uint `json:"seva_id" binding:"required"`
}

// ========================= SEVA HANDLERS =============================

func (h *Handler) CreateSeva(c *gin.Context) {
	var input CreateSevaRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
		return
	}

	// Use access context instead of direct user access
	accessContext := c.MustGet("access_context").(middleware.AccessContext)
	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "No accessible entity"})
		return
	}

	// Extract IP address
	ip := middleware.GetIPFromContext(c)

	seva := Seva{
		EntityID:          *entityID,
		Name:              input.Name,
		SevaType:          input.SevaType,
		Description:       input.Description,
		Price:             input.Price,
		Date:              input.Date,
		StartTime:         input.StartTime,
		EndTime:           input.EndTime,
		Duration:          input.Duration,
		MaxBookingsPerDay: input.MaxBookingsPerDay,
		Status:            "upcoming",        // ✅ defaulted
		IsActive:          true,              // ✅ defaulted
	}

	if err := h.service.CreateSeva(c, &seva, accessContext, ip); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create seva: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Seva created successfully", "seva": seva})
}

// 🆕 List all sevas for temple admin with filters and pagination
func (h *Handler) ListEntitySevas(c *gin.Context) {
	// Use access context for entity access
	accessContext := c.MustGet("access_context").(middleware.AccessContext)
	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "No accessible entity"})
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	sevaType := c.Query("seva_type")
	search := c.Query("search")
	status := c.Query("status")
	isActive := c.Query("is_active")

	sevas, total, err := h.service.GetSevasWithFilters(
		c,
		*entityID,
		sevaType,
		search,
		status,
		isActive,
		limit,
		offset,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch sevas: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sevas": sevas,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// 🆕 Get seva by ID for temple admin
func (h *Handler) GetSevaByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid seva ID"})
		return
	}

	// Use access context for entity verification
	accessContext := c.MustGet("access_context").(middleware.AccessContext)
	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "No accessible entity"})
		return
	}

	seva, err := h.service.GetSevaByID(c, uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Seva not found"})
		return
	}

	// Verify seva belongs to accessible entity
	if seva.EntityID != *entityID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this seva"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"seva": seva})
}

// 🆕 Update seva
func (h *Handler) UpdateSeva(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid seva ID"})
		return
	}

	var input UpdateSevaRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
		return
	}

	// Use access context for permissions
	accessContext := c.MustGet("access_context").(middleware.AccessContext)
	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "No accessible entity"})
		return
	}

	// Extract IP address
	ip := middleware.GetIPFromContext(c)

	// Get existing seva to verify ownership and preserve data
	existingSeva, err := h.service.GetSevaByID(c, uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Seva not found"})
		return
	}

	// Verify seva belongs to accessible entity
	if existingSeva.EntityID != *entityID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this seva"})
		return
	}

	// Update fields only if provided
	updatedSeva := *existingSeva
	if input.Name != "" {
		updatedSeva.Name = input.Name
	}
	if input.SevaType != "" {
		updatedSeva.SevaType = input.SevaType
	}
	if input.Description != "" {
		updatedSeva.Description = input.Description
	}
	if input.Price > 0 {
		updatedSeva.Price = input.Price
	}
	if input.Date != "" {
		updatedSeva.Date = input.Date
	}
	if input.StartTime != "" {
		updatedSeva.StartTime = input.StartTime
	}
	if input.EndTime != "" {
		updatedSeva.EndTime = input.EndTime
	}
	if input.Duration > 0 {
		updatedSeva.Duration = input.Duration
	}
	if input.MaxBookingsPerDay > 0 {
		updatedSeva.MaxBookingsPerDay = input.MaxBookingsPerDay
	}
	if input.IsActive != nil {
		updatedSeva.IsActive = *input.IsActive
	}

	if err := h.service.UpdateSeva(c, &updatedSeva, accessContext, ip); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update seva: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Seva updated successfully", "seva": updatedSeva})
}

// 🆕 Delete seva (soft delete by setting is_active = false)
func (h *Handler) DeleteSeva(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid seva ID"})
		return
	}

	// Use access context for permissions
	accessContext := c.MustGet("access_context").(middleware.AccessContext)
	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "No accessible entity"})
		return
	}

	// Extract IP address
	ip := middleware.GetIPFromContext(c)

	// Get existing seva to verify ownership
	existingSeva, err := h.service.GetSevaByID(c, uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Seva not found"})
		return
	}

	// Verify seva belongs to accessible entity
	if existingSeva.EntityID != *entityID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this seva"})
		return
	}

	if err := h.service.DeleteSeva(c, uint(id), accessContext, ip); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete seva: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Seva deleted successfully"})
}

func (h *Handler) GetSevas(c *gin.Context) {
	// Keep devotee logic unchanged
	user := c.MustGet("user").(auth.User)
	if user.Role.RoleName != "devotee" || user.EntityID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	sevaType := c.Query("seva_type")
	search := c.Query("search")

	sevas, err := h.service.GetPaginatedSevas(
		c,
		*user.EntityID,
		sevaType,
		search,
		limit,
		offset,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch sevas: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"sevas": sevas})
}

// ========================= BOOKING HANDLERS =============================

func (h *Handler) BookSeva(c *gin.Context) {
	var input BookSevaRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
		return
	}

	// Keep devotee logic unchanged
	user := c.MustGet("user").(auth.User)
	if user.Role.RoleName != "devotee" || user.EntityID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized or invalid entity"})
		return
	}

	// Extract IP address
	ip := middleware.GetIPFromContext(c)

	booking := SevaBooking{
		SevaID:      input.SevaID,
		UserID:      user.ID,
		EntityID:    *user.EntityID,
		BookingTime: time.Now(),       // ⏱️ Auto-generated now
		Status:      "pending",        // default state
	}

	if err := h.service.BookSeva(c, &booking, "devotee", user.ID, *user.EntityID, ip); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Booking failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Seva booked successfully",
		"booking": booking,
	})
}

func (h *Handler) GetMyBookings(c *gin.Context) {
	// Keep devotee logic unchanged
	user := c.MustGet("user").(auth.User)
	bookings, err := h.service.GetBookingsForUser(c, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch bookings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"bookings": bookings})
}

func (h *Handler) GetEntityBookings(c *gin.Context) {
	// Use access context instead of direct user access
	accessContext := c.MustGet("access_context").(middleware.AccessContext)
	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "No accessible entity"})
		return
	}

	status := c.Query("status")
	sevaType := c.Query("seva_type")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	search := c.Query("search")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	bookings, err := h.service.GetDetailedBookingsWithFilters(
		c, *entityID, status, sevaType, startDate, endDate, search, limit, offset,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch detailed bookings: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"bookings": bookings})
}

func (h *Handler) GetBookingByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid booking ID"})
		return
	}

	booking, err := h.service.GetBookingByID(c, uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"booking": booking})
}

func (h *Handler) GetBookingCounts(c *gin.Context) {
	// Handle both devotee and temple admin access
	user := c.MustGet("user").(auth.User)
	var entityID *uint

	// Check if user is devotee (original logic)
	if user.Role.RoleName == "devotee" {
		if user.EntityID == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
			return
		}
		entityID = user.EntityID
	} else {
		// Use access context for temple admin users
		accessContext := c.MustGet("access_context").(middleware.AccessContext)
		entityID = accessContext.GetAccessibleEntityID()
		if entityID == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "No accessible entity"})
			return
		}
	}

	counts, err := h.service.GetBookingStatusCounts(c, *entityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch counts: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"counts": counts})
}

func (h *Handler) UpdateBookingStatus(c *gin.Context) {
	// Access control is already handled by RequireWriteAccess() middleware
	user := c.MustGet("user").(auth.User)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid booking ID"})
		return
	}

	var input struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.Status == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status field"})
		return
	}

	// Extract IP address
	ip := middleware.GetIPFromContext(c)

	if err := h.service.UpdateBookingStatus(c, uint(id), input.Status, user.ID, ip); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Status update failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Booking status updated successfully"})
}