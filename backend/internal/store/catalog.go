package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"xiweispace/backend/internal/models"
)

func (s *Store) GetArtist(ctx context.Context) (models.Artist, error) {
	var a models.Artist
	var profile []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT id, name, name_en, avatar, cover, bio, profile_data
		FROM artists ORDER BY created_at LIMIT 1`,
	).Scan(&a.ID, &a.Name, &a.NameEN, &a.Avatar, &a.Cover, &a.Bio, &profile)
	if err != nil {
		return a, mapNoRows(err)
	}
	a.ProfileData = map[string]any{}
	_ = json.Unmarshal(profile, &a.ProfileData)
	return a, nil
}

type ArtistInput struct {
	Name        string         `json:"name"`
	NameEN      string         `json:"name_en"`
	Avatar      string         `json:"avatar"`
	Cover       string         `json:"cover"`
	Bio         string         `json:"bio"`
	ProfileData map[string]any `json:"profile_data"`
}

func (s *Store) UpdateArtist(ctx context.Context, id string, in ArtistInput) (models.Artist, error) {
	if strings.TrimSpace(in.Name) == "" {
		return models.Artist{}, fmt.Errorf("name is required")
	}
	if in.ProfileData == nil {
		in.ProfileData = map[string]any{}
	}
	profile, _ := json.Marshal(in.ProfileData)
	var a models.Artist
	var raw []byte
	err := s.Pool.QueryRow(ctx, `
		UPDATE artists SET name=$2, name_en=$3, avatar=$4, cover=$5, bio=$6, profile_data=$7, updated_at=now()
		WHERE id=$1::uuid
		RETURNING id, name, name_en, avatar, cover, bio, profile_data`,
		id, in.Name, in.NameEN, in.Avatar, in.Cover, in.Bio, profile,
	).Scan(&a.ID, &a.Name, &a.NameEN, &a.Avatar, &a.Cover, &a.Bio, &raw)
	if err != nil {
		return a, mapNoRows(err)
	}
	a.ProfileData = map[string]any{}
	_ = json.Unmarshal(raw, &a.ProfileData)
	return a, nil
}

const sceneColumns = `s.id, s.code, s.name, s.description, s.icon, s.sort_order, s.enabled,
	COALESCE(p.weight, 0), COALESCE(p.enabled, false), COALESCE(p.is_default, false)`

func scanScenes(rows pgx.Rows) ([]models.Scene, error) {
	defer rows.Close()
	out := make([]models.Scene, 0)
	for rows.Next() {
		var sc models.Scene
		if err := rows.Scan(&sc.ID, &sc.Code, &sc.Name, &sc.Description, &sc.Icon, &sc.SortOrder, &sc.Enabled, &sc.Weight, &sc.Selected, &sc.IsDefault); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (s *Store) ListScenes(ctx context.Context, userID string) ([]models.Scene, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+sceneColumns+`
		FROM scenes s
		LEFT JOIN user_scene_preferences p ON p.scene_id = s.id AND p.user_id = NULLIF($1,'')::uuid
		WHERE s.enabled = true
		ORDER BY s.sort_order, s.name`, userID)
	if err != nil {
		return nil, err
	}
	return scanScenes(rows)
}

func (s *Store) ListAllScenes(ctx context.Context) ([]models.Scene, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, code, name, description, icon, sort_order, enabled, 0, enabled, false
		FROM scenes ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Scene, 0)
	for rows.Next() {
		var sc models.Scene
		if err := rows.Scan(&sc.ID, &sc.Code, &sc.Name, &sc.Description, &sc.Icon, &sc.SortOrder, &sc.Enabled, &sc.Weight, &sc.Selected, &sc.IsDefault); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (s *Store) UpdateScene(ctx context.Context, id string, name, description *string, enabled *bool) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE scenes SET
			name = COALESCE($2, name),
			description = COALESCE($3, description),
			enabled = COALESCE($4, enabled)
		WHERE id=$1::uuid`, id, name, description, enabled)
	return err
}

