package repository

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"akari-bridge/internal/domain"
)

type PlaybackRepository struct {
	conn *DBConn
}

func NewPlaybackRepository(conn *DBConn) domain.IPlaybackRepository {
	return &PlaybackRepository{conn: conn}
}

func (r *PlaybackRepository) SaveItemDuration(itemId string, totalTicks int64) error {
	if totalTicks <= 0 || itemId == "" {
		return nil
	}
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	query := `
	INSERT INTO playback_histories (user_id, item_id, total_ticks, last_played_date)
	VALUES ('admin', ?, ?, ?)
	ON CONFLICT(user_id, item_id) DO UPDATE SET
		total_ticks = CASE WHEN excluded.total_ticks > 0 THEN excluded.total_ticks ELSE playback_histories.total_ticks END
	`
	_, err := r.conn.db.Exec(query, itemId, totalTicks, time.Now())
	return err
}

func (r *PlaybackRepository) GetItemTotalTicks(itemId string) int64 {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	var ticks int64
	_ = r.conn.db.QueryRow(`
		SELECT total_ticks FROM playback_histories
		WHERE item_id = ? AND total_ticks > 0
		ORDER BY total_ticks DESC LIMIT 1
	`, itemId).Scan(&ticks)
	return ticks
}

func (r *PlaybackRepository) UpdatePlaybackProgress(userId, itemId string, posTicks, totalTicks int64) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}

	played := 0
	if totalTicks > 0 && float64(posTicks)/float64(totalTicks) >= 0.90 {
		played = 1
	}

	query := `
	INSERT INTO playback_histories (user_id, item_id, position_ticks, total_ticks, played, last_played_date)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_id, item_id) DO UPDATE SET
		position_ticks = excluded.position_ticks,
		total_ticks = CASE WHEN excluded.total_ticks > 0 THEN excluded.total_ticks ELSE playback_histories.total_ticks END,
		played = CASE WHEN excluded.played = 1 THEN 1 ELSE playback_histories.played END,
		last_played_date = excluded.last_played_date
	`
	_, err := r.conn.db.Exec(query, userId, itemId, posTicks, totalTicks, played, time.Now())
	return err
}

func (r *PlaybackRepository) GetPlaybackProgress(userId, itemId string) (int64, int64, bool) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}

	var posTicks, totalTicks int64
	var played int
	err := r.conn.db.QueryRow(`
		SELECT position_ticks, total_ticks, played
		FROM playback_histories WHERE user_id = ? AND item_id = ?
	`, userId, itemId).Scan(&posTicks, &totalTicks, &played)
	if err != nil {
		return 0, 0, false
	}
	return posTicks, totalTicks, played == 1
}

func (r *PlaybackRepository) GetProgress(userId, itemId string) (*domain.PlaybackRecord, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}

	var rec domain.PlaybackRecord
	rec.ItemId = itemId
	var playedInt int
	var lastPlayed sql.NullTime

	err := r.conn.db.QueryRow(`
		SELECT position_ticks, total_ticks, played, play_count, last_played_date
		FROM playback_histories WHERE user_id = ? AND item_id = ?
	`, userId, itemId).Scan(&rec.PositionTicks, &rec.TotalTicks, &playedInt, &rec.PlayCount, &lastPlayed)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	rec.Played = playedInt == 1
	if lastPlayed.Valid {
		rec.LastPlayedDate = lastPlayed.Time
	}
	return &rec, nil
}

func (r *PlaybackRepository) MarkPlayed(userId, itemId string, played bool) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}

	playedInt := 0
	if played {
		playedInt = 1
	}

	query := `
	INSERT INTO playback_histories (user_id, item_id, position_ticks, total_ticks, played, play_count, last_played_date)
	VALUES (?, ?, CASE WHEN ? = 1 THEN 10000 ELSE 0 END, 10000, ?, ?, ?)
	ON CONFLICT(user_id, item_id) DO UPDATE SET
		played = excluded.played,
		play_count = play_count + CASE WHEN excluded.played = 1 THEN 1 ELSE 0 END,
		position_ticks = CASE WHEN excluded.played = 1 THEN total_ticks ELSE 0 END,
		last_played_date = excluded.last_played_date
	`
	_, err := r.conn.db.Exec(query, userId, itemId, playedInt, playedInt, playedInt, time.Now())
	return err
}

