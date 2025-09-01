package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/sharath018/temple-management-backend/config"
	"github.com/sharath018/temple-management-backend/database"
	"github.com/sharath018/temple-management-backend/internal/auditlog" // NEW IMPORT
	"github.com/sharath018/temple-management-backend/internal/auth"
	"github.com/sharath018/temple-management-backend/internal/donation"
	"github.com/sharath018/temple-management-backend/internal/entity"
	"github.com/sharath018/temple-management-backend/internal/event"
	"github.com/sharath018/temple-management-backend/internal/eventrsvp"
	"github.com/sharath018/temple-management-backend/internal/notification"
	"github.com/sharath018/temple-management-backend/internal/reports"
	"github.com/sharath018/temple-management-backend/internal/seva"
	"github.com/sharath018/temple-management-backend/internal/superadmin"
	"github.com/sharath018/temple-management-backend/internal/userprofile"
	"github.com/sharath018/temple-management-backend/middleware"
	"net/http"
	"strings"

	_ "github.com/sharath018/temple-management-backend/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func Setup(r *gin.Engine, cfg *config.Config) {
	// Add static file serving for the public directory
	r.Static("/public", "./public")
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "OK"})
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// NEW: Add a direct route for reset password
	r.GET("/auth-pages/reset-password", func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.HTML(http.StatusBadRequest, "reset_password.html", gin.H{
				"error": "No reset token provided. Please check your email link.",
			})
			return
		}
		c.HTML(http.StatusOK, "reset_password.html", gin.H{
			"token": token,
		})
	})

	api := r.Group("/api/v1")
	api.Use(middleware.RateLimiterMiddleware()) // 🛡 Global rate limit: 5 req/sec per IP
	api.Use(middleware.AuditMiddleware())       // 🔍 NEW: Audit middleware to capture IP

	// ========== Initialize Audit Log Module ==========
	auditRepo := auditlog.NewRepository(database.DB)
	auditSvc := auditlog.NewService(auditRepo)
	auditHandler := auditlog.NewHandler(auditSvc)

	// ========== Auth ==========
	authRepo := auth.NewRepository(database.DB)
	authSvc := auth.NewService(authRepo, cfg) // ✅ INJECT AUDIT SERVICE
	authHandler := auth.NewHandler(authSvc)   // ✅ INJECT AUDIT SERVICE

	authGroup := api.Group("/auth")
	{
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/refresh", authHandler.Refresh)

		// ✅ NEW: Forgot/Reset/Logout
		authGroup.POST("/forgot-password", authHandler.ForgotPassword)
		authGroup.POST("/reset-password", authHandler.ResetPassword)

		// ✅ NEW: Public roles endpoint for registration (no auth required)
		authGroup.GET("/public-roles", authHandler.GetPublicRoles)

		// Logout requires Auth Middleware - Note: Check your middleware.AuthMiddleware signature
		// If your middleware has been updated to accept (*config.Config, *gorm.DB), keep it as is
		// If it still requires auth.Service, change to: middleware.AuthMiddleware(cfg, authSvc)
		authGroup.POST("/logout", middleware.AuthMiddleware(cfg, authSvc), authHandler.Logout)
	}

	protected := api.Group("/")
	protected.Use(middleware.AuthMiddleware(cfg, authSvc)) // Note: Verify middleware signature

	// Dashboards
	protected.GET("/tenant/dashboard", middleware.RBACMiddleware("templeadmin"), func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Temple Admin dashboard access granted!"})
	})
	protected.GET("/entity/:id/devotee/dashboard", middleware.RBACMiddleware("devotee"), func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Devotee dashboard access granted!"})
	})
	protected.GET("/entity/:id/volunteer/dashboard", middleware.RBACMiddleware("volunteer"), func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Volunteer dashboard access granted!"})
	})

	// ========== Audit Logs (SuperAdmin Only) ==========
	auditRoutes := protected.Group("/auditlogs")
	auditRoutes.Use(middleware.RBACMiddleware("superadmin"))
	{
		auditRoutes.GET("/", auditHandler.GetAuditLogs)
		auditRoutes.GET("/:id", auditHandler.GetAuditLogByID)
		auditRoutes.GET("/stats", auditHandler.GetAuditLogStats)
	}

	// ========== Super Admin ==========
	superadminRepo := superadmin.NewRepository(database.DB)
	superadminService := superadmin.NewService(superadminRepo, auditSvc) // ✅ INJECT AUDIT SERVICE
	superadminHandler := superadmin.NewHandler(superadminService)

	superadminRoutes := protected.Group("/superadmin")
	superadminRoutes.Use(middleware.RBACMiddleware("superadmin"))
	{
		// ================ TENANT APPROVAL MANAGEMENT ================
		// 🔁 Paginated list of all tenants with optional ?status=pending&limit=10&page=1
		superadminRoutes.GET("/tenants", superadminHandler.GetTenantsWithFilters)
		superadminRoutes.PATCH("/tenants/:id/approval", superadminHandler.UpdateTenantApprovalStatus)
		
		// ================ ENTITY APPROVAL MANAGEMENT ================
		// 🔁 Paginated list of entities with optional ?status=pending&limit=10&page=1
		superadminRoutes.GET("/entities", superadminHandler.GetEntitiesWithFilters)
		superadminRoutes.PATCH("/entities/:id/approval", superadminHandler.UpdateEntityApprovalStatus)

		// ================ DASHBOARD METRICS ================
		superadminRoutes.GET("/tenant-approval-count", superadminHandler.GetTenantApprovalCounts)
		superadminRoutes.GET("/temple-approval-count", superadminHandler.GetTempleApprovalCounts)

		// ================ USER MANAGEMENT ================
		// Create new user (admin-created users)
		superadminRoutes.POST("/users", superadminHandler.CreateUser)

		// Get all users with pagination and filters (excluding devotee and volunteer)
		// Query params: ?limit=10&page=1&search=john&role=templeadmin&status=active
		superadminRoutes.GET("/users", superadminHandler.GetUsers)

		// Get user by ID
		superadminRoutes.GET("/users/:id", superadminHandler.GetUserByID)

		// Update user
		superadminRoutes.PUT("/users/:id", superadminHandler.UpdateUser)

		// Delete user (soft delete)
		superadminRoutes.DELETE("/users/:id", superadminHandler.DeleteUser)

		// Activate/Deactivate user
		superadminRoutes.PATCH("/users/:id/status", superadminHandler.UpdateUserStatus)

		// Get all available user roles
		superadminRoutes.GET("/user-roles", superadminHandler.GetUserRoles)

		// Role management routes
		superadminRoutes.GET("/roles", superadminHandler.GetRoles)
		superadminRoutes.POST("/roles", superadminHandler.CreateRole)
		superadminRoutes.PUT("/roles/:id", superadminHandler.UpdateRole)
		superadminRoutes.PATCH("/roles/:id/status", superadminHandler.ToggleRoleStatus)

		// Reset user password (superadmin resets any user's password)
		superadminRoutes.POST("/users/:id/reset-password", superadminHandler.ResetUserPassword)
		superadminRoutes.GET("/users/search", superadminHandler.SearchUserByEmail)
		superadminRoutes.GET("/tenants/assignable", superadminHandler.GetTenantsForAssignment)
		// Assigns a list of users to a selected temple/tenant
		superadminRoutes.POST("/users/assign", superadminHandler.AssignUsersToTenant)
		// Bulk upload users via CSV
        superadminRoutes.POST("/users/bulk-upload", superadminHandler.BulkUploadUsers)
		
		// ================ SUPERADMIN REPORTS ================
		// Add dedicated routes for reports with multiple tenants
		reportsRepo := reports.NewRepository(database.DB)
		reportsExporter := reports.NewReportExporter()
		reportsService := reports.NewReportService(reportsRepo, reportsExporter, auditSvc)
		reportsHandler := reports.NewHandler(reportsService, reportsRepo, auditSvc)
		
		// Reports endpoints for superadmin with multiple tenants support
		superadminRoutes.GET("/reports/activities", reportsHandler.GetSuperAdminActivities)
		superadminRoutes.GET("/reports/temple-registered", reportsHandler.GetSuperAdminTempleRegisteredReport)
		superadminRoutes.GET("/reports/devotee-birthdays", reportsHandler.GetSuperAdminDevoteeBirthdaysReport)
		superadminRoutes.GET("/reports/devotee-list", reportsHandler.GetSuperAdminDevoteeListReport)
		superadminRoutes.GET("/reports/devotee-profile", reportsHandler.GetSuperAdminDevoteeProfileReport)
		superadminRoutes.GET("/reports/audit-logs", reportsHandler.GetSuperAdminAuditLogsReport)
	superadminRoutes.GET("/reports/approval-status", reportsHandler.GetApprovalStatusReport)
superadminRoutes.GET("/reports/user-details", reportsHandler.GetUserDetailsReport)


		// Support for tenant-specific routes (for backwards compatibility)
		superadminRoutes.GET("/tenants/:id/reports/activities", reportsHandler.GetSuperAdminTenantActivities)
		superadminRoutes.GET("/tenants/:id/reports/temple-registered", reportsHandler.GetSuperAdminTenantTempleRegisteredReport)
		superadminRoutes.GET("/tenants/:id/reports/devotee-birthdays", reportsHandler.GetSuperAdminTenantDevoteeBirthdaysReport)
		superadminRoutes.GET("/tenants/:id/reports/devotee-list", reportsHandler.GetSuperAdminTenantDevoteeListReport)
		superadminRoutes.GET("/tenants/:id/reports/devotee-profile", reportsHandler.GetSuperAdminTenantDevoteeProfileReport)
		superadminRoutes.GET("/tenants/:id/reports/audit-logs", reportsHandler.GetSuperAdminTenantAuditLogsReport)
	}

	protected.GET("/tenants/selection", 
		middleware.RBACMiddleware("superadmin", "standarduser", "monitoringuser"), 
		superadminHandler.GetTenantsForSelection)
	

// ========== Seva ==========
sevaRepo := seva.NewRepository(database.DB)
sevaService := seva.NewService(sevaRepo, auditSvc)    // ✅ INJECT AUDIT SERVICE
sevaHandler := seva.NewHandler(sevaService, auditSvc) // ✅ INJECT AUDIT SERVICE

sevaRoutes := protected.Group("/sevas")
sevaRoutes.GET("/booking-counts", sevaHandler.GetBookingCounts)


// 🔐 Temple Admin Routes (templeadmin, standarduser, monitoringuser)
templeSevaRoutes := sevaRoutes.Group("")
templeSevaRoutes.Use(middleware.RequireTempleAccess())
{
	// Write operations - only templeadmin and standarduser can access
	writeRoutes := templeSevaRoutes.Group("")
	writeRoutes.Use(middleware.RequireWriteAccess())
	{
		writeRoutes.POST("/", sevaHandler.CreateSeva)
		writeRoutes.PATCH("/bookings/:id/status", sevaHandler.UpdateBookingStatus)
	}
	
	// Read operations - all three roles can access
	templeSevaRoutes.GET("/entity-bookings", sevaHandler.GetEntityBookings)
	templeSevaRoutes.GET("/bookings/:id", sevaHandler.GetBookingByID)
}

// 🔐 Devotee Only Routes (keep existing middleware)
devoteeSevaRoutes := sevaRoutes.Group("")
devoteeSevaRoutes.Use(middleware.RBACMiddleware("devotee"))
{
	devoteeSevaRoutes.POST("/bookings", sevaHandler.BookSeva)
	devoteeSevaRoutes.GET("/my-bookings", sevaHandler.GetMyBookings)
	devoteeSevaRoutes.GET("/", sevaHandler.GetSevas) // Devotee view: Paginated & Filterable Sevas
}

// ========== Entity ==========
	{
		entityRepo := entity.NewRepository(database.DB)
		// donationRepo := donation.NewRepository(database.DB)
		profileRepo := userprofile.NewRepository(database.DB)
		profileService := userprofile.NewService(profileRepo, authRepo, auditSvc)

		// ✅ INJECT AUDIT SERVICE INTO ENTITY SERVICE
		entityService := entity.NewService(entityRepo, profileService, auditSvc)
		entityHandler := entity.NewHandler(entityService)

		// Add special endpoint for templeadmins to view their created entities
		protected.GET("/entities/by-creator", middleware.RBACMiddleware("templeadmin"), func(c *gin.Context) {
			// Get user ID from context
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
			
			// Call repository to get entities created by this user
			entities, err := entityRepo.GetEntitiesByCreator(user.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch temples", "details": err.Error()})
				return
			}
			
			c.JSON(http.StatusOK, entities)
		})

		// In routes.go - update the entity routes section
entityRoutes := protected.Group("/entities")
// Allow templeadmin, standarduser, monitoringuser to access entity routes
entityRoutes.Use(middleware.RequireTempleAccess())
{
    // Write operations - only templeadmin and standarduser can access
    writeRoutes := entityRoutes.Group("")
    writeRoutes.Use(middleware.RequireWriteAccess())
    {
        // This line should be REMOVED since we moved it out of this group
        // writeRoutes.POST("/", entityHandler.CreateEntity)
        writeRoutes.PUT("/:id", entityHandler.UpdateEntity)
        writeRoutes.DELETE("/:id", entityHandler.DeleteEntity)
        writeRoutes.PATCH("/:entityID/devotees/:userID/status", entityHandler.UpdateDevoteeMembershipStatus)
    }
    
    // Read operations - all three roles can access
    // This line should be REMOVED since we moved it out of this group
    // entityRoutes.GET("/", entityHandler.GetAllEntities)
    entityRoutes.GET("/:id", entityHandler.GetEntityByID)
    entityRoutes.GET("/:id/devotees", entityHandler.GetDevoteesByEntity)
    entityRoutes.GET("/:id/devotee-stats", entityHandler.GetDevoteeStats)
    entityRoutes.GET("/dashboard-summary", entityHandler.GetDashboardSummary)
}

		// Special endpoints that bypass temple access check
		protected.POST("/entities", middleware.RBACMiddleware("templeadmin", "superadmin", "standarduser"), entityHandler.CreateEntity)
		protected.GET("/entities", middleware.RBACMiddleware("templeadmin", "superadmin", "standarduser"), entityHandler.GetAllEntities)
	}
