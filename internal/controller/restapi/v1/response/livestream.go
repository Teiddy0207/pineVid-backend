package response

import "time"

// LiveSessionResponse represents a single broadcast event (one run of is_live=true).
// The parent LivestreamResponse describes the channel; this describes the session.
type LiveSessionResponse struct {
	ID            string     `json:"id" example:"sess_550e8400"`
	LivestreamID  string     `json:"livestream_id" example:"ls_550e8400"`
	Title         string     `json:"title" example:"Weekend coding stream"`
	ThumbnailURL  string     `json:"thumbnail_url" example:"https://media.pipevid.com/images/cover.jpg"`
	Category      string     `json:"category" example:"Technology"`
	StartedAt     time.Time  `json:"started_at"`
	EndedAt       *time.Time `json:"ended_at,omitempty"`
	ReplayVideoID string     `json:"replay_video_id,omitempty" example:"vid_abc123"`
	// DurationSecs is computed from StartedAt..EndedAt when EndedAt is set.
	DurationSecs *int64 `json:"duration_secs,omitempty"`
	// Totals aggregated by the live-agg worker from the live events stream
	// (a few seconds behind realtime while the session is live).
	Hearts         int64 `json:"hearts" example:"1520"`
	ChatMessages   int64 `json:"chat_messages" example:"342"`
	UniqueChatters int   `json:"unique_chatters" example:"57"`
}

// LivestreamStreamer is the nested streamer sub-object within LivestreamResponse
// (mirrors the VideoCreator / CommentUser convention).
type LivestreamStreamer struct {
	ID             string `json:"id"     example:"usr_12345"`
	Name           string `json:"name"   example:"quanh_dep_trai"`
	Avatar         string `json:"avatar" example:"http://localhost:9000/raw-videos/avatars/user.jpg"`
	FollowersCount int64  `json:"followers_count" example:"1420"`
}

type LivestreamResponse struct {
	ID           string             `json:"id" example:"ls_550e8400"`
	Streamer     LivestreamStreamer `json:"streamer"`
	Title        string             `json:"title" example:"Exploring the edge of the universe"`
	Category     string             `json:"category" example:"Science"`
	ThumbnailURL string             `json:"thumbnail_url" example:"https://media.pipevid.com/public-assets/images/cover.jpg"`
	IsLive       bool               `json:"is_live" example:"true"`
	HLSUrl       string             `json:"hls_url" example:"https://s3.pipevid.com/live/streamkey/index.m3u8"`
	ViewersCount int64              `json:"viewers_count" example:"24812"`
	StartedAt    *time.Time         `json:"started_at,omitempty"`
	EndedAt      *time.Time         `json:"ended_at,omitempty"`
}

type StreamKeyResponse struct {
	// LivestreamID is the channel ID used by the public room (/live/{id}),
	// chat, hearts and session history — it is not the user ID.
	LivestreamID string     `json:"livestream_id" example:"ls_550e8400"`
	ServerUrl    string     `json:"server_url" example:"rtmp://live.pipevid.com/live"`
	StreamKey    string     `json:"stream_key" example:"sk_live_8h2k_92md_71px"`
	IsLive       bool       `json:"is_live" example:"false"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	ViewersCount int64      `json:"viewers_count" example:"0"`
}

// ChatUser is the nested sender sub-object within ChatMessageResponse.
type ChatUser struct {
	Name   string `json:"name"   example:"julesm"`
	Avatar string `json:"avatar" example:"JM"`
}

type ChatMessageResponse struct {
	StreamID string `json:"stream_id" example:"ls_550e8400"`
	// Type discriminates the SSE event: "chat" (default) or "heart". Frontend
	// listeners should branch on this instead of guessing from Text content.
	Type string   `json:"type,omitempty" example:"chat"`
	User ChatUser `json:"user"`
	Text string   `json:"text" example:"This stream is amazing!"`
	// Value carries the room's new running heart total when Type=="heart".
	Value     int64  `json:"value,omitempty" example:"128"`
	CreatedAt string `json:"created_at" example:"14:32:05"`
}

type LiveViewersResponse struct {
	StreamID     string `json:"stream_id"`
	Broadcast    string `json:"broadcast"`
	ViewersCount int64  `json:"viewers_count"`
}
