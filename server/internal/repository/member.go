package repository

import (
	"context"
	"smart-daily/internal/model"
	"strings"

	"gorm.io/gorm"
)

type MemberRepo struct{ db *gorm.DB }

func NewMemberRepo(db *gorm.DB) *MemberRepo { return &MemberRepo{db: db} }

// ListActive returns non-deleted members, ordered: has-team first, leader first, test accounts last.
func (r *MemberRepo) ListActive(ctx context.Context) ([]model.Member, error) {
	var members []model.Member
	err := r.db.WithContext(ctx).Scopes(model.ActiveMembers).
		Order("CASE WHEN name LIKE '测试%' THEN 2 WHEN team_id = 0 THEN 1 ELSE 0 END, team_id, CASE WHEN role = 'Leader' THEN 0 ELSE 1 END, name").
		Find(&members).Error
	return members, err
}

// FindByUsername finds an active member by username (for login).
func (r *MemberRepo) FindByUsername(ctx context.Context, username string) (*model.Member, error) {
	var m model.Member
	err := r.db.WithContext(ctx).Scopes(model.ActiveMembers).Where("username = ?", username).First(&m).Error
	return &m, err
}

// Create inserts a new member.
func (r *MemberRepo) Create(ctx context.Context, m *model.Member) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// Update updates specified fields for a member.
func (r *MemberRepo) Update(ctx context.Context, id int, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.Member{}).Where("id = ?", id).Updates(updates).Error
}

// SoftDelete marks a member as deleted.
func (r *MemberRepo) SoftDelete(ctx context.Context, id int) error {
	return r.db.WithContext(ctx).Model(&model.Member{}).Where("id = ?", id).Update("status", "deleted").Error
}

// ListTeams returns all teams ordered by name.
func (r *MemberRepo) ListTeams(ctx context.Context) ([]model.Team, error) {
	var teams []model.Team
	err := r.db.WithContext(ctx).Order("name").Find(&teams).Error
	return teams, err
}

// CreateTeam inserts a new team.
func (r *MemberRepo) CreateTeam(ctx context.Context, t *model.Team) error {
	return r.db.WithContext(ctx).Create(t).Error
}

// TeamMap returns a map of team ID → team name.
func (r *MemberRepo) TeamMap(ctx context.Context) (map[int]string, error) {
	teams, err := r.ListTeams(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[int]string, len(teams))
	for _, t := range teams {
		m[t.ID] = t.Name
	}
	return m, nil
}

// MatchByName finds a member ID by name. Exact match first, then substring match. Returns 0 if not found.
func MatchByName(name string, members []model.Member) int {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0
	}
	normalized := strings.ReplaceAll(name, " ", "")
	for _, m := range members {
		if strings.ReplaceAll(m.Name, " ", "") == normalized {
			return m.ID
		}
	}
	for _, m := range members {
		if strings.Contains(m.Name, name) || strings.Contains(name, m.Name) {
			return m.ID
		}
	}
	return 0
}