// ========== Event & RSVP ==========
eventRepo := event.NewRepository(database.DB)
eventService := event.NewService(eventRepo, auditSvc) // ✅ INJECT AUDIT SERVICE
eventHandler := event.NewHandler(eventService)

// Event routes - all require temple access
eventRoutes := protected.Group("/events")
eventRoutes.Use(middleware.RequireTempleAccess())
{
    // Write operations - only templeadmin and standarduser can access
    writeRoutes := eventRoutes.Group("")
    writeRoutes.Use(middleware.RequireWriteAccess())
    {
        writeRoutes.POST("/", eventHandler.CreateEvent)
        writeRoutes.POST("", eventHandler.CreateEvent)
        writeRoutes.PUT("/:id", eventHandler.UpdateEvent)
        writeRoutes.DELETE("/:id", eventHandler.DeleteEvent)
    }
    
    // Read operations - all three roles can access
    eventRoutes.GET("/", eventHandler.ListEvents)
    eventRoutes.GET("/:id", eventHandler.GetEventByID)
    eventRoutes.GET("/upcoming", eventHandler.GetUpcomingEvents)
    eventRoutes.GET("/stats", eventHandler.GetEventStats)
}

// Event RSVP routes (keeping existing logic for devotee/volunteer access)
{
    rsvpRepo := eventrsvp.NewRepository(database.DB)
    rsvpService := eventrsvp.NewService(rsvpRepo, eventService)
    rsvpHandler := eventrsvp.NewHandler(rsvpService, eventService)

    rsvpRoutes := protected.Group("/event-rsvps")
    rsvpRoutes.POST("/:eventID", middleware.RBACMiddleware("devotee", "volunteer"), rsvpHandler.CreateRSVP)
    rsvpRoutes.GET("/:eventID", middleware.RBACMiddleware("devotee"), rsvpHandler.GetRSVPsByEvent)
    rsvpRoutes.GET("/my", middleware.RBACMiddleware("devotee", "volunteer"), rsvpHandler.GetMyRSVPs)
}

	// ========== User Profile & Membership ==========
	profileRepo := userprofile.NewRepository(database.DB)
	profileService := userprofile.NewService(profileRepo, authRepo, auditSvc) // ✅ INJECT AUDIT SERVICE
	profileHandler := userprofile.NewHandler(profileService)

	profileRoutes := protected.Group("/profiles")
	{
		profileRoutes.GET("/me", middleware.RBACMiddleware("devotee"), profileHandler.GetMyProfile)
		profileRoutes.POST("/", middleware.RBACMiddleware("devotee"), profileHandler.CreateOrUpdateProfile)
		profileRoutes.PUT("/", middleware.RBACMiddleware("devotee"), profileHandler.CreateOrUpdateProfile)
	}

	// ========== Membership (Join Temples) ==========
	membershipRoutes := protected.Group("/memberships")
	{
		membershipRoutes.POST("/", middleware.RBACMiddleware("devotee", "volunteer"), profileHandler.JoinTemple)
		membershipRoutes.GET("/", middleware.RBACMiddleware("devotee", "volunteer"), profileHandler.ListMemberships)
	}

	// ========== Temple Search ==========
	templeSearchRoutes := protected.Group("/temples")
	{
		templeSearchRoutes.GET("/search", middleware.RBACMiddleware("devotee", "volunteer"), profileHandler.SearchTemples)
		templeSearchRoutes.GET("/recent", middleware.RBACMiddleware("devotee", "volunteer"), profileHandler.GetRecentTemples)
	}

// ========== Donations with New Permission System ==========
	{
		donationRepo := donation.NewRepository(database.DB)
		donationService := donation.NewService(donationRepo, cfg, auditSvc) // ✅ INJECT AUDIT SERVICE
		donationHandler := donation.NewHandler(donationService)

		donationRoutes := protected.Group("/donations")
		{
			// ========== DEVOTEE ROUTES (UNCHANGED) ==========
			devoteeRoutes := donationRoutes.Group("")
			devoteeRoutes.Use(middleware.RBACMiddleware("devotee"))
			{
				devoteeRoutes.POST("/", donationHandler.CreateDonation)        // Create donation
				devoteeRoutes.POST("/verify", donationHandler.VerifyDonation)  // Verify payment
				devoteeRoutes.GET("/my", donationHandler.GetMyDonations)       // View my donations
			}

			// ========== TEMPLE ADMIN ROUTES (UPDATED PERMISSIONS) ==========
			templeRoutes := donationRoutes.Group("")
			templeRoutes.Use(middleware.RequireTempleAccess()) // Allow templeadmin, standarduser, monitoringuser
			{
				// Read-only operations - all three roles can access
				templeRoutes.GET("/", donationHandler.GetDonationsByEntity)         // View entity donations
				templeRoutes.GET("/dashboard", donationHandler.GetDashboard)        // Dashboard stats
				templeRoutes.GET("/top-donors", donationHandler.GetTopDonors)       // Top donors
				templeRoutes.GET("/analytics", donationHandler.GetAnalytics)        // Analytics

				// Write operations - only templeadmin and standarduser can access
				writeRoutes := templeRoutes.Group("")
				writeRoutes.Use(middleware.RequireWriteAccess())
				{
					writeRoutes.GET("/export", donationHandler.ExportDonations)     // Export donations
				}
			}

			// ========== SHARED ROUTES (BOTH DEVOTEE AND TEMPLE ADMIN) ==========
			// Receipt generation - both devotees and temple admins can access
			donationRoutes.GET("/:id/receipt", 
				middleware.RBACMiddleware("devotee", "templeadmin", "standarduser", "monitoringuser"), 
				donationHandler.GenerateReceipt)
			
			// Recent donations - both devotees and temple admins can access
			donationRoutes.GET("/recent", 
				middleware.RBACMiddleware("devotee", "templeadmin", "standarduser", "monitoringuser"), 
				donationHandler.GetRecentDonations)
		}
	}

