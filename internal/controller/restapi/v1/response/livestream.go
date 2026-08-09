package response

import "time"

type LivestreamResponse struct {
	ID           string     `json:"id" example:"ls_550e8400"`
	UserID       string     `json:"user_id" example:"usr_12345"`
	Title        string     `json:"title" example:"Exploring the edge of the universe"`
	Category     string     `json:"category" example:"Science"`
	IsLive       bool       `json:"is_live" example:"true"`
	HLSUrl       string     `json:"hls_url" example:"https://s3.pipevid.com/live/streamkey/index.m3u8"`
	ViewersCount int64      `json:"viewers_count" example:"24812"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
}

type StreamKeyResponse struct {
	ServerUrl string `json:"server_url" example:"rtmp://live.pipevid.com/live"`
	StreamKey string `json:"stream_key" example:"sk_live_8h2k_92md_71px"`
}

type ChatMessageResponse struct {
	StreamID  string `json:"stream_id" example:"ls_550e8400"`
	Username  string `json:"username" example:"julesm"`
	Avatar    string `json:"avatar" example:"JM"`
	Text      string `json:"text" example:"This stream is amazing!"`
	CreatedAt string `json:"created_at" example:"14:32:05"`
}
