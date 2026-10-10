package repository

import (
	"fmt"
	"time"

	"akari-bridge/internal/domain"
)

type FavoriteRepository struct {
	conn *DBConn
}

func NewFavoriteRepository(conn *DBConn) domain.IFavoriteRepository {
	return &FavoriteRepository{conn: conn}
}

func (r *FavoriteRepository) ToggleFavoriteItem(userId, itemId string, isFav bool) error {
	return r.MarkFavorite(userId, itemId, isFav)
}

func (r *FavoriteRepository) MarkFavorite(userId, itemId string, isFavorite bool) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}

	favInt := 0
	if isFavorite {
		favInt = 1
	}

	query := `
	INSERT INTO playback_histories (user_id, item_id, is_favorite, last_played_date)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(user_id, item_id) DO UPDATE SET
		is_favorite = excluded.is_favorite,
		last_played_date = excluded.last_played_date
	`
	_, err := r.conn.db.Exec(query, userId, itemId, favInt, time.Now())
	return err
}

func (r *FavoriteRepository) IsFavoriteItem(userId, itemId string) bool {
	isFav, _ := r.IsFavorite(userId, itemId)
	return isFav
}

func (r *FavoriteRepository) IsFavorite(userId, itemId string) (bool, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}

	var isFav int
	err := r.conn.db.QueryRow(`
		SELECT is_favorite FROM playback_histories WHERE user_id = ? AND item_id = ?
	`, userId, itemId).Scan(&isFav)
	if err != nil {
		return false, nil
	}
	return isFav == 1, nil
}

func (r *FavoriteRepository) UnmarkAllSubjectFavorites(userId string, subjectId int) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}

	subPrefix1 := fmt.Sprintf("bgm_sub_%d", subjectId)
	subPrefix2 := fmt.Sprintf("bgm_season_%d_%%", subjectId)
	subPrefix3 := fmt.Sprintf("bgm_ep_%d_%%", subjectId)

	_, err := r.conn.db.Exec(`
		UPDATE playback_histories SET is_favorite = 0
		WHERE user_id = ? AND (item_id = ? OR item_id LIKE ? OR item_id LIKE ?)
	`, userId, subPrefix1, subPrefix2, subPrefix3)
	return err
}

func (r *FavoriteRepository) ListFavoriteItems(userId string) ([]string, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	if userId == "00000000000000000000000000000001" || userId == "" || userId == "default" {
		userId = "admin"
	}

	rows, err := r.conn.db.Query(`
		SELECT item_id FROM playback_histories
		WHERE user_id = ? AND is_favorite = 1
		ORDER BY last_played_date DESC
	`, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			list = append(list, id)
		}
	}
	return list, nil
}
