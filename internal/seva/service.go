package seva

import (
	"context"
	"errors"
	"time"

	"github.com/sharath018/temple-management-backend/internal/auditlog"
	"github.com/sharath018/temple-management-backend/middleware"
)

type Service interface {
	// Seva Core
	CreateSeva(ctx context.Context, seva *Seva, accessContext middleware.AccessContext, ip string) error
	UpdateSeva(ctx context.Context, seva *Seva, accessContext middleware.AccessContext, ip string) error
	DeleteSeva(ctx context.Context, sevaID uint, accessContext middleware.AccessContext, ip string) error
	GetSevasByEntity(ctx context.Context, entityID uint) ([]Seva, error)
	GetSevaByID(ctx context.Context, id uint) (*Seva, error)

	// 🆕 Enhanced seva listing with filters for temple admin
	GetSevasWithFilters(ctx context.Context, entityID uint, sevaType, search, status, isActive string, limit, offset int) ([]Seva, int64, error)

	// Booking Core
	BookSeva(ctx context.Context, booking *SevaBooking, userRole string, userID uint, entityID uint, ip string) error
	GetBookingsForUser(ctx context.Context, userID uint) ([]SevaBooking, error)
	GetBookingsForEntity(ctx context.Context, entityID uint) ([]SevaBooking, error)
	UpdateBookingStatus(ctx context.Context, bookingID uint, newStatus string, userID uint, ip string) error

	// Composite Booking Details
	GetDetailedBookingsForEntity(ctx context.Context, entityID uint) ([]DetailedBooking, error)

	// Filters, Search, Pagination
	SearchBookings(ctx context.Context, filter BookingFilter) ([]DetailedBooking, int64, error) // ✅ New

	// Counts
	GetBookingCountsByStatus(ctx context.Context, entityID uint) (BookingStatusCounts, error) // ✅ New

	GetDetailedBookingsWithFilters(ctx context.Context, entityID uint, status, sevaType, startDate, endDate, search string, limit, offset int) ([]DetailedBooking, error)
	GetBookingByID(ctx context.Context, bookingID uint) (*SevaBooking, error)
	GetBookingStatusCounts(ctx context.Context, entityID uint) (BookingStatusCounts, error)

	GetPaginatedSevas(ctx context.Context, entityID uint, sevaType string, search string, limit int, offset int) ([]Seva, error)
}

type service struct {
	repo       Repository
	auditSvc   auditlog.Service
}

func NewService(repo Repository, auditSvc auditlog.Service) Service {
	return &service{
		repo:     repo,
		auditSvc: auditSvc,
	}
}

// Updated to use access context
func (s *service) CreateSeva(ctx context.Context, seva *Seva, accessContext middleware.AccessContext, ip string) error {
	// Check write permissions
	if !accessContext.CanWrite() {
		s.auditSvc.LogAction(ctx, &accessContext.UserID, accessContext.GetAccessibleEntityID(), "SEVA_CREATE_FAILED", map[string]interface{}{
			"reason": "write access denied",
			"seva_name": seva.Name,
		}, ip, "failure")
		return errors.New("write access denied")
	}

	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		s.auditSvc.LogAction(ctx, &accessContext.UserID, nil, "SEVA_CREATE_FAILED", map[string]interface{}{
			"reason": "no accessible entity",
			"seva_name": seva.Name,
		}, ip, "failure")
		return errors.New("no accessible entity")
	}

	seva.EntityID = *entityID

	// Create seva
	err := s.repo.CreateSeva(ctx, seva)
	if err != nil {
		// Audit failed creation
		s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_CREATE_FAILED", map[string]interface{}{
			"seva_name": seva.Name,
			"seva_type": seva.SevaType,
			"error": err.Error(),
		}, ip, "failure")
		return err
	}

	// Audit successful creation
	s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_CREATED", map[string]interface{}{
		"seva_id": seva.ID,
		"seva_name": seva.Name,
		"seva_type": seva.SevaType,
		"price": seva.Price,
		"role": accessContext.RoleName,
	}, ip, "success")

	return nil
}

// 🆕 Updated to use access context
func (s *service) UpdateSeva(ctx context.Context, seva *Seva, accessContext middleware.AccessContext, ip string) error {
	// Check write permissions
	if !accessContext.CanWrite() {
		s.auditSvc.LogAction(ctx, &accessContext.UserID, accessContext.GetAccessibleEntityID(), "SEVA_UPDATE_FAILED", map[string]interface{}{
			"reason": "write access denied",
			"seva_id": seva.ID,
		}, ip, "failure")
		return errors.New("write access denied")
	}

	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		s.auditSvc.LogAction(ctx, &accessContext.UserID, nil, "SEVA_UPDATE_FAILED", map[string]interface{}{
			"reason": "no accessible entity",
			"seva_id": seva.ID,
		}, ip, "failure")
		return errors.New("no accessible entity")
	}

	seva.EntityID = *entityID

	// Update seva
	err := s.repo.UpdateSeva(ctx, seva)
	if err != nil {
		// Audit failed update
		s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_UPDATE_FAILED", map[string]interface{}{
			"seva_id": seva.ID,
			"seva_name": seva.Name,
			"error": err.Error(),
		}, ip, "failure")
		return err
	}

	// Audit successful update
	s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_UPDATED", map[string]interface{}{
		"seva_id": seva.ID,
		"seva_name": seva.Name,
		"seva_type": seva.SevaType,
		"price": seva.Price,
		"role": accessContext.RoleName,
	}, ip, "success")

	return nil
}

