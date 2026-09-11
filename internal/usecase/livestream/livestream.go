package livestream

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/events"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/usecase"
	pkgminio "github.com/evrone/go-clean-template/pkg/minio"
	"github.com/evrone/go-clean-template/pkg/nats"
	"github.com/google/uuid"
)

// pendingEnd tracks a stream that unpublished but hasn't been finalized yet —
// it may still reconnect within the reconnection grace window. dvrPath is
// filled in if SRS's on_dvr webhook arrives while we're still waiting (which
// it normally does, since SRS closes the DVR session right after unpublish,
// well before our grace period elapses).
type pendingEnd struct {
	timer   *time.Timer
	dvrPath string
}

type UseCase struct {
	repo          repo.LivestreamRepo
	videoRepo     repo.VideoRepo
	followRepo    repo.FollowRepo
	chatHub       *events.ChatHub
	notifUc       usecase.Notification
	minioClient   *pkgminio.Client
	natsPublisher *nats.Publisher
	rawBucket     string
	dvrLocalDir   string
	graceDuration time.Duration

	mu      sync.Mutex
	pending map[string]*pendingEnd
}

func New(
	r repo.LivestreamRepo, vr repo.VideoRepo, followRepo repo.FollowRepo, chatHub *events.ChatHub, notifUc usecase.Notification,
	minioClient *pkgminio.Client, natsPublisher *nats.Publisher, rawBucket, dvrLocalDir string,
	graceDuration time.Duration,
) *UseCase {
	return &UseCase{
		repo: r, videoRepo: vr, followRepo: followRepo, chatHub: chatHub, notifUc: notifUc,
		minioClient: minioClient, natsPublisher: natsPublisher,
		rawBucket: rawBucket, dvrLocalDir: dvrLocalDir,
		graceDuration: graceDuration,
		pending:       make(map[string]*pendingEnd),
	}
}

// withFollowersCount enriches an already-mapped LivestreamResponse with the
// streamer's real follower count. Best-effort: a lookup failure just leaves
// the count at its zero value rather than failing the whole response.
func (u *UseCase) withFollowersCount(ctx context.Context, res response.LivestreamResponse) response.LivestreamResponse {
	if u.followRepo == nil {
		return res
	}
	if count, err := u.followRepo.CountFollowers(ctx, res.Streamer.ID); err == nil {
		res.Streamer.FollowersCount = count
	}
	return res
}

func (u *UseCase) GetStreamKey(ctx context.Context, userID string) (response.StreamKeyResponse, error) {
	ls, err := u.repo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, entity.ErrLivestreamNotFound) {
			// Auto create livestream record for user if not exists
			newStreamKey := fmt.Sprintf("sk_live_%s", uuid.New().String()[:18])
			newLs := entity.Livestream{
				ID:        uuid.New().String(),
				UserID:    userID,
				StreamKey: newStreamKey,
				Title:     "My Livestream",
				Category:  "General",
				IsLive:    false,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}
			if err := u.repo.Store(ctx, &newLs); err != nil {
				return response.StreamKeyResponse{}, fmt.Errorf("LivestreamUseCase - GetStreamKey - Store: %w", err)
			}
			ls = newLs
		} else {
			return response.StreamKeyResponse{}, err
		}
	}

	return response.StreamKeyResponse{
		ServerUrl:    "rtmp://live.pipevid.com/live",
		StreamKey:    ls.StreamKey,
		IsLive:       ls.IsLive,
		StartedAt:    ls.StartedAt,
		ViewersCount: ls.ViewersCount,
	}, nil
}

func (u *UseCase) ResetStreamKey(ctx context.Context, userID string) (response.StreamKeyResponse, error) {
	ls, err := u.repo.GetByUserID(ctx, userID)
	if err != nil {
		return response.StreamKeyResponse{}, err
	}

	ls.StreamKey = fmt.Sprintf("sk_live_%s", uuid.New().String()[:18])
	ls.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, &ls); err != nil {
		return response.StreamKeyResponse{}, fmt.Errorf("LivestreamUseCase - ResetStreamKey - Update: %w", err)
	}

	return response.StreamKeyResponse{
		ServerUrl:    "rtmp://live.pipevid.com/live",
		StreamKey:    ls.StreamKey,
		IsLive:       ls.IsLive,
		StartedAt:    ls.StartedAt,
		ViewersCount: ls.ViewersCount,
	}, nil
}

