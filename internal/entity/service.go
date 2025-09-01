package entity

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/sharath018/temple-management-backend/internal/auth"
	"github.com/sharath018/temple-management-backend/internal/auditlog"
)

type MembershipService interface {
	UpdateMembershipStatus(userID, entityID uint, status string) error
}

type Service struct {
	Repo              *Repository
	MembershipService MembershipService
	AuditService      auditlog.Service // 🆕 ADD AUDIT SERVICE
}

func NewService(r *Repository, ms MembershipService, as auditlog.Service) *Service {
	return &Service{
		Repo:              r,
		MembershipService: ms,
		AuditService:      as, // 🆕 INJECT AUDIT SERVICE
	}
}

var (
	ErrMissingFields = errors.New("temple name, deity, phone, and email are required")
)

// ========== ENTITY CORE ==========

// Temple Admin → Create Entity
// Temple Admin → Create Entity
// Temple Admin → Create Entity
// Temple Admin → Create Entity
func (s *Service) CreateEntity(e *Entity, userID uint, ip string) error {
    // Validate required fields
    if strings.TrimSpace(e.Name) == "" ||
        e.MainDeity == nil || strings.TrimSpace(*e.MainDeity) == "" ||
        strings.TrimSpace(e.Phone) == "" ||
        strings.TrimSpace(e.Email) == "" {
        
        // 🔍 LOG FAILED TEMPLE CREATION ATTEMPT
        auditDetails := map[string]interface{}{
            "temple_name": strings.TrimSpace(e.Name),
            "email":       strings.TrimSpace(e.Email),
            "error":       "Missing required fields",
        }
        s.AuditService.LogAction(context.Background(), &userID, nil, "TEMPLE_CREATE_FAILED", auditDetails, ip, "failure")
        
        return ErrMissingFields
    }

    now := time.Now()

    // Set metadata
    e.Status = "pending"
    // DO NOT override the CreatedBy field here - it's already set by the handler
    // The CreatedBy field should already be set correctly
    e.CreatedAt = now
    e.UpdatedAt = now

    // Sanitize inputs
    e.Name = strings.TrimSpace(e.Name)
    e.Email = strings.TrimSpace(e.Email)
    e.Phone = strings.TrimSpace(e.Phone)
    e.TempleType = strings.TrimSpace(e.TempleType)
    e.Description = strings.TrimSpace(e.Description)
    e.StreetAddress = strings.TrimSpace(e.StreetAddress)
    e.City = strings.TrimSpace(e.City)
    e.State = strings.TrimSpace(e.State)
    e.District = strings.TrimSpace(e.District)
    e.Pincode = strings.TrimSpace(e.Pincode)

    // Trim main deity if present
    if e.MainDeity != nil {
        trimmed := strings.TrimSpace(*e.MainDeity)
        e.MainDeity = &trimmed
    }

    // Trim document URLs
    e.RegistrationCertURL = strings.TrimSpace(e.RegistrationCertURL)
    e.TrustDeedURL = strings.TrimSpace(e.TrustDeedURL)
    e.PropertyDocsURL = strings.TrimSpace(e.PropertyDocsURL)
    e.AdditionalDocsURLs = strings.TrimSpace(e.AdditionalDocsURLs)

    // Save entity
    if err := s.Repo.CreateEntity(e); err != nil {
        // 🔍 LOG FAILED TEMPLE CREATION (DB ERROR)
        auditDetails := map[string]interface{}{
            "temple_name": e.Name,
            "email":       e.Email,
            "error":       err.Error(),
        }
        s.AuditService.LogAction(context.Background(), &userID, nil, "TEMPLE_CREATE_FAILED", auditDetails, ip, "failure")
        
        return err
    }

    // Create approval request
    req := &auth.ApprovalRequest{
        UserID:      userID,
        EntityID:    &e.ID,
        RequestType: "temple_approval",
        Status:      "pending",
        CreatedAt:   now,
        UpdatedAt:   now,
    }

    if err := s.Repo.CreateApprovalRequest(req); err != nil {
        // 🔍 LOG FAILED APPROVAL REQUEST CREATION
        auditDetails := map[string]interface{}{
            "temple_name": e.Name,
            "temple_id":   e.ID,
            "email":       e.Email,
            "error":       err.Error(),
        }
        s.AuditService.LogAction(context.Background(), &userID, &e.ID, "TEMPLE_APPROVAL_REQUEST_FAILED", auditDetails, ip, "failure")
        
        return err
    }

    // 🔍 LOG SUCCESSFUL TEMPLE CREATION
    auditDetails := map[string]interface{}{
        "temple_name":   e.Name,
        "temple_id":     e.ID,
        "temple_type":   e.TempleType,
        "email":         e.Email,
        "phone":         e.Phone,
        "city":          e.City,
        "state":         e.State,
        "main_deity":    e.MainDeity,
        "status":        e.Status,
    }
    s.AuditService.LogAction(context.Background(), &userID, &e.ID, "TEMPLE_CREATED", auditDetails, ip, "success")

    return nil
}