// 🆕 Updated to use access context with soft delete
func (s *service) DeleteSeva(ctx context.Context, sevaID uint, accessContext middleware.AccessContext, ip string) error {
	// Check write permissions
	if !accessContext.CanWrite() {
		s.auditSvc.LogAction(ctx, &accessContext.UserID, accessContext.GetAccessibleEntityID(), "SEVA_DELETE_FAILED", map[string]interface{}{
			"reason": "write access denied",
			"seva_id": sevaID,
		}, ip, "failure")
		return errors.New("write access denied")
	}

	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		s.auditSvc.LogAction(ctx, &accessContext.UserID, nil, "SEVA_DELETE_FAILED", map[string]interface{}{
			"reason": "no accessible entity",
			"seva_id": sevaID,
		}, ip, "failure")
		return errors.New("no accessible entity")
	}

	// Get seva details for audit logging before deletion
	seva, err := s.repo.GetSevaByID(ctx, sevaID)
	if err != nil {
		s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_DELETE_FAILED", map[string]interface{}{
			"seva_id": sevaID,
			"reason": "seva not found",
			"error": err.Error(),
		}, ip, "failure")
		return err
	}

	// Verify seva belongs to accessible entity
	if seva.EntityID != *entityID {
		s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_DELETE_FAILED", map[string]interface{}{
			"seva_id": sevaID,
			"reason": "access denied to this seva",
		}, ip, "failure")
		return errors.New("access denied to this seva")
	}

	// Use soft delete instead of hard delete to preserve booking history
	err = s.repo.SoftDeleteSeva(ctx, sevaID)
	if err != nil {
		// Audit failed deletion
		s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_DELETE_FAILED", map[string]interface{}{
			"seva_id": sevaID,
			"seva_name": seva.Name,
			"error": err.Error(),
		}, ip, "failure")
		return err
	}

	// Audit successful deletion
	s.auditSvc.LogAction(ctx, &accessContext.UserID, entityID, "SEVA_DELETED", map[string]interface{}{
		"seva_id": sevaID,
		"seva_name": seva.Name,
		"seva_type": seva.SevaType,
		"price": seva.Price,
		"role": accessContext.RoleName,
	}, ip, "success")

	return nil
}

func (s *service) GetSevasByEntity(ctx context.Context, entityID uint) ([]Seva, error) {
	return s.repo.ListSevasByEntityID(ctx, entityID)
}

func (s *service) GetSevaByID(ctx context.Context, id uint) (*Seva, error) {
	return s.repo.GetSevaByID(ctx, id)
}

// 🆕 Enhanced seva listing with filters for temple admin
func (s *service) GetSevasWithFilters(ctx context.Context, entityID uint, sevaType, search, status, isActive string, limit, offset int) ([]Seva, int64, error) {
	return s.repo.GetSevasWithFilters(ctx, entityID, sevaType, search, status, isActive, limit, offset)
}

// Devotee only - keep unchanged
func (s *service) BookSeva(ctx context.Context, booking *SevaBooking, userRole string, userID uint, entityID uint, ip string) error {
	if userRole != "devotee" {
		// Audit failed attempt
		s.auditSvc.LogAction(ctx, &userID, &entityID, "SEVA_BOOKING_FAILED", map[string]interface{}{
			"reason": "unauthorized access",
			"seva_id": booking.SevaID,
		}, ip, "failure")
		return errors.New("unauthorized: only devotee can book sevas")
	}

	// Validate Seva exists and is active
	seva, err := s.repo.GetSevaByID(ctx, booking.SevaID)
	if err != nil {
		// Audit failed booking
		s.auditSvc.LogAction(ctx, &userID, &entityID, "SEVA_BOOKING_FAILED", map[string]interface{}{
			"seva_id": booking.SevaID,
			"reason": "seva not found",
			"error": err.Error(),
		}, ip, "failure")
		return err
	}

	// Check if seva is active
	if !seva.IsActive {
		s.auditSvc.LogAction(ctx, &userID, &entityID, "SEVA_BOOKING_FAILED", map[string]interface{}{
			"seva_id": booking.SevaID,
			"seva_name": seva.Name,
			"reason": "seva is not active",
		}, ip, "failure")
		return errors.New("seva is not active for booking")
	}

	booking.UserID = userID
	booking.EntityID = entityID
	booking.BookingTime = time.Now() // ✅ Fix: Set current time
	booking.Status = "pending"

	// Create booking
	err = s.repo.BookSeva(ctx, booking)
	if err != nil {
		// Audit failed booking
		s.auditSvc.LogAction(ctx, &userID, &entityID, "SEVA_BOOKING_FAILED", map[string]interface{}{
			"seva_id": booking.SevaID,
			"seva_name": seva.Name,
			"error": err.Error(),
		}, ip, "failure")
		return err
	}

	// Audit successful booking
	s.auditSvc.LogAction(ctx, &userID, &entityID, "SEVA_BOOKED", map[string]interface{}{
		"booking_id": booking.ID,
		"seva_id": booking.SevaID,
		"seva_name": seva.Name,
		"seva_type": seva.SevaType,
		"status": booking.Status,
	}, ip, "success")

	return nil
}