func (u *UseCase) AuthenticateStreamKey(ctx context.Context, req request.StreamKeyAuth) (bool, error) {
	// A reconnect within the grace window: the stream was never actually
	// marked offline (see UnpublishStream), so there's nothing to restore —
	// just cancel the pending finalize and let the publish through.
	u.mu.Lock()
	if p, ok := u.pending[req.StreamKey]; ok {
		p.timer.Stop()
		delete(u.pending, req.StreamKey)
		u.mu.Unlock()
		return true, nil
	}
	u.mu.Unlock()

	ls, err := u.repo.GetByStreamKey(ctx, req.StreamKey)
	if err != nil {
		return false, entity.ErrInvalidStreamKey
	}

	// Mark stream as live. HLSUrl is a relative path served by SRS itself
	// (vhost hls_m3u8_file = [app]/[stream].m3u8) — the frontend prefixes
	// "/live/..." paths with the SRS host, same convention as VOD's
	// "/hls-streams/..." being prefixed with the MinIO host.
	now := time.Now().UTC()
	ls.IsLive = true
	ls.StartedAt = &now
	ls.EndedAt = nil
	ls.HLSUrl = fmt.Sprintf("/live/%s.m3u8", ls.StreamKey)
	ls.UpdatedAt = now

	_ = u.repo.Update(ctx, &ls)

	// Notify followers that this streamer just went live. Best-effort and
	// async: SRS is waiting on this HTTP response to admit the RTMP publish,
	// so this must never slow down or fail the auth decision.
	if u.notifUc != nil {
		go func() {
			notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = u.notifUc.NotifyFollowers(
				notifyCtx,
				ls.UserID, ls.UserName, ls.UserAvatar,
				entity.NotificationTypeLiveStart,
				fmt.Sprintf("%s is live now", ls.UserName),
				ls.Title,
				fmt.Sprintf("/live/%s", ls.ID),
			)
		}()
	}

	return true, nil
}

// UnpublishStream is called via SRS's on_unpublish webhook when a streamer
// disconnects (OBS closed, network drop, etc). Rather than ending the stream
// immediately, it starts a reconnection grace period: the stream is only
// finalized (marked offline, DVR processed) if no matching on_publish arrives
// within graceDuration. This absorbs brief network blips without cutting
// viewers off or generating a throwaway replay video for the interrupted
// segment.
func (u *UseCase) UnpublishStream(ctx context.Context, streamKey string) error {
	if _, err := u.repo.GetByStreamKey(ctx, streamKey); err != nil {
		return err
	}

	timer := time.AfterFunc(u.graceDuration, func() {
		u.finalizeStreamEnd(streamKey)
	})

	u.mu.Lock()
	if existing, ok := u.pending[streamKey]; ok {
		existing.timer.Stop()
	}
	u.pending[streamKey] = &pendingEnd{timer: timer}
	u.mu.Unlock()

	return nil
}

