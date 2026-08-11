package recommendation

import (
	"context"
	"fmt"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/pkg/postgres"
)

type Repo struct {
	*postgres.Postgres
}

func New(pg *postgres.Postgres) *Repo {
	return &Repo{Postgres: pg}
}

// FetchInteractions builds the user-video interaction dataset from likes, comments, and views
func (r *Repo) FetchInteractions(ctx context.Context) ([]entity.UserVideoInteraction, error) {
	sql := `
		SELECT user_id, video_id, SUM(rating) as total_rating FROM (
			SELECT user_id, video_id, 2.0 as rating FROM likes
			UNION ALL
			SELECT user_id, video_id, 3.0 as rating FROM comments
		) interactions
		GROUP BY user_id, video_id;
	`

	rows, err := r.Pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("RecommendationRepo - FetchInteractions: %w", err)
	}
	defer rows.Close()

	interactions := make([]entity.UserVideoInteraction, 0)
	for rows.Next() {
		var inter entity.UserVideoInteraction
		if err := rows.Scan(&inter.UserID, &inter.VideoID, &inter.Rating); err != nil {
			return nil, fmt.Errorf("RecommendationRepo - FetchInteractions Scan: %w", err)
		}
		interactions = append(interactions, inter)
	}

	return interactions, nil
}
