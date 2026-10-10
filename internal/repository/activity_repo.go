package repository

import (
	"time"

	"akari-bridge/internal/domain"
)

type ActivityRepository struct {
	conn *DBConn
}

func NewActivityRepository(conn *DBConn) domain.IActivityRepository {
	return &ActivityRepository{conn: conn}
}

func (r *ActivityRepository) AddActivityLog(logType, action, detail, ip string) error {
	return r.RecordAuditLog(logType, ip, "", action+": "+detail)
}

func (r *ActivityRepository) RecordAuditLog(action, ip, userAgent, details string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	_, err := r.conn.db.Exec(`
		INSERT INTO audit_logs (event_type, details, ip_address, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, action, details, ip, userAgent, time.Now())
	return err
}

func (r *ActivityRepository) GetRecentActivityLogs(limit int) ([]domain.ActivityLog, error) {
	logs, _, err := r.GetAuditLogs(limit, 0)
	return logs, err
}

func (r *ActivityRepository) GetAuditLogs(limit, offset int) ([]domain.ActivityLog, int, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	var total int
	if err := r.conn.db.QueryRow(`SELECT COUNT(*) FROM audit_logs`).Scan(&total); err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.conn.db.Query(`
		SELECT id, event_type, details, ip_address, created_at
		FROM audit_logs
		ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []domain.ActivityLog
	for rows.Next() {
		var l domain.ActivityLog
		if err := rows.Scan(&l.ID, &l.Type, &l.Detail, &l.IP, &l.Timestamp); err == nil {
			logs = append(logs, l)
		}
	}
	return logs, total, nil
}

func (r *ActivityRepository) ClearAuditLogs() error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	_, err := r.conn.db.Exec(`DELETE FROM audit_logs`)
	return err
}
