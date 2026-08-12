package response

import "time"

// LivestreamStreamer is the nested streamer sub-object within LivestreamResponse
// (mirrors the VideoCreator / CommentUser convention).
type LivestreamStreamer struct {
	ID     string `json:"id"     example:"usr_12345"`
	Name   string `json:"name"   example:"quanh_dep_trai"`
	Avatar string `json:"avatar" example:"http://localhost:9000/raw-videos/avatars/user.jpg"`
}

type LivestreamResponse struct {
	ID           string              `json:"id" example:"ls_550e8400"`
	Streamer     LivestreamStreamer  `json:"streamer"`
	Title        string              `json:"title" example:"Exploring the edge of the universe"`
	Category     string              `json:"category" example:"Science"`
	IsLive       bool                `json:"is_live" example:"true"`
	HLSUrl       string              `json:"hls_url" example:"https://s3.pipevid.com/live/streamkey/index.m3u8"`
	ViewersCount int64               `json:"viewers_count" example:"24812"`
	StartedAt    *time.Time          `json:"started_at,omitempty"`
	EndedAt      *time.Time          `json:"ended_at,omitempty"`
}

type StreamKeyResponse struct {
	ServerUrl string `json:"server_url" example:"rtmp://live.pipevid.com/live"`
	StreamKey string `json:"stream_key" example:"sk_live_8h2k_92md_71px"`
}

// ChatUser is the nested sender sub-object within ChatMessageResponse.
type ChatUser struct {
	Name   string `json:"name"   example:"julesm"`
	Avatar string `json:"avatar" example:"JM"`
}

type ChatMessageResponse struct {
	StreamID  string   `json:"stream_id" example:"ls_550e8400"`
	User      ChatUser `json:"user"`
	Text      string   `json:"text" example:"This stream is amazing!"`
	CreatedAt string   `json:"created_at" example:"14:32:05"`
}