// Super Admin → Get all temples
func (s *Service) GetAllEntities() ([]Entity, error) {
	return s.Repo.GetAllEntities()
}

// Temple Admin → Get entities created by specific user
func (s *Service) GetEntitiesByCreator(creatorID uint) ([]Entity, error) {
	return s.Repo.GetEntitiesByCreator(creatorID)
}

// Anyone → View a temple by ID
func (s *Service) GetEntityByID(id int) (Entity, error) {
	return s.Repo.GetEntityByID(id)
}

// Temple Admin → Update own temple
func (s *Service) UpdateEntity(e Entity, userID uint, ip string) error {
	// Get existing entity for comparison
	existingEntity, err := s.Repo.GetEntityByID(int(e.ID))
	if err != nil {
		// 🔍 LOG FAILED TEMPLE UPDATE ATTEMPT (NOT FOUND)
		auditDetails := map[string]interface{}{
			"temple_id": e.ID,
			"error":     "Temple not found",
		}
		s.AuditService.LogAction(context.Background(), &userID, &e.ID, "TEMPLE_UPDATE_FAILED", auditDetails, ip, "failure")
		
		return err
	}

	e.UpdatedAt = time.Now()
	
	if err := s.Repo.UpdateEntity(e); err != nil {
		// 🔍 LOG FAILED TEMPLE UPDATE (DB ERROR)
		auditDetails := map[string]interface{}{
			"temple_id":   e.ID,
			"temple_name": e.Name,
			"error":       err.Error(),
		}
		s.AuditService.LogAction(context.Background(), &userID, &e.ID, "TEMPLE_UPDATE_FAILED", auditDetails, ip, "failure")
		
		return err
	}

	// 🔍 LOG SUCCESSFUL TEMPLE UPDATE
	auditDetails := map[string]interface{}{
		"temple_id":         e.ID,
		"temple_name":       e.Name,
		"previous_name":     existingEntity.Name,
		"temple_type":       e.TempleType,
		"email":             e.Email,
		"phone":             e.Phone,
		"city":              e.City,
		"state":             e.State,
		"main_deity":        e.MainDeity,
		"description":       e.Description,
		"updated_fields":    getUpdatedFields(existingEntity, e),
	}
	s.AuditService.LogAction(context.Background(), &userID, &e.ID, "TEMPLE_UPDATED", auditDetails, ip, "success")

	return nil
}

// Super Admin → Delete temple
func (s *Service) DeleteEntity(id int, userID uint, ip string) error {
	// Get existing entity for audit log
	existingEntity, err := s.Repo.GetEntityByID(id)
	if err != nil {
		// 🔍 LOG FAILED TEMPLE DELETION ATTEMPT (NOT FOUND)
		auditDetails := map[string]interface{}{
			"temple_id": id,
			"error":     "Temple not found",
		}
		entityID := uint(id)
		s.AuditService.LogAction(context.Background(), &userID, &entityID, "TEMPLE_DELETE_FAILED", auditDetails, ip, "failure")
		
		return err
	}

	if err := s.Repo.DeleteEntity(id); err != nil {
		// 🔍 LOG FAILED TEMPLE DELETION (DB ERROR)
		auditDetails := map[string]interface{}{
			"temple_id":   id,
			"temple_name": existingEntity.Name,
			"error":       err.Error(),
		}
		entityID := uint(id)
		s.AuditService.LogAction(context.Background(), &userID, &entityID, "TEMPLE_DELETE_FAILED", auditDetails, ip, "failure")
		
		return err
	}

	// 🔍 LOG SUCCESSFUL TEMPLE DELETION
	auditDetails := map[string]interface{}{
		"temple_id":   id,
		"temple_name": existingEntity.Name,
		"temple_type": existingEntity.TempleType,
		"email":       existingEntity.Email,
		"city":        existingEntity.City,
		"state":       existingEntity.State,
	}
	entityID := uint(id)
	s.AuditService.LogAction(context.Background(), &userID, &entityID, "TEMPLE_DELETED", auditDetails, ip, "success")

	return nil
}

