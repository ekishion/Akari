package repository

import (
	"encoding/json"
	"fmt"
	"time"

	"akari-bridge/internal/domain"
	"akari-bridge/internal/engine"
)

type RuleRepository struct {
	conn *DBConn
}

func NewRuleRepository(conn *DBConn) domain.IRuleRepository {
	return &RuleRepository{conn: conn}
}

func (r *RuleRepository) GetAllPlugins() ([]engine.Plugin, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	rows, err := r.conn.db.Query(`SELECT name, version, enabled, data_json FROM plugins ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []engine.Plugin
	for rows.Next() {
		var name, version, dataJson string
		var enabledInt int
		if err := rows.Scan(&name, &version, &enabledInt, &dataJson); err == nil {
			var p engine.Plugin
			if err := json.Unmarshal([]byte(dataJson), &p); err == nil {
				p.Enabled = enabledInt == 1
				list = append(list, p)
			}
		}
	}
	return list, nil
}

func (r *RuleRepository) GetPluginByName(name string) (*engine.Plugin, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	var version, dataJson string
	var enabledInt int
	err := r.conn.db.QueryRow(`SELECT version, enabled, data_json FROM plugins WHERE name = ?`, name).Scan(&version, &enabledInt, &dataJson)
	if err != nil {
		return nil, err
	}

	var p engine.Plugin
	if err := json.Unmarshal([]byte(dataJson), &p); err != nil {
		return nil, err
	}
	p.Enabled = enabledInt == 1
	return &p, nil
}

func (r *RuleRepository) SavePlugin(p *engine.Plugin) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	data, err := json.Marshal(p)
	if err != nil {
		return err
	}

	enabledInt := 0
	if p.Enabled {
		enabledInt = 1
	}

	query := `
	INSERT INTO plugins (id, name, version, enabled, data_json, updated_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(name) DO UPDATE SET
		version = excluded.version,
		enabled = excluded.enabled,
		data_json = excluded.data_json,
		updated_at = excluded.updated_at
	`
	_, err = r.conn.db.Exec(query, p.Name, p.Name, p.Version, enabledInt, string(data), time.Now())
	return err
}

func (r *RuleRepository) DeletePlugin(name string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	_, err := r.conn.db.Exec(`DELETE FROM plugins WHERE name = ?`, name)
	return err
}

func (r *RuleRepository) TogglePlugin(name string, enabled bool) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	enInt := 0
	if enabled {
		enInt = 1
	}
	_, err := r.conn.db.Exec(`UPDATE plugins SET enabled = ?, updated_at = ? WHERE name = ?`, enInt, time.Now(), name)
	return err
}

func (r *RuleRepository) ImportPluginsJSON(data []byte) (int, error) {
	var imported []engine.Plugin
	if err := json.Unmarshal(data, &imported); err != nil {
		var single engine.Plugin
		if err2 := json.Unmarshal(data, &single); err2 != nil {
			return 0, fmt.Errorf("invalid json plugin structure: %w", err)
		}
		imported = append(imported, single)
	}

	count := 0
	for _, p := range imported {
		if p.Name != "" {
			if err := r.SavePlugin(&p); err == nil {
				count++
			}
		}
	}
	return count, nil
}

func (r *RuleRepository) ExportPluginsJSON() ([]byte, error) {
	plugins, err := r.GetAllPlugins()
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(plugins, "", "  ")
}
