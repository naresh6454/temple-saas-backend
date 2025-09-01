package entity

import (
	"time"
	"math"

	"github.com/sharath018/temple-management-backend/internal/auth"
	"gorm.io/gorm"
)

type Repository struct {
	DB *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{DB: db}
}

// ========== ENTITY CORE ==========

// Create a new temple entity
func (r *Repository) CreateEntity(e *Entity) error {
	return r.DB.Create(e).Error
}

// Create an approval request for the temple (linked to auth module)
func (r *Repository) CreateApprovalRequest(req *auth.ApprovalRequest) error {
	return r.DB.Create(req).Error
}

// Fetch all temple entities (ordered by most recent)
func (r *Repository) GetAllEntities() ([]Entity, error) {
	var entities []Entity
	err := r.DB.Order("created_at DESC").Find(&entities).Error
	return entities, err
}

// Fetch entities created by a specific user (ordered by most recent)
func (r *Repository) GetEntitiesByCreator(creatorID uint) ([]Entity, error) {
	var entities []Entity
	err := r.DB.Where("created_by = ?", creatorID).Order("created_at DESC").Find(&entities).Error
	return entities, err
}

// Fetch a single temple entity by ID
func (r *Repository) GetEntityByID(id int) (Entity, error) {
	var entity Entity
	err := r.DB.First(&entity, id).Error
	return entity, err
}

// GetUserRole retrieves the role for a specific user
func (r *Repository) GetUserRole(userID uint) (string, error) {
    var roleName string
    err := r.DB.Table("users").
        Select("user_roles.role_name").
        Joins("JOIN user_roles ON users.role_id = user_roles.id").
        Where("users.id = ?", userID).
        Scan(&roleName).Error
    return roleName, err
}

// GetTenantIDForUser retrieves the tenant ID for a standard user
func (r *Repository) GetTenantIDForUser(userID uint) (uint, error) {
    var tenantID uint
    err := r.DB.Table("tenant_user_assignments").
        Select("tenant_id").
        Where("user_id = ? AND status = ?", userID, "active").
        Limit(1).
        Scan(&tenantID).Error
    return tenantID, err
}

// Update an existing temple entity
func (r *Repository) UpdateEntity(e Entity) error {
	e.UpdatedAt = time.Now()
	
	// Create a map of all fields to update, even if they're zero values
	updates := map[string]interface{}{
		"name":                 e.Name,
		"main_deity":           e.MainDeity,
		"temple_type":          e.TempleType,
		"established_year":     e.EstablishedYear,
		"email":                e.Email,
		"phone":                e.Phone,
		"description":          e.Description,
		"street_address":       e.StreetAddress,
		"landmark":             e.Landmark,
		"city":                 e.City,
		"district":             e.District,
		"state":                e.State,
		"pincode":              e.Pincode,
		"map_link":             e.MapLink,
		"registration_cert_url": e.RegistrationCertURL,
		"trust_deed_url":       e.TrustDeedURL,
		"property_docs_url":    e.PropertyDocsURL,
		"additional_docs_urls": e.AdditionalDocsURLs,
		"updated_at":           e.UpdatedAt,
	}
	
	return r.DB.Model(&Entity{}).Where("id = ?", e.ID).Updates(updates).Error
}

// Delete a temple entity by ID
func (r *Repository) DeleteEntity(id int) error {
	return r.DB.Delete(&Entity{}, id).Error
}

// ========== DEVOTEE MANAGEMENT ==========