// ========== Notifications ==========
	{
		notificationRepo := notification.NewRepository(database.DB)
		notificationService := notification.NewService(notificationRepo, authRepo, cfg, auditSvc) // ✅ INJECT AUDIT SERVICE

		notificationHandler := notification.NewHandler(notificationService, auditSvc) // ✅ INJECT AUDIT SERVICE

		// Updated to use new middleware system
		notificationRoutes := protected.Group("/notifications")
		notificationRoutes.Use(middleware.RequireTempleAccess()) // Allow templeadmin, standarduser, monitoringuser
		{
			// Write operations - only templeadmin and standarduser can access
			writeRoutes := notificationRoutes.Group("")
			writeRoutes.Use(middleware.RequireWriteAccess())
			{
				// 🧩 Templates
				writeRoutes.POST("/templates", notificationHandler.CreateTemplate)
				writeRoutes.PUT("/templates/:id", notificationHandler.UpdateTemplate)
				writeRoutes.DELETE("/templates/:id", notificationHandler.DeleteTemplate)

				// 📬 Send Notification
				writeRoutes.POST("/send", notificationHandler.SendNotification)
			}

			// Read operations - all three roles can access
			notificationRoutes.GET("/templates", notificationHandler.GetTemplates)
			notificationRoutes.GET("/templates/:id", notificationHandler.GetTemplateByID)

			// 📜 View Logs
			notificationRoutes.GET("/logs", notificationHandler.GetMyNotifications)
		}
	}

// ========== Reports ==========
{
	reportsRepo := reports.NewRepository(database.DB)
	reportsExporter := reports.NewReportExporter()
	reportsService := reports.NewReportService(reportsRepo, reportsExporter, auditSvc) // ✅ INJECT AUDIT SERVICE
	reportsHandler := reports.NewHandler(reportsService, reportsRepo, auditSvc)        // ✅ INJECT AUDIT SERVICE

	reportsRoutes := protected.Group("/entities/:id/reports")
	reportsRoutes.Use(middleware.RequireTempleAccess()) // Allow templeadmin, standarduser, monitoringuser
	{
		// All report endpoints are read-only by default, but may generate downloadable files
		// Since report generation can be considered a "sensitive" operation, we can optionally require write access
		// However, for now, let's allow all three roles to view and export reports
		
		reportsRoutes.GET("/activities", reportsHandler.GetActivities)
		reportsRoutes.GET("/temple-registered", reportsHandler.GetTempleRegisteredReport)
		reportsRoutes.GET("/devotee-birthdays", reportsHandler.GetDevoteeBirthdaysReport)
		reportsRoutes.GET("/devotee-list", reportsHandler.GetDevoteeListReport)
		reportsRoutes.GET("/devotee-profile", reportsHandler.GetDevoteeProfileReport)
		reportsRoutes.GET("/audit-logs", reportsHandler.GetAuditLogsReport)  // fixed typo
		
		
		// If you want to restrict export functionality to only users with write access, 
		// you can create a separate group with write access requirement:
		/*
		exportRoutes := reportsRoutes.Group("")
		exportRoutes.Use(middleware.RequireWriteAccess())
		{
			exportRoutes.GET("/activities", reportsHandler.GetActivities) // when ?format= is provided
			exportRoutes.GET("/temple-registered", reportsHandler.GetTempleRegisteredReport) // when ?format= is provided
			// ... other export endpoints
		}
		*/
	}
}

	// Serve the SPA (Single Page Application) for any other route
	r.NoRoute(func(c *gin.Context) {
		// Check if the request is for an API endpoint
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			// If it's an API route that wasn't found, return 404 JSON
			c.JSON(http.StatusNotFound, gin.H{"error": "API endpoint not found"})
			return
		}

		// Serve the index.html file for all other routes
		c.File("./public/index.html")
	})
}