type ScenePrefInput struct {
	SceneID   string `json:"scene_id"`
	Weight    int    `json:"weight"`
	Enabled   bool   `json:"enabled"`
	IsDefault bool   `json:"is_default"`
}

func (s *Store) SetScenePreferences(ctx context.Context, userID string, prefs []ScenePrefInput) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM user_scene_preferences WHERE user_id=$1::uuid`, userID); err != nil {
			return err
		}
		defaultSeen := false
		for _, p := range prefs {
			if p.Weight < 1 || p.Weight > 5 {
				p.Weight = 3
			}
			isDefault := p.IsDefault && p.Enabled && !defaultSeen
			if isDefault {
				defaultSeen = true
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO user_scene_preferences (user_id, scene_id, weight, enabled, is_default)
				SELECT $1::uuid, s.id, $3, $4, $5 FROM scenes s WHERE s.id=$2::uuid AND s.enabled=true
				ON CONFLICT (user_id, scene_id) DO UPDATE SET weight=EXCLUDED.weight, enabled=EXCLUDED.enabled, is_default=EXCLUDED.is_default`,
				userID, p.SceneID, p.Weight, p.Enabled, isDefault); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) SceneWeights(ctx context.Context, userID string) (map[string]int, error) {
	out := map[string]int{}
	if userID == "" {
		return out, nil
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT sc.code, p.weight FROM user_scene_preferences p
		JOIN scenes sc ON sc.id=p.scene_id
		WHERE p.user_id=$1::uuid AND p.enabled=true`, userID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		var w int
		if err := rows.Scan(&code, &w); err != nil {
			return out, err
		}
		out[code] = w
	}
	return out, rows.Err()
}

const workSelect = `
	SELECT w.id, w.artist_id, w.title, w.type, w.cover, w.description,
	       COALESCE(to_char(w.release_date, 'YYYY-MM-DD'), ''), w.status, w.platform, w.official_url,
	       COALESCE((
	         SELECT json_agg(json_build_object(
	           'id', ch.id, 'work_id', ch.work_id, 'name', ch.name,
	           'avatar', ch.avatar, 'description', ch.description
	         ) ORDER BY ch.created_at)
	         FROM characters ch WHERE ch.work_id = w.id
	       ), '[]'::json)
	FROM works w`

func scanWork(row pgx.Row) (models.Work, error) {
	var w models.Work
	var chars []byte
	err := row.Scan(&w.ID, &w.ArtistID, &w.Title, &w.Type, &w.Cover, &w.Description, &w.ReleaseDate, &w.Status, &w.Platform, &w.OfficialURL, &chars)
	if err != nil {
		return w, mapNoRows(err)
	}
	w.Characters = []models.Character{}
	_ = json.Unmarshal(chars, &w.Characters)
	return w, nil
}

func (s *Store) ListWorks(ctx context.Context, artistID, typ string) ([]models.Work, error) {
	rows, err := s.Pool.Query(ctx, workSelect+`
		WHERE w.artist_id=$1::uuid AND ($2='' OR w.type=$2)
		ORDER BY w.release_date DESC NULLS LAST, w.title`, artistID, typ)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Work, 0)
	for rows.Next() {
		w, err := scanWork(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) GetWork(ctx context.Context, id string) (models.Work, error) {
	return scanWork(s.Pool.QueryRow(ctx, workSelect+` WHERE w.id=$1::uuid`, id))
}

type WorkInput struct {
	Title       string `json:"title"`
	Type        string `json:"type"`
	Cover       string `json:"cover"`
	Description string `json:"description"`
	ReleaseDate string `json:"release_date"`
	Status      string `json:"status"`
	Platform    string `json:"platform"`
	OfficialURL string `json:"official_url"`
}

func normalizeWorkInput(in WorkInput) WorkInput {
	if in.Type == "" {
		in.Type = "drama"
	}
	if in.Status == "" {
		in.Status = "aired"
	}
	return in
}

func (s *Store) CreateWork(ctx context.Context, artistID string, in WorkInput) (models.Work, error) {
	in = normalizeWorkInput(in)
	if strings.TrimSpace(in.Title) == "" {
		return models.Work{}, fmt.Errorf("title is required")
	}
	var id string
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO works (artist_id, title, type, cover, description, release_date, status, platform, official_url)
		VALUES ($1::uuid, $2, $3, $4, $5, NULLIF($6,'')::date, $7, $8, $9)
		RETURNING id`,
		artistID, in.Title, in.Type, in.Cover, in.Description, in.ReleaseDate, in.Status, in.Platform, in.OfficialURL,
	).Scan(&id)
	if err != nil {
		return models.Work{}, err
	}
	return s.GetWork(ctx, id)
}

func (s *Store) UpdateWork(ctx context.Context, id string, in WorkInput) (models.Work, error) {
	in = normalizeWorkInput(in)
	_, err := s.Pool.Exec(ctx, `
		UPDATE works SET title=$2, type=$3, cover=$4, description=$5,
			release_date=NULLIF($6,'')::date, status=$7, platform=$8, official_url=$9, updated_at=now()
		WHERE id=$1::uuid`,
		id, in.Title, in.Type, in.Cover, in.Description, in.ReleaseDate, in.Status, in.Platform, in.OfficialURL)
	if err != nil {
		return models.Work{}, err
	}
	return s.GetWork(ctx, id)
}

func (s *Store) DeleteWork(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM works WHERE id=$1::uuid`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListCharacters(ctx context.Context, workID string) ([]models.Character, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, work_id, name, avatar, description FROM characters
		WHERE ($1='' OR work_id=$1::uuid) ORDER BY created_at`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Character, 0)
	for rows.Next() {
		var c models.Character
		if err := rows.Scan(&c.ID, &c.WorkID, &c.Name, &c.Avatar, &c.Description); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetCharacter(ctx context.Context, id string) (models.Character, error) {
	var c models.Character
	var w models.WorkRef
	err := s.Pool.QueryRow(ctx, `
		SELECT ch.id, ch.work_id, ch.name, ch.avatar, ch.description,
		       w.title, w.type, COALESCE(to_char(w.release_date,'YYYY-MM-DD'),''), w.cover
		FROM characters ch JOIN works w ON w.id=ch.work_id WHERE ch.id=$1::uuid`, id,
	).Scan(&c.ID, &c.WorkID, &c.Name, &c.Avatar, &c.Description, &w.Title, &w.Type, &w.ReleaseDate, &w.Cover)
	if err != nil {
		return c, mapNoRows(err)
	}
	w.ID = c.WorkID
	c.Work = &w
	return c, nil
}

type CharacterInput struct {
	WorkID      string `json:"work_id"`
	Name        string `json:"name"`
	Avatar      string `json:"avatar"`
	Description string `json:"description"`
}

func (s *Store) CreateCharacter(ctx context.Context, in CharacterInput) (models.Character, error) {
	if strings.TrimSpace(in.Name) == "" || in.WorkID == "" {
		return models.Character{}, fmt.Errorf("work_id and name are required")
	}
	var id string
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO characters (work_id, name, avatar, description)
		VALUES ($1::uuid, $2, $3, $4) RETURNING id`,
		in.WorkID, in.Name, in.Avatar, in.Description).Scan(&id)
	if err != nil {
		return models.Character{}, err
	}
	return s.GetCharacter(ctx, id)
}

func (s *Store) UpdateCharacter(ctx context.Context, id string, in CharacterInput) (models.Character, error) {
	_, err := s.Pool.Exec(ctx, `
		UPDATE characters SET work_id=$2::uuid, name=$3, avatar=$4, description=$5 WHERE id=$1::uuid`,
		id, in.WorkID, in.Name, in.Avatar, in.Description)
	if err != nil {
		return models.Character{}, err
	}
	return s.GetCharacter(ctx, id)
}

func (s *Store) DeleteCharacter(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM characters WHERE id=$1::uuid`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
