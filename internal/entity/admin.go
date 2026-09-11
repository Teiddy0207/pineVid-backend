package entity

// ChannelStats aggregates channel-level counts for the Admin Dashboard — a
// "channel" is any user who has published at least one video or gone live
// at least once.
type ChannelStats struct {
	TotalChannels  int64
	ActiveChannels int64
	BannedChannels int64
}

// CategoryStat is the video count for a single category, used to render the
// Admin Dashboard's category breakdown.
type CategoryStat struct {
	Category   string
	VideoCount int64
}
