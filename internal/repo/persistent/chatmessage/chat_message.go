package chatmessage

import (
	"context"
	"fmt"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/postgres"
)

type Repo struct {
	*postgres.Postgres
}

func New(pg *postgres.Postgres) repo.ChatMessageRepo {
	return &Repo{pg}
}

func (r *Repo) Store(ctx context.Context, msg *entity.LiveChatMessage) error {
	sql, args, err := r.Builder.
		Insert("live_chat_messages").
		Columns("session_id", "user_id", "username", "avatar", "text", "created_at").
		Values(msg.SessionID, msg.UserID, msg.Username, msg.Avatar, msg.Text, msg.CreatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("ChatMessageRepo - Store - Builder: %w", err)
	}

	if _, err := r.Pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("ChatMessageRepo - Store - Exec: %w", err)
	}
	return nil
}

func (r *Repo) ListBySessionID(ctx context.Context, sessionID string, limit, offset int) ([]entity.LiveChatMessage, int, error) {
	countSQL, countArgs, err := r.Builder.
		Select("COUNT(*)").
		From("live_chat_messages").
		Where("session_id = ?", sessionID).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("ChatMessageRepo - ListBySessionID - countBuilder: %w", err)
	}

	var total int
	if err := r.Pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ChatMessageRepo - ListBySessionID - count: %w", err)
	}

	dataSQL, dataArgs, err := r.Builder.
		Select("id", "session_id", "username", "avatar", "text", "created_at").
		From("live_chat_messages").
		Where("session_id = ?", sessionID).
		OrderBy("created_at ASC").
		Limit(uint64(limit)).
		Offset(uint64(offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("ChatMessageRepo - ListBySessionID - dataBuilder: %w", err)
	}

	rows, err := r.Pool.Query(ctx, dataSQL, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("ChatMessageRepo - ListBySessionID - Query: %w", err)
	}
	defer rows.Close()

	messages := make([]entity.LiveChatMessage, 0, limit)
	for rows.Next() {
		var m entity.LiveChatMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Username, &m.Avatar, &m.Text, &m.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("ChatMessageRepo - ListBySessionID - Scan: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, total, nil
}
