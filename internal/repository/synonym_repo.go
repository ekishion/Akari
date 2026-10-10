package repository

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"akari-bridge/internal/domain"
)

type SynonymRepository struct {
	conn *DBConn
}

func NewSynonymRepository(conn *DBConn) domain.ISynonymRepository {
	return &SynonymRepository{conn: conn}
}

func (r *SynonymRepository) GetGlobalSynonyms() (map[string]string, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	rows, err := r.conn.db.Query(`SELECT pattern, replacement FROM global_synonyms WHERE enabled = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]string)
	for rows.Next() {
		var p, rep string
		if err := rows.Scan(&p, &rep); err == nil {
			res[p] = rep
		}
	}
	return res, nil
}

func (r *SynonymRepository) SaveGlobalSynonyms(synonyms map[string]string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	tx, err := r.conn.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO global_synonyms (pattern, replacement, enabled)
		VALUES (?, ?, 1)
		ON CONFLICT(pattern) DO UPDATE SET replacement = excluded.replacement, enabled = 1
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for p, rep := range synonyms {
		p = strings.TrimSpace(p)
		rep = strings.TrimSpace(rep)
		if p != "" && rep != "" {
			if _, err := stmt.Exec(p, rep); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (r *SynonymRepository) ListGlobalSynonyms() ([]map[string]any, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	rows, err := r.conn.db.Query(`SELECT pattern, replacement, enabled FROM global_synonyms ORDER BY pattern ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []map[string]any
	for rows.Next() {
		var p, rep string
		var en int
		if err := rows.Scan(&p, &rep, &en); err == nil {
			list = append(list, map[string]any{
				"pattern":     p,
				"replacement": rep,
				"enabled":     en == 1,
			})
		}
	}
	return list, nil
}

func (r *SynonymRepository) UpsertGlobalSynonym(pattern, replacement string, enabled bool) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	enInt := 0
	if enabled {
		enInt = 1
	}

	query := `
	INSERT INTO global_synonyms (pattern, replacement, enabled)
	VALUES (?, ?, ?)
	ON CONFLICT(pattern) DO UPDATE SET replacement = excluded.replacement, enabled = excluded.enabled
	`
	_, err := r.conn.db.Exec(query, strings.TrimSpace(pattern), strings.TrimSpace(replacement), enInt)
	return err
}

func (r *SynonymRepository) DeleteGlobalSynonymByPattern(pattern string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	_, err := r.conn.db.Exec(`DELETE FROM global_synonyms WHERE pattern = ?`, pattern)
	return err
}

func (r *SynonymRepository) ResetDefaultGlobalSynonyms() error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	defaultSeeds := [][2]string{
		{"超时空", "超"},
		{"已经死了", "已死"},
		{"电影版", "剧场版"},
		{"总集篇", "剧场版"},
	}
	for _, pair := range defaultSeeds {
		_, _ = r.conn.db.Exec(`INSERT INTO global_synonyms (pattern, replacement, enabled) VALUES (?, ?, 1) ON CONFLICT(pattern) DO NOTHING`, pair[0], pair[1])
	}
	return nil
}

func (r *SynonymRepository) GetSubjectAliases(subId int) ([]string, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	var raw string
	err := r.conn.db.QueryRow(`SELECT aliases_json FROM subject_aliases WHERE subject_id = ?`, subId).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	var aliases []string
	if err := json.Unmarshal([]byte(raw), &aliases); err != nil {
		return nil, err
	}
	return aliases, nil
}

func (r *SynonymRepository) ListSubjectAliases() ([]map[string]any, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	rows, err := r.conn.db.Query(`SELECT subject_id, title, aliases_json, updated_at FROM subject_aliases ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []map[string]any
	for rows.Next() {
		var subId int
		var title, raw string
		var updatedAt time.Time
		if err := rows.Scan(&subId, &title, &raw, &updatedAt); err == nil {
			var aliases []string
			_ = json.Unmarshal([]byte(raw), &aliases)
			list = append(list, map[string]any{
				"subjectId": subId,
				"title":     title,
				"aliases":   aliases,
				"updatedAt": updatedAt,
			})
		}
	}
	return list, nil
}

func (r *SynonymRepository) UpsertSubjectAliases(subId int, title string, aliases []string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	data, err := json.Marshal(aliases)
	if err != nil {
		return err
	}

	query := `
	INSERT INTO subject_aliases (subject_id, title, aliases_json, updated_at)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(subject_id) DO UPDATE SET
		title = excluded.title,
		aliases_json = excluded.aliases_json,
		updated_at = excluded.updated_at
	`
	_, err = r.conn.db.Exec(query, subId, strings.TrimSpace(title), string(data), time.Now())
	return err
}

func (r *SynonymRepository) SaveSubjectAliases(subId int, aliases []string) error {
	return r.UpsertSubjectAliases(subId, "", aliases)
}

func (r *SynonymRepository) DeleteSubjectAliases(subId int) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	_, err := r.conn.db.Exec(`DELETE FROM subject_aliases WHERE subject_id = ?`, subId)
	return err
}

