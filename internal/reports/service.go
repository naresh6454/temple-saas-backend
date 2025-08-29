package reports

import (
    "context"
    "fmt"

    "github.com/sharath018/temple-management-backend/internal/auditlog"
)

// ReportService performs business logic and coordinates repo + exporter.
type ReportService interface {
    GetActivities(req ActivitiesReportRequest) (ReportData, error)
    ExportActivities(ctx context.Context, req ActivitiesReportRequest, userID *uint, ip string) ([]byte, string, string, error)

    GetTempleRegisteredReport(req TempleRegisteredReportRequest, entityIDs []string) ([]TempleRegisteredReportRow, error)
    ExportTempleRegisteredReport(ctx context.Context, req TempleRegisteredReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error)

    GetDevoteeBirthdaysReport(req DevoteeBirthdaysReportRequest, entityIDs []string) ([]DevoteeBirthdayReportRow, error)
    ExportDevoteeBirthdaysReport(ctx context.Context, req DevoteeBirthdaysReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error)

    GetDevoteeListReport(req DevoteeListReportRequest, entityIDs []string) ([]DevoteeListReportRow, error)
    ExportDevoteeListReport(ctx context.Context, req DevoteeListReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error)

    GetDevoteeProfileReport(req DevoteeProfileReportRequest, entityIDs []string) ([]DevoteeProfileReportRow, error)
    ExportDevoteeProfileReport(ctx context.Context, req DevoteeProfileReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error)

    GetAuditLogsReport(req AuditLogReportRequest, entityIDs []string) ([]AuditLogReportRow, error)
    ExportAuditLogsReport(ctx context.Context, req AuditLogReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error)
    
    // New methods for UserDetails and ApprovalStatus
    GetApprovalStatusReport(req ApprovalStatusReportRequest, entityIDs []string) ([]ApprovalStatusReportRow, error)
    ExportApprovalStatusReport(ctx context.Context, req ApprovalStatusReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error)
    
    GetUserDetailsReport(req UserDetailReportRequest, entityIDs []string) ([]UserDetailsReportRow, error)
    ExportUserDetailsReport(ctx context.Context, req UserDetailReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error)
}

type reportService struct {
    repo     ReportRepository
    exporter ReportExporter
    auditSvc auditlog.Service
}

func NewReportService(repo ReportRepository, exporter ReportExporter, auditSvc auditlog.Service) ReportService {
    return &reportService{
        repo:     repo,
        exporter: exporter,
        auditSvc: auditSvc,
    }
}

// ===============================
// Utility
// ===============================

func convertUintSlice(strs []string) []uint {
    out := make([]uint, 0, len(strs))
    for _, s := range strs {
        var id uint
        _, err := fmt.Sscan(s, &id)
        if err == nil {
            out = append(out, id)
        }
    }
    return out
}

// ===============================
// Activities Reports
// ===============================

func (s *reportService) GetActivities(req ActivitiesReportRequest) (ReportData, error) {
    if req.Type != ReportTypeEvents && req.Type != ReportTypeSevas &&
        req.Type != ReportTypeBookings && req.Type != ReportTypeDonations {
        return ReportData{}, fmt.Errorf("invalid report type: %s", req.Type)
    }
    start := req.StartDate
    end := req.EndDate

    var data ReportData
    var err error
    switch req.Type {
    case ReportTypeEvents:
        data.Events, err = s.repo.GetEvents(convertUintSlice(req.EntityIDs), start, end)
    case ReportTypeSevas:
        data.Sevas, err = s.repo.GetSevas(convertUintSlice(req.EntityIDs), start, end)
    case ReportTypeBookings:
        data.Bookings, err = s.repo.GetSevaBookings(convertUintSlice(req.EntityIDs), start, end)
    case ReportTypeDonations:
        data.Donations, err = s.repo.GetDonations(convertUintSlice(req.EntityIDs), start, end)
    }
    return data, err
}