func (r *PlaybackRepository) GetUserWatchingSubjectIDs(userId string, limit int) []int {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.conn.db.Query(`
		SELECT item_id FROM playback_histories
		WHERE user_id = ? AND is_favorite = 1
		ORDER BY last_played_date DESC LIMIT 100
	`, userId)
	if err != nil {
		return nil
	}
	defer rows.Close()

	seen := make(map[int]bool)
	var subIds []int

	for rows.Next() {
		var itemId string
		if err := rows.Scan(&itemId); err == nil {
			sid := 0
			if strings.HasPrefix(itemId, "bgm_sub_") {
				sid, _ = strconv.Atoi(strings.TrimPrefix(itemId, "bgm_sub_"))
			} else if strings.HasPrefix(itemId, "bgm_ep_") {
				clean := strings.TrimPrefix(itemId, "bgm_ep_")
				parts := strings.Split(clean, "_")
				if len(parts) > 0 {
					sid, _ = strconv.Atoi(parts[0])
				}
			} else if strings.HasPrefix(itemId, "bgm_season_") {
				clean := strings.TrimPrefix(itemId, "bgm_season_")
				parts := strings.Split(clean, "_")
				if len(parts) > 0 {
					sid, _ = strconv.Atoi(parts[0])
				}
			}
			if sid > 0 && !seen[sid] {
				seen[sid] = true
				subIds = append(subIds, sid)
				if len(subIds) >= limit {
					break
				}
			}
		}
	}
	return subIds
}

func (r *PlaybackRepository) GetUserResumeItems(userId string, limit int) []string {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}
	if limit <= 0 {
		limit = 20
	}

	rows, err := r.conn.db.Query(`
		SELECT item_id FROM playback_histories
		WHERE user_id = ? AND played = 0 AND position_ticks > 0
		ORDER BY last_played_date DESC LIMIT ?
	`, userId, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var itemIds []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			itemIds = append(itemIds, id)
		}
	}
	return itemIds
}

func (r *PlaybackRepository) ListRecentHistories(limit int) ([]domain.PlaybackRecord, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	rows, err := r.conn.db.Query(`
		SELECT item_id, position_ticks, total_ticks, played, play_count, last_played_date
		FROM playback_histories
		WHERE played = 1 OR position_ticks > 0
		ORDER BY last_played_date DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.PlaybackRecord
	for rows.Next() {
		var rec domain.PlaybackRecord
		var playedInt int
		var lastPlayed sql.NullTime
		if err := rows.Scan(&rec.ItemId, &rec.PositionTicks, &rec.TotalTicks, &playedInt, &rec.PlayCount, &lastPlayed); err == nil {
			rec.Played = playedInt == 1
			if lastPlayed.Valid {
				rec.LastPlayedDate = lastPlayed.Time
			}
			list = append(list, rec)
		}
	}
	return list, nil
}

func (r *PlaybackRepository) ListUserPlaybackRecords(userId string) ([]domain.PlaybackRecord, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}

	rows, err := r.conn.db.Query(`
		SELECT item_id, position_ticks, total_ticks, played, play_count, last_played_date
		FROM playback_histories 
		WHERE user_id = ? AND (played = 1 OR position_ticks > 0)
		ORDER BY last_played_date DESC
	`, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.PlaybackRecord
	for rows.Next() {
		var rec domain.PlaybackRecord
		var playedInt int
		var lastPlayed sql.NullTime

		if err := rows.Scan(&rec.ItemId, &rec.PositionTicks, &rec.TotalTicks, &playedInt, &rec.PlayCount, &lastPlayed); err == nil {
			rec.Played = playedInt == 1
			if lastPlayed.Valid {
				rec.LastPlayedDate = lastPlayed.Time
			}
			list = append(list, rec)
		}
	}
	return list, nil
}