func (s *service) GetBookingsForUser(ctx context.Context, userID uint) ([]SevaBooking, error) {
	return s.repo.ListBookingsByUserID(ctx, userID)
}

func (s *service) GetBookingsForEntity(ctx context.Context, entityID uint) ([]SevaBooking, error) {
	return s.repo.ListBookingsByEntityID(ctx, entityID)
}

// Temple Admin only - keep existing logic since it's used through access context
func (s *service) UpdateBookingStatus(ctx context.Context, bookingID uint, newStatus string, userID uint, ip string) error {
	// Get booking details for audit
	booking, err := s.repo.GetBookingByID(ctx, bookingID)
	if err != nil {
		// Audit failed attempt
		s.auditSvc.LogAction(ctx, &userID, nil, "SEVA_BOOKING_STATUS_UPDATE_FAILED", map[string]interface{}{
			"booking_id": bookingID,
			"new_status": newStatus,
			"reason": "booking not found",
			"error": err.Error(),
		}, ip, "failure")
		return err
	}

	// Get seva details for better audit logging
	seva, _ := s.repo.GetSevaByID(ctx, booking.SevaID)

	// Update status
	err = s.repo.UpdateBookingStatus(ctx, bookingID, newStatus)
	if err != nil {
		// Audit failed update
		s.auditSvc.LogAction(ctx, &userID, &booking.EntityID, "SEVA_BOOKING_STATUS_UPDATE_FAILED", map[string]interface{}{
			"booking_id": bookingID,
			"seva_id": booking.SevaID,
			"new_status": newStatus,
			"error": err.Error(),
		}, ip, "failure")
		return err
	}

	// Audit successful status update with specific action
	action := "SEVA_BOOKING_STATUS_UPDATED"
	if newStatus == "approved" {
		action = "SEVA_BOOKING_APPROVED"
	} else if newStatus == "rejected" {
		action = "SEVA_BOOKING_REJECTED"
	}

	auditDetails := map[string]interface{}{
		"booking_id": bookingID,
		"seva_id": booking.SevaID,
		"devotee_id": booking.UserID,
		"old_status": booking.Status,
		"new_status": newStatus,
	}

	if seva != nil {
		auditDetails["seva_name"] = seva.Name
		auditDetails["seva_type"] = seva.SevaType
	}

	s.auditSvc.LogAction(ctx, &userID, &booking.EntityID, action, auditDetails, ip, "success")

	return nil
}

// 🔄 Templeadmin only: Full booking table with names, types, etc.
func (s *service) GetDetailedBookingsForEntity(ctx context.Context, entityID uint) ([]DetailedBooking, error) {
	return s.repo.ListBookingsWithDetails(ctx, entityID)
}

// ✅ GetBookingByID: Public or Templeadmin
func (s *service) GetBookingByID(ctx context.Context, bookingID uint) (*SevaBooking, error) {
	return s.repo.GetBookingByID(ctx, bookingID)
}

// ✅ SearchBookings: Templeadmin with filters (pagination, search, etc.)
func (s *service) SearchBookings(ctx context.Context, filter BookingFilter) ([]DetailedBooking, int64, error) {
	return s.repo.SearchBookingsWithFilters(ctx, filter)
}

// ✅ Count booking statuses: Templeadmin dashboard metrics
func (s *service) GetBookingCountsByStatus(ctx context.Context, entityID uint) (BookingStatusCounts, error) {
	return s.repo.CountBookingsByStatus(ctx, entityID)
}

func (s *service) GetDetailedBookingsWithFilters(ctx context.Context, entityID uint, status, sevaType, startDate, endDate, search string, limit, offset int) ([]DetailedBooking, error) {
	filter := BookingFilter{
		EntityID:  entityID,
		Status:    status,
		SevaType:  sevaType,
		StartDate: startDate,
		EndDate:   endDate,
		Search:    search,
		Limit:     limit,
		Offset:    offset,
	}
	bookings, _, err := s.repo.SearchBookingsWithFilters(ctx, filter)
	return bookings, err
}

func (s *service) GetBookingStatusCounts(ctx context.Context, entityID uint) (BookingStatusCounts, error) {
	return s.repo.CountBookingsByStatus(ctx, entityID)
}

func (s *service) GetPaginatedSevas(ctx context.Context, entityID uint, sevaType string, search string, limit int, offset int) ([]Seva, error) {
	return s.repo.ListPaginatedSevas(ctx, entityID, sevaType, search, limit, offset)
}