// finalizeStreamEnd runs after the reconnection grace period elapses with no
// reconnect. It marks the stream offline and, if SRS's on_dvr webhook handed
// us a recording path while we were waiting, processes it into a replay
// video now.
func (u *UseCase) finalizeStreamEnd(streamKey string) {
	u.mu.Lock()
	p, ok := u.pending[streamKey]
	delete(u.pending, streamKey)
	u.mu.Unlock()
	if !ok {
		return // already cancelled by a reconnect
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ls, err := u.repo.GetByStreamKey(ctx, streamKey)
	if err != nil {
		return
	}

	now := time.Now().UTC()
	ls.IsLive = false
	ls.EndedAt = &now
	ls.UpdatedAt = now
	_ = u.repo.Update(ctx, &ls)

	if p.dvrPath != "" {
		_ = u.processDVR(ctx, ls, p.dvrPath)
	}
}

// dvrObjectKeyDuration computes a human-readable "HH:MM:SS"/"MM:SS" duration
// string from a livestream's started_at to now.
func dvrDuration(startedAt *time.Time, now time.Time) string {
	if startedAt == nil {
		return "00:00"
	}
	dur := now.Sub(*startedAt)
	h := int(dur.Hours())
	m := int(dur.Minutes()) % 60
	s := int(dur.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// HandleDVRComplete is called via SRS's on_dvr webhook once a livestream
// recording has been fully written to disk. SRS treats every reconnect as a
// brand new RTMP session, so on_dvr fires for the interrupted segment almost
// immediately after on_unpublish — well before our reconnection grace period
// (see UnpublishStream) has had a chance to expire. If we're still within
// that grace window, stash the recording path instead of processing it now:
// if the streamer reconnects, this segment is discarded (the stream is
// treated as continuous); only if the grace period actually expires does
// finalizeStreamEnd process it as the real end-of-stream recording.
func (u *UseCase) HandleDVRComplete(ctx context.Context, streamKey string) error {
	localPath := filepath.Join(u.dvrLocalDir, "live", streamKey+".flv")

	u.mu.Lock()
	if p, ok := u.pending[streamKey]; ok {
		p.dvrPath = localPath
		u.mu.Unlock()
		return nil
	}
	u.mu.Unlock()

	ls, err := u.repo.GetByStreamKey(ctx, streamKey)
	if err != nil {
		return fmt.Errorf("LivestreamUseCase - HandleDVRComplete - GetByStreamKey: %w", err)
	}

	return u.processDVR(ctx, ls, localPath)
}

// processDVR uploads a finished DVR recording to the raw-videos S3 bucket,
// creates a private draft "replay" video row, and publishes a NATS transcode
// job — reusing the exact same pipeline (and existing /v1/transcode/callback
// completion webhook) a manual upload goes through, rather than fabricating a
// "complete" video that points at a file nobody produced.
func (u *UseCase) processDVR(ctx context.Context, ls entity.Livestream, localPath string) error {
	if u.minioClient == nil || u.natsPublisher == nil || u.videoRepo == nil || u.dvrLocalDir == "" {
		return errors.New("processDVR: DVR pipeline not configured")
	}

	if _, err := os.Stat(localPath); err != nil {
		return fmt.Errorf("LivestreamUseCase - processDVR - recording file not found at %s: %w", localPath, err)
	}

	now := time.Now().UTC()
	videoID := uuid.New().String()
	rawS3Key := fmt.Sprintf("raw-uploads/%s/raw.flv", videoID)

	if err := u.minioClient.UploadFile(ctx, u.rawBucket, rawS3Key, localPath); err != nil {
		return fmt.Errorf("LivestreamUseCase - processDVR - UploadFile: %w", err)
	}
	_ = os.Remove(localPath) // best-effort local cleanup, the file now lives in S3

	replayVideo := entity.Video{
		ID:          videoID,
		UserID:      ls.UserID,
		UserName:    ls.UserName,
		UserAvatar:  ls.UserAvatar,
		Title:       fmt.Sprintf("[Replay] %s", ls.Title),
		Description: fmt.Sprintf("Bản ghi hình phát trực tiếp ngày %s", now.Format("02/01/2006 15:04")),
		Category:    ls.Category,
		Status:      entity.VideoStatusProcessing,
		Visibility:  entity.VideoVisibilityPrivate, // streamer reviews & publishes from Studio
		RawS3Key:    rawS3Key,
		Duration:    dvrDuration(ls.StartedAt, now),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := u.videoRepo.Store(ctx, &replayVideo); err != nil {
		return fmt.Errorf("LivestreamUseCase - processDVR - Store: %w", err)
	}

	_ = u.natsPublisher.PublishTranscodeJob(videoID, rawS3Key, ls.UserID)

	return nil
}

func (u *UseCase) GetStreamByID(ctx context.Context, id string) (response.LivestreamResponse, error) {
	ls, err := u.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, entity.ErrLivestreamNotFound) {
			now := time.Now().UTC()
			mockLs := entity.Livestream{
				ID:           id,
				UserID:       "usr_demo",
				Title:        "Exploring the edge of the universe",
				Category:     "Science",
				IsLive:       true,
				HLSUrl:       "http://localhost:8082/live/sk_live_8h2k_92md_71px.m3u8",
				ViewersCount: 0,
				StartedAt:    &now,
			}
			if id == "2" {
				mockLs.Title = "Ranked grind · road to Radiant"
				mockLs.Category = "Gaming"
				mockLs.ViewersCount = 0
			} else if id == "3" {
				mockLs.Title = "Late night studio session"
				mockLs.Category = "Music"
				mockLs.ViewersCount = 0
			}
			return u.withFollowersCount(ctx, mapper.ToLivestreamResponse(mockLs)), nil
		}
		return response.LivestreamResponse{}, err
	}
	return u.withFollowersCount(ctx, mapper.ToLivestreamResponse(ls)), nil
}

func (u *UseCase) ListActiveStreams(ctx context.Context, category string, page, limit int) (response.PageResponse[response.LivestreamResponse], error) {
	offset := (page - 1) * limit
	streams, total, err := u.repo.ListActive(ctx, category, limit, offset)
	if err != nil {
		return response.PageResponse[response.LivestreamResponse]{}, err
	}

	pageResp := mapper.ToLivestreamPageResponse(streams, total, page, limit)
	for i := range pageResp.Data {
		pageResp.Data[i] = u.withFollowersCount(ctx, pageResp.Data[i])
	}
	return pageResp, nil
}

func (u *UseCase) UpdateStreamInfo(ctx context.Context, userID string, req request.UpdateLivestreamInfo) (response.LivestreamResponse, error) {
	ls, err := u.repo.GetByUserID(ctx, userID)
	if err != nil {
		return response.LivestreamResponse{}, err
	}

	if req.Title != "" {
		ls.Title = req.Title
	}
	if req.Category != "" {
		ls.Category = req.Category
	}
	ls.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, &ls); err != nil {
		return response.LivestreamResponse{}, fmt.Errorf("LivestreamUseCase - UpdateStreamInfo - Update: %w", err)
	}

	return mapper.ToLivestreamResponse(ls), nil
}

func (u *UseCase) SendChatMessage(ctx context.Context, streamID string, req request.SendChatMessage) (response.ChatMessageResponse, error) {
	if req.Username == "" {
		req.Username = "Guest"
	}
	if req.Avatar == "" {
		req.Avatar = req.Username[0:1]
	}

	msg := events.ChatMessage{
		StreamID:  streamID,
		Username:  req.Username,
		Avatar:    req.Avatar,
		Text:      req.Text,
		CreatedAt: time.Now().Format("15:04:05"),
	}

	if u.chatHub != nil {
		u.chatHub.Broadcast(streamID, msg)
	}

	return mapper.ToChatMessageResponse(msg), nil
}

// SubscribeChat maps the raw internal ChatHub event stream to the public
// response DTO shape before handing it to the controller, so the SSE stream
// (like the REST endpoint) never leaks the flat internal events.ChatMessage.
func (u *UseCase) SubscribeChat(streamID string) (<-chan response.ChatMessageResponse, func(), error) {
	if u.chatHub == nil {
		return nil, nil, errors.New("chat hub unavailable")
	}
	raw, unsub := u.chatHub.Subscribe(streamID)

	mapped := make(chan response.ChatMessageResponse)
	go func() {
		defer close(mapped)
		for msg := range raw {
			mapped <- mapper.ToChatMessageResponse(msg)
		}
	}()

	return mapped, unsub, nil
}