// DevoteeDTO represents the devotee information returned by queries
type DevoteeDTO struct {
	UserID   uint   `json:"user_id"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Status   string `json:"status"`
}

// Get all devotees for a specific entity
func (r *Repository) GetDevoteesByEntityID(entityID uint) ([]DevoteeDTO, error) {
	var devotees []DevoteeDTO

	err := r.DB.
		Table("user_entity_memberships AS uem").
		Select("u.id AS user_id, u.full_name, u.email, u.phone, uem.status").
		Joins("JOIN users u ON u.id = uem.user_id").
		Joins("JOIN user_roles ur ON u.role_id = ur.id").
		Where("uem.entity_id = ? AND ur.role_name = ?", entityID, "devotee").
		Scan(&devotees).Error

	return devotees, err
}

// DevoteeStats represents statistics about devotees for an entity
type DevoteeStats struct {
	TotalDevotees  int64 `json:"total_devotees"`
	ActiveDevotees int64 `json:"active_devotees"`
	NewThisMonth   int64 `json:"new_this_month"`
	// ProfileCompletionRate float64 `json:"profile_completion_rate"` // Optional
}

// Get devotee statistics for a specific entity
func (r *Repository) GetDevoteeStats(entityID uint) (DevoteeStats, error) {
	var stats DevoteeStats

	// Total Devotees
	err := r.DB.Table("user_entity_memberships").
		Joins("JOIN users ON users.id = user_entity_memberships.user_id").
		Joins("JOIN user_roles ON user_roles.id = users.role_id").
		Where("user_entity_memberships.entity_id = ? AND user_roles.role_name = ?", entityID, "devotee").
		Count(&stats.TotalDevotees).Error
	if err != nil {
		return stats, err
	}

	// Active Devotees
	err = r.DB.Table("user_entity_memberships").
		Joins("JOIN users ON users.id = user_entity_memberships.user_id").
		Joins("JOIN user_roles ON user_roles.id = users.role_id").
		Where("user_entity_memberships.entity_id = ? AND user_roles.role_name = ? AND user_entity_memberships.status = ?", entityID, "devotee", "active").
		Count(&stats.ActiveDevotees).Error
	if err != nil {
		return stats, err
	}

	// New Devotees This Month
	err = r.DB.Table("user_entity_memberships").
		Joins("JOIN users ON users.id = user_entity_memberships.user_id").
		Joins("JOIN user_roles ON user_roles.id = users.role_id").
		Where("user_entity_memberships.entity_id = ? AND user_roles.role_name = ? AND user_entity_memberships.created_at >= DATE_TRUNC('month', NOW())", entityID, "devotee").
		Count(&stats.NewThisMonth).Error
	if err != nil {
		return stats, err
	}

	return stats, nil
}


// Count total devotees for an entity
func (r *Repository) CountDevotees(entityID uint) (int64, error) {
	var count int64
	err := r.DB.
		Table("user_entity_memberships AS uem").
		Joins("JOIN user_roles ur ON ur.id = (SELECT role_id FROM users WHERE users.id = uem.user_id)").
		Where("uem.entity_id = ? AND ur.role_name = ?", entityID, "devotee").
		Count(&count).Error
	return count, err
}

// Count devotees added this month
func (r *Repository) CountDevoteesThisMonth(entityID uint) (int64, error) {
	var count int64
	err := r.DB.
		Table("user_entity_memberships AS uem").
		Joins("JOIN user_roles ur ON ur.id = (SELECT role_id FROM users WHERE users.id = uem.user_id)").
		Where("uem.entity_id = ? AND ur.role_name = ? AND uem.created_at >= DATE_TRUNC('month', NOW())", entityID, "devotee").
		Count(&count).Error
	return count, err
}

// Count today's seva bookings
func (r *Repository) CountSevaBookingsToday(entityID uint) (int64, error) {
	var count int64
	err := r.DB.
		Table("seva_bookings").
		Where("entity_id = ? AND DATE(booking_time) = CURRENT_DATE", entityID).
		Count(&count).Error
	return count, err
}

// Count seva bookings in current month
func (r *Repository) CountSevaBookingsThisMonth(entityID uint) (int64, error) {
	var count int64
	err := r.DB.
		Table("seva_bookings").
		Where("entity_id = ? AND booking_time >= DATE_TRUNC('month', NOW())", entityID).
		Count(&count).Error
	return count, err
}

// Get total donation ₹ this month and % change vs last month
func (r *Repository) GetMonthDonationsWithChange(entityID uint) (float64, float64, error) {
	var currentMonth, previousMonth float64

	// This month
	err := r.DB.
		Table("donations").
		Select("COALESCE(SUM(amount), 0)").
		Where("entity_id = ? AND created_at >= DATE_TRUNC('month', NOW())", entityID).
		Scan(&currentMonth).Error
	if err != nil {
		return 0, 0, err
	}

	// Last month
	err = r.DB.
		Table("donations").
		Select("COALESCE(SUM(amount), 0)").
		Where("entity_id = ? AND created_at >= DATE_TRUNC('month', NOW()) - INTERVAL '1 month' AND created_at < DATE_TRUNC('month', NOW())", entityID).
		Scan(&previousMonth).Error
	if err != nil {
		return 0, 0, err
	}

	// % change (guard against divide by zero)
	var percentChange float64
	if previousMonth > 0 {
		percentChange = ((currentMonth - previousMonth) / previousMonth) * 100
	} else if currentMonth > 0 {
		percentChange = 100
	} else {
		percentChange = 0
	}

	return currentMonth, math.Round(percentChange*100) / 100, nil
}

// Count total upcoming events (future start_date)
func (r *Repository) CountUpcomingEvents(entityID uint) (int64, error) {
	var count int64
	err := r.DB.
		Table("events").
		Where("entity_id = ? AND event_date >= CURRENT_DATE", entityID).
		Count(&count).Error
	return count, err
}

// Count events in this week (Sun–Sat)
func (r *Repository) CountUpcomingEventsThisWeek(entityID uint) (int64, error) {
	var count int64
	err := r.DB.
		Table("events").
		Where(`
	entity_id = ?
	AND event_date >= DATE_TRUNC('week', CURRENT_DATE)
	AND event_date < DATE_TRUNC('week', CURRENT_DATE) + INTERVAL '7 days'
`, entityID).
		Count(&count).Error
	return count, err
}