// ========== DEVOTEE MANAGEMENT ==========

// Temple Admin → Get devotees for specific entity
func (s *Service) GetDevotees(entityID uint) ([]DevoteeDTO, error) {
	return s.Repo.GetDevoteesByEntityID(entityID)
}

// Temple Admin → Get devotee statistics for entity
func (s *Service) GetDevoteeStats(entityID uint) (DevoteeStats, error) {
	return s.Repo.GetDevoteeStats(entityID)
}

// DashboardSummary is the structured JSON response
type DashboardSummary struct {
	RegisteredDevotees struct {
		Total     int64 `json:"total"`
		ThisMonth int64 `json:"this_month"`
	} `json:"registered_devotees"`

	SevaBookings struct {
		Today     int64 `json:"today"`
		ThisMonth int64 `json:"this_month"`
	} `json:"seva_bookings"`

	MonthDonations struct {
		Amount        float64 `json:"amount"`
		PercentChange float64 `json:"percent_change"`
	} `json:"month_donations"`

	UpcomingEvents struct {
		Total     int64 `json:"total"`
		ThisWeek  int64 `json:"this_week"`
	} `json:"upcoming_events"`
}

// Temple Admin → Dashboard Summary
func (s *Service) GetDashboardSummary(entityID uint) (DashboardSummary, error) {
	var summary DashboardSummary

	// ============= DEVOTEES =============
	totalDevotees, err := s.Repo.CountDevotees(entityID)
	if err != nil {
		return summary, err
	}
	thisMonthDevotees, err := s.Repo.CountDevoteesThisMonth(entityID)
	if err != nil {
		return summary, err
	}
	summary.RegisteredDevotees.Total = totalDevotees
	summary.RegisteredDevotees.ThisMonth = thisMonthDevotees

	// ============= SEVA BOOKINGS =============
	todaySevas, err := s.Repo.CountSevaBookingsToday(entityID)
	if err != nil {
		return summary, err
	}
	monthSevas, err := s.Repo.CountSevaBookingsThisMonth(entityID)
	if err != nil {
		return summary, err
	}
	summary.SevaBookings.Today = todaySevas
	summary.SevaBookings.ThisMonth = monthSevas

	// ============= DONATIONS =============
	monthDonationAmount, percentChange, err := s.Repo.GetMonthDonationsWithChange(entityID)
	if err != nil {
		return summary, err
	}
	summary.MonthDonations.Amount = monthDonationAmount
	summary.MonthDonations.PercentChange = percentChange

	// ============= UPCOMING EVENTS =============
	totalUpcoming, err := s.Repo.CountUpcomingEvents(entityID)
	if err != nil {
		return summary, err
	}
	thisWeekUpcoming, err := s.Repo.CountUpcomingEventsThisWeek(entityID)
	if err != nil {
		return summary, err
	}
	summary.UpcomingEvents.Total = totalUpcoming
	summary.UpcomingEvents.ThisWeek = thisWeekUpcoming

	return summary, nil
}

// Helper function to track what fields were updated
func getUpdatedFields(old, new Entity) []string {
	var updatedFields []string

	if old.Name != new.Name {
		updatedFields = append(updatedFields, "name")
	}
	if (old.MainDeity == nil && new.MainDeity != nil) || 
	   (old.MainDeity != nil && new.MainDeity == nil) ||
	   (old.MainDeity != nil && new.MainDeity != nil && *old.MainDeity != *new.MainDeity) {
		updatedFields = append(updatedFields, "main_deity")
	}
	if old.TempleType != new.TempleType {
		updatedFields = append(updatedFields, "temple_type")
	}
	if old.Email != new.Email {
		updatedFields = append(updatedFields, "email")
	}
	if old.Phone != new.Phone {
		updatedFields = append(updatedFields, "phone")
	}
	if old.Description != new.Description {
		updatedFields = append(updatedFields, "description")
	}
	if old.StreetAddress != new.StreetAddress {
		updatedFields = append(updatedFields, "street_address")
	}
	if old.City != new.City {
		updatedFields = append(updatedFields, "city")
	}
	if old.State != new.State {
		updatedFields = append(updatedFields, "state")
	}
	if old.District != new.District {
		updatedFields = append(updatedFields, "district")
	}
	if old.Pincode != new.Pincode {
		updatedFields = append(updatedFields, "pincode")
	}

	return updatedFields
}