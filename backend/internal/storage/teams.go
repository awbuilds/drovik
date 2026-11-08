package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Team struct {
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type TeamSummary struct {
	Team
	MemberCount int
	ServerCount int
}

type TeamMember struct {
	UserID    string
	Username  string
	Role      string
	CreatedAt time.Time
}

type TeamServer struct {
	ServerID    string
	DisplayName string
	Host        string
	AccessLevel string
	CreatedAt   time.Time
}

type TeamRepository struct {
	db *sql.DB
}

func NewTeamRepository(db *sql.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

func (r *TeamRepository) Create(ctx context.Context, team Team) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO teams (id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		team.ID, team.Name, team.Description, formatTime(team.CreatedAt), formatTime(team.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("create team: %w", err)
	}
	return nil
}

func (r *TeamRepository) Get(ctx context.Context, id string) (*Team, error) {
	var team Team
	var createdAt, updatedAt string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, created_at, updated_at FROM teams WHERE id = ?`, id,
	).Scan(&team.ID, &team.Name, &team.Description, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get team: %w", err)
	}
	team.CreatedAt = parseTime(createdAt)
	team.UpdatedAt = parseTime(updatedAt)
	return &team, nil
}

func (r *TeamRepository) List(ctx context.Context) ([]TeamSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.id, t.name, t.description, t.created_at, t.updated_at,
		       (SELECT COUNT(*) FROM team_members m WHERE m.team_id = t.id),
		       (SELECT COUNT(*) FROM team_servers s WHERE s.team_id = t.id)
		FROM teams t ORDER BY t.name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	defer func() { _ = rows.Close() }()
	teams := make([]TeamSummary, 0, 8)
	for rows.Next() {
		var team TeamSummary
		var createdAt, updatedAt string
		if err := rows.Scan(&team.ID, &team.Name, &team.Description, &createdAt, &updatedAt,
			&team.MemberCount, &team.ServerCount); err != nil {
			return nil, fmt.Errorf("scan team: %w", err)
		}
		team.CreatedAt = parseTime(createdAt)
		team.UpdatedAt = parseTime(updatedAt)
		teams = append(teams, team)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate teams: %w", err)
	}
	return teams, nil
}

func (r *TeamRepository) Update(ctx context.Context, id, name, description string, at time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE teams SET name = ?, description = ?, updated_at = ? WHERE id = ?`,
		name, description, formatTime(at), id)
	if err != nil {
		return fmt.Errorf("update team: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated teams: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TeamRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM teams WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete team: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted teams: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TeamRepository) AddMember(ctx context.Context, teamID, userID string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO team_members (team_id, user_id, created_at) VALUES (?, ?, ?)
		ON CONFLICT(team_id, user_id) DO NOTHING`, teamID, userID, formatTime(at))
	if err != nil {
		return fmt.Errorf("add team member: %w", err)
	}
	return nil
}

func (r *TeamRepository) RemoveMember(ctx context.Context, teamID, userID string) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM team_members WHERE team_id = ? AND user_id = ?`, teamID, userID)
	if err != nil {
		return fmt.Errorf("remove team member: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count removed team members: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TeamRepository) ListMembers(ctx context.Context, teamID string) ([]TeamMember, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, u.username, u.role, m.created_at
		FROM team_members m JOIN users u ON u.id = m.user_id
		WHERE m.team_id = ? ORDER BY u.username COLLATE NOCASE`, teamID)
	if err != nil {
		return nil, fmt.Errorf("list team members: %w", err)
	}
	defer func() { _ = rows.Close() }()
	members := make([]TeamMember, 0, 8)
	for rows.Next() {
		var member TeamMember
		var createdAt string
		if err := rows.Scan(&member.UserID, &member.Username, &member.Role, &createdAt); err != nil {
			return nil, fmt.Errorf("scan team member: %w", err)
		}
		member.CreatedAt = parseTime(createdAt)
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team members: %w", err)
	}
	return members, nil
}

func (r *TeamRepository) SetServer(ctx context.Context, teamID, serverID, accessLevel string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO team_servers (team_id, server_id, access_level, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(team_id, server_id) DO UPDATE SET access_level = excluded.access_level`,
		teamID, serverID, accessLevel, formatTime(at))
	if err != nil {
		return fmt.Errorf("set team server: %w", err)
	}
	return nil
}

func (r *TeamRepository) RemoveServer(ctx context.Context, teamID, serverID string) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM team_servers WHERE team_id = ? AND server_id = ?`, teamID, serverID)
	if err != nil {
		return fmt.Errorf("remove team server: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count removed team servers: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TeamRepository) ListServers(ctx context.Context, teamID string) ([]TeamServer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.display_name, s.host, ts.access_level, ts.created_at
		FROM team_servers ts JOIN servers s ON s.id = ts.server_id
		WHERE ts.team_id = ? ORDER BY s.display_name COLLATE NOCASE`, teamID)
	if err != nil {
		return nil, fmt.Errorf("list team servers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	servers := make([]TeamServer, 0, 8)
	for rows.Next() {
		var server TeamServer
		var createdAt string
		if err := rows.Scan(&server.ServerID, &server.DisplayName, &server.Host, &server.AccessLevel, &createdAt); err != nil {
			return nil, fmt.Errorf("scan team server: %w", err)
		}
		server.CreatedAt = parseTime(createdAt)
		servers = append(servers, server)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team servers: %w", err)
	}
	return servers, nil
}

// AccessLevel returns the strongest team-derived level for a user and server.
func (r *TeamRepository) AccessLevel(ctx context.Context, userID, serverID string) (string, bool, error) {
	var access string
	err := r.db.QueryRowContext(ctx, `
		SELECT ts.access_level
		FROM team_servers ts JOIN team_members tm ON tm.team_id = ts.team_id
		WHERE tm.user_id = ? AND ts.server_id = ?
		ORDER BY CASE ts.access_level WHEN 'write' THEN 2 ELSE 1 END DESC
		LIMIT 1`, userID, serverID).Scan(&access)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("team access level: %w", err)
	}
	return access, true, nil
}

func (r *TeamRepository) IsMember(ctx context.Context, teamID, userID string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM team_members WHERE team_id = ? AND user_id = ?`, teamID, userID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check team membership: %w", err)
	}
	return true, nil
}