func (s *reportService) ExportActivities(ctx context.Context, req ActivitiesReportRequest, userID *uint, ip string) ([]byte, string, string, error) {
    data, err := s.GetActivities(req)
    if err != nil {
        details := map[string]interface{}{
            "report_type": req.Type,
            "format":      req.Format,
            "error":       err.Error(),
        }
        s.auditSvc.LogAction(ctx, userID, nil, "TEMPLE_ACTIVITIES_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
        return nil, "", "", err
    }

    bytes, filename, mimeType, err := s.exporter.Export(req.Type, req.Format, data)
    if err != nil {
        details := map[string]interface{}{
            "report_type": req.Type,
            "format":      req.Format,
            "error":       err.Error(),
        }
        s.auditSvc.LogAction(ctx, userID, nil, "TEMPLE_ACTIVITIES_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
        return nil, "", "", err
    }

    details := map[string]interface{}{
        "report_type": req.Type,
        "format":      req.Format,
        "filename":    filename,
        "entity_ids":  req.EntityIDs,
        "date_range":  req.DateRange,
    }
    s.auditSvc.LogAction(ctx, userID, nil, "TEMPLE_ACTIVITIES_REPORT_DOWNLOADED", details, ip, "success")

    return bytes, filename, mimeType, nil
}

// ===============================
// Temple Registered Reports
// ===============================

func (s *reportService) GetTempleRegisteredReport(req TempleRegisteredReportRequest, entityIDs []string) ([]TempleRegisteredReportRow, error) {
    return s.repo.GetTemplesRegistered(convertUintSlice(entityIDs), req.StartDate, req.EndDate, req.Status)
}

func (s *reportService) ExportTempleRegisteredReport(ctx context.Context, req TempleRegisteredReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error) {
    rows, err := s.GetTempleRegisteredReport(req, entityIDs)
    if err != nil {
        details := map[string]interface{}{
            "report_type": "temple_registered",
            "format":      req.Format,
            "error":       err.Error(),
        }
        s.auditSvc.LogAction(ctx, userID, nil, "TEMPLE_REGISTER_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
        return nil, "", "", err
    }

    data := ReportData{TemplesRegistered: rows}
    bytes, filename, mimeType, err := s.exporter.Export(reportType, req.Format, data)
    if err != nil {
        details := map[string]interface{}{
            "report_type": "temple_registered",
            "format":      req.Format,
            "error":       err.Error(),
        }
        s.auditSvc.LogAction(ctx, userID, nil, "TEMPLE_REGISTER_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
        return nil, "", "", err
    }

    details := map[string]interface{}{
        "report_type":  "temple_registered",
        "format":       req.Format,
        "filename":     filename,
        "entity_ids":   entityIDs,
        "status":       req.Status,
        "date_range":   req.DateRange,
        "record_count": len(rows),
    }
    s.auditSvc.LogAction(ctx, userID, nil, "TEMPLE_REGISTER_REPORT_DOWNLOADED", details, ip, "success")

    return bytes, filename, mimeType, nil
}

// ===============================
// Devotee Reports
// ===============================

func (s *reportService) GetDevoteeBirthdaysReport(req DevoteeBirthdaysReportRequest, entityIDs []string) ([]DevoteeBirthdayReportRow, error) {
    return s.repo.GetDevoteeBirthdays(convertUintSlice(entityIDs), req.StartDate, req.EndDate)
}

func (s *reportService) ExportDevoteeBirthdaysReport(ctx context.Context, req DevoteeBirthdaysReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error) {
    rows, err := s.GetDevoteeBirthdaysReport(req, entityIDs)
    if err != nil {
        details := map[string]interface{}{
            "report_type": "devotee_birthdays",
            "format":      req.Format,
            "error":       err.Error(),
        }
        s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_BIRTHDAYS_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
        return nil, "", "", err
    }

    data := ReportData{DevoteeBirthdays: rows}
    bytes, filename, mimeType, err := s.exporter.Export(reportType, req.Format, data)
    if err != nil {
        details := map[string]interface{}{
            "report_type": "devotee_birthdays",
            "format":      req.Format,
            "error":       err.Error(),
        }
        s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_BIRTHDAYS_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
        return nil, "", "", err
    }

    details := map[string]interface{}{
        "report_type":  "devotee_birthdays",
        "format":       req.Format,
        "filename":     filename,
        "entity_ids":   entityIDs,
        "date_range":   req.DateRange,
        "record_count": len(rows),
    }
    s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_BIRTHDAYS_REPORT_DOWNLOADED", details, ip, "success")

    return bytes, filename, mimeType, nil
}

func (s *reportService) GetDevoteeListReport(req DevoteeListReportRequest, entityIDs []string) ([]DevoteeListReportRow, error) {
    return s.repo.GetDevoteeList(convertUintSlice(entityIDs), req.StartDate, req.EndDate, req.Status)
}

func (s *reportService) ExportDevoteeListReport(ctx context.Context, req DevoteeListReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error) {
    rows, err := s.GetDevoteeListReport(req, entityIDs)
    if err != nil {
        details := map[string]interface{}{
            "report_type": "devotee_list",
            "format":      req.Format,
            "error":       err.Error(),
            "status":      req.Status,
        }
        s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_LIST_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
        return nil, "", "", err
    }

    data := ReportData{DevoteeList: rows}
    bytes, filename, mimeType, err := s.exporter.Export(reportType, req.Format, data)
    if err != nil {
        details := map[string]interface{}{
            "report_type": "devotee_list",
            "format":      req.Format,
            "error":       err.Error(),
            "status":      req.Status,
        }
        s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_LIST_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
        return nil, "", "", err
    }

    details := map[string]interface{}{
        "report_type":  "devotee_list",
        "format":       req.Format,
        "filename":     filename,
        "entity_ids":   entityIDs,
        "status":       req.Status,
        "date_range":   req.DateRange,
        "record_count": len(rows),
    }
    s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_LIST_REPORT_DOWNLOADED", details, ip, "success")

    return bytes, filename, mimeType, nil
}

// ===============================
// Devotee Profile Reports
// ===============================

func (s *reportService) GetDevoteeProfileReport(req DevoteeProfileReportRequest, entityIDs []string) ([]DevoteeProfileReportRow, error) {
    return s.repo.GetDevoteeProfiles(convertUintSlice(entityIDs), req.StartDate, req.EndDate, req.Status)
}

func (s *reportService) ExportDevoteeProfileReport(ctx context.Context, req DevoteeProfileReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error) {
    rows, err := s.GetDevoteeProfileReport(req, entityIDs)
    if err != nil {
        details := map[string]interface{}{
            "report_type": "devotee_profile",
            "format":      req.Format,
            "error":       err.Error(),
            "status":      req.Status,
        }
        s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_PROFILE_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
        return nil, "", "", err
    }

    data := ReportData{DevoteeProfiles: rows}
    bytes, filename, mimeType, err := s.exporter.Export(reportType, req.Format, data)
    if err != nil {
        details := map[string]interface{}{
            "report_type": "devotee_profile",
            "format":      req.Format,
            "error":       err.Error(),
            "status":      req.Status,
        }
        s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_PROFILE_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
        return nil, "", "", err
    }

    details := map[string]interface{}{
        "report_type":  "devotee_profile",
        "format":       req.Format,
        "filename":     filename,
        "entity_ids":   entityIDs,
        "status":       req.Status,
        "date_range":   req.DateRange,
        "record_count": len(rows),
    }
    s.auditSvc.LogAction(ctx, userID, nil, "DEVOTEE_PROFILE_REPORT_DOWNLOADED", details, ip, "success")

    return bytes, filename, mimeType, nil
}

// ===============================
// Audit Logs Reports
// ===============================

func (s *reportService) GetAuditLogsReport(req AuditLogReportRequest, entityIDs []string) ([]AuditLogReportRow, error) {
    ids := convertUintSlice(entityIDs)

    var actionFilters []string
    if req.Action != "" {
        actionFilters = append(actionFilters, req.Action)
    }

    // Pass status separately to repo
    return s.repo.GetAuditLogs(ids, req.StartDate, req.EndDate, actionFilters, req.Status)
}

func (s *reportService) ExportAuditLogsReport(ctx context.Context, req AuditLogReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error) {
    rows, err := s.GetAuditLogsReport(req, entityIDs)
    if err != nil {
        details := map[string]interface{}{
            "report_type": "audit_logs",
            "format":      req.Format,
            "error":       err.Error(),
            "action":      req.Action,
            "status":      req.Status,
        }
        s.auditSvc.LogAction(ctx, userID, nil, "AUDIT_LOGS_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
        return nil, "", "", err
    }

    data := ReportData{AuditLogs: rows}
    bytes, filename, mimeType, err := s.exporter.Export(reportType, req.Format, data)
    if err != nil {
        details := map[string]interface{}{
            "report_type": "audit_logs",
            "format":      req.Format,
            "error":       err.Error(),
            "action":      req.Action,
            "status":      req.Status,
        }
        s.auditSvc.LogAction(ctx, userID, nil, "AUDIT_LOGS_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
        return nil, "", "", err
    }

    details := map[string]interface{}{
        "report_type":  "audit_logs",
        "format":       req.Format,
        "filename":     filename,
        "entity_ids":   entityIDs,
        "action":       req.Action,
        "status":       req.Status,
        "date_range":   req.DateRange,
        "record_count": len(rows),
    }
    s.auditSvc.LogAction(ctx, userID, nil, "AUDIT_LOGS_REPORT_DOWNLOADED", details, ip, "success")

    return bytes, filename, mimeType, nil
}

// ===============================
// Approval Status Reports
// ===============================

// GetApprovalStatusReport retrieves approval status data based on filters
func (s *reportService) GetApprovalStatusReport(req ApprovalStatusReportRequest, entityIDs []string) ([]ApprovalStatusReportRow, error) {
    // Convert string IDs to uint
    ids := convertUintSlice(entityIDs)
    
    // Validate role if specified - ensure only Tenant or Temple roles are allowed
    if req.Role != "" && req.Role != "Tenant" && req.Role != "Temple" {
        // Return empty result for invalid roles instead of error
        return []ApprovalStatusReportRow{}, nil
    }

    // Get approval status data from repository
    rows, err := s.repo.GetApprovalStatus(ids, req.StartDate, req.EndDate, req.Role, req.Status)
    if err != nil {
        return nil, fmt.Errorf("failed to get approval status data: %w", err)
    }

    // Log the count of rows for debugging
    fmt.Printf("Retrieved %d approval status rows\n", len(rows))
    
    return rows, nil
}

// ExportApprovalStatusReport exports approval status data in the requested format
func (s *reportService) ExportApprovalStatusReport(ctx context.Context, req ApprovalStatusReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error) {
    // Get the approval status data
    rows, err := s.GetApprovalStatusReport(req, entityIDs)
    if err != nil {
        details := map[string]interface{}{
            "report_type": "approval_status",
            "format":      req.Format,
            "error":       err.Error(),
            "role":        req.Role,
            "status":      req.Status,
        }
        s.auditSvc.LogAction(ctx, userID, nil, "APPROVAL_STATUS_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
        return nil, "", "", err
    }

    // If no rows were found, log that information but continue with export
    if len(rows) == 0 {
        fmt.Println("No approval status rows found matching the filters")
    }

    // Prepare data for export
    data := ReportData{ApprovalStatus: rows}
    
    // Export using the appropriate format
    bytes, filename, mimeType, err := s.exporter.Export(reportType, req.Format, data)
    if err != nil {
        details := map[string]interface{}{
            "report_type": "approval_status",
            "format":      req.Format,
            "error":       err.Error(),
            "role":        req.Role,
            "status":      req.Status,
        }
        s.auditSvc.LogAction(ctx, userID, nil, "APPROVAL_STATUS_REPORT_DOWNLOAD_FAILED", details, ip, "failure")
        return nil, "", "", err
    }

    // Log successful export
    details := map[string]interface{}{
        "report_type":  "approval_status",
        "format":       req.Format,
        "filename":     filename,
        "entity_ids":   entityIDs,
        "role":         req.Role,
        "status":       req.Status,
        "date_range":   req.DateRange,
        "record_count": len(rows),
    }
    s.auditSvc.LogAction(ctx, userID, nil, "APPROVAL_STATUS_REPORT_DOWNLOADED", details, ip, "success")

    return bytes, filename, mimeType, nil
}

// ===============================
// User Details Reports
// ===============================

func (s *reportService) GetUserDetailsReport(req UserDetailReportRequest, entityIDs []string) ([]UserDetailsReportRow, error) {
	var ids []uint
	if entityIDs != nil && len(entityIDs) > 0 {
		ids = convertUintSlice(entityIDs)
	} else {
		// nil or empty → superadmin: fetch all users
		ids = nil
	}

	return s.repo.GetUserDetails(ids, req.StartDate, req.EndDate, req.Role, req.Status)
}

func (s *reportService) ExportUserDetailsReport(ctx context.Context, req UserDetailReportRequest, entityIDs []string, reportType string, userID *uint, ip string) ([]byte, string, string, error) {
	rows, err := s.GetUserDetailsReport(req, entityIDs)
	if err != nil {
		s.auditSvc.LogAction(ctx, userID, nil, "USER_DETAILS_REPORT_DOWNLOAD_FAILED", map[string]interface{}{
			"report_type": "user_details",
			"format":      req.Format,
			"error":       err.Error(),
			"role":        req.Role,
			"status":      req.Status,
		}, ip, "failure")
		return nil, "", "", err
	}

	data := ReportData{UserDetails: rows}
	bytes, filename, mimeType, err := s.exporter.Export(reportType, req.Format, data)
	if err != nil {
		s.auditSvc.LogAction(ctx, userID, nil, "USER_DETAILS_REPORT_DOWNLOAD_FAILED", map[string]interface{}{
			"report_type": "user_details",
			"format":      req.Format,
			"error":       err.Error(),
			"role":        req.Role,
			"status":      req.Status,
		}, ip, "failure")
		return nil, "", "", err
	}

	s.auditSvc.LogAction(ctx, userID, nil, "USER_DETAILS_REPORT_DOWNLOADED", map[string]interface{}{
		"report_type":  "user_details",
		"format":       req.Format,
		"filename":     filename,
		"entity_ids":   entityIDs,
		"role":         req.Role,
		"status":       req.Status,
		"date_range":   req.DateRange,
		"record_count": len(rows),
	}, ip, "success")

	return bytes, filename, mimeType, nil
}