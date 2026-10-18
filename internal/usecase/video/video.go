package video

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/repo/persistent/view"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/internal/usecase/transcodejob"
	"github.com/evrone/go-clean-template/pkg/nats"
	"github.com/google/uuid"
)

type UseCase struct {
	repo      repo.VideoRepo
	viewRepo  *view.Repo
	likeRepo  repo.LikeRepo
	savedRepo repo.SavedVideoRepo
	jobs      *transcodejob.Enqueuer
	notifUc   usecase.Notification
	viewPub   ViewPublisher
	
	ticketWaitDisabled bool

	// uploadBaseURL is the object-store address the browser uploads to.
	uploadBaseURL string

	// uploadSigner, when set, replaces the bare upload URL with a presigned one.
	uploadSigner UploadSigner
	rawBucket    string

	// publicBucket is the anonymously readable bucket for user-facing images.
	publicBucket string
}

// UploadSigner issues presigned PUT URLs for the raw-videos bucket.
type UploadSigner interface {
	PresignPut(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error)
}

// uploadURLTTL is how long a presigned upload URL stays valid: long enough for
// a large upload on a slow link, short enough that a leaked URL expires.
const uploadURLTTL = 2 * time.Hour

// imageUploadURLTTL is short: images are small and uploaded right away.
const imageUploadURLTTL = 15 * time.Minute

// defaultPublicBucket matches MINIO_PUBLIC_BUCKET's default.
const defaultPublicBucket = "public-assets"

// WithUploadSigner makes CreateUpload return presigned URLs for bucket.
func (u *UseCase) WithUploadSigner(s UploadSigner, bucket string) *UseCase {
	u.uploadSigner = s
	u.rawBucket = bucket
	return u
}

// WithPublicBucket sets the anonymously readable bucket that CreateImageUpload
// writes to (MINIO_PUBLIC_BUCKET).
func (u *UseCase) WithPublicBucket(bucket string) *UseCase {
	u.publicBucket = bucket
	return u
}

// defaultUploadBaseURL is the local MinIO of the dev stack.
const defaultUploadBaseURL = "http://localhost:9000"

// WithUploadBaseURL sets the public address of the object store used in the
// upload URLs (MINIO_PUBLIC_URL); needed whenever the browser is not on the
// same machine as MinIO.
func (u *UseCase) WithUploadBaseURL(base string) *UseCase {
	u.uploadBaseURL = strings.TrimRight(base, "/")
	return u
}

// ViewPublisher announces new view totals (docs/message-contract.md §2.3).
type ViewPublisher interface {
	PublishViewUpdated(ev nats.ViewUpdated) error
}

// WithViewPublisher enables live view-count announcements.
func (u *UseCase) WithViewPublisher(p ViewPublisher) *UseCase {
	u.viewPub = p
	return u
}

// DisableTicketWait is for BENCHMARK_MODE only.
func (u *UseCase) DisableTicketWait() {
	u.ticketWaitDisabled = true
}

func New(r repo.VideoRepo, vRepo *view.Repo, likeRepo repo.LikeRepo, savedRepo repo.SavedVideoRepo, jobs *transcodejob.Enqueuer, notifUc usecase.Notification) *UseCase {
	return &UseCase{
		repo:      r,
		viewRepo:  vRepo,
		likeRepo:  likeRepo,
		savedRepo: savedRepo,
		jobs:      jobs,
		notifUc:   notifUc,
	}
}

// StartViewSession is called when playback starts: it returns a one-time
// ticket the client redeems via RecordView after entity.MinWatchForView of
// watching. userID may be "" (anonymous viewers count too).
func (u *UseCase) StartViewSession(ctx context.Context, videoID, userID string) (string, error) {
	if err := u.EnsureViewable(ctx, videoID, userID); err != nil {
		return "", err
	}
	if u.viewRepo == nil {
		return "", fmt.Errorf("VideoUseCase - StartViewSession: view counter unavailable")
	}
	return u.viewRepo.IssueViewTicket(ctx, videoID)
}

// RecordView redeems a view ticket (enforcing the minimum watch time on the
// server clock) and counts the view, one per viewer per video per 60 s
// (view.ViewerID). It returns the video's total views — synced plus pending —
// and, when the view counted, announces that total on video.views.updated
// for the live WebSocket counters.
func (u *UseCase) RecordView(ctx context.Context, videoID, ticket, userID, clientIP, userAgent string) (bool, int64, error) {
	// Viewability first: a private video answers 404 whether or not the
	// caller holds a ticket, so its existence doesn't leak.
	v, err := u.repo.GetByID(ctx, videoID)
	if err != nil {
		return false, 0, err
	}
	if !v.ViewableBy(userID) {
		return false, 0, entity.ErrVideoNotFound
	}
	if u.viewRepo == nil {
		return false, v.Views, nil
	}

	minWatch := entity.MinWatchForView
	if u.ticketWaitDisabled {
		minWatch = 0
	}
	if err := u.viewRepo.RedeemViewTicket(ctx, ticket, videoID, minWatch); err != nil {
		return false, 0, err
	}

	recorded, _, err := u.viewRepo.RecordView(ctx, videoID, userID, clientIP, userAgent)
	if err != nil {
		return false, 0, err
	}

	total := v.Views + u.viewRepo.GetPendingViewsForVideo(ctx, videoID)
	if recorded && u.viewPub != nil {
		// Best effort: the view is counted either way; only the live
		// counter update is lost if NATS is down.
		_ = u.viewPub.PublishViewUpdated(nats.ViewUpdated{VideoID: videoID, TotalViews: total, TS: time.Now().UnixMilli()})
	}
	return recorded, total, nil
}

// CurrentViews is the total a watch page shows on connect: synced views in
// PostgreSQL plus those still pending in Redis.
func (u *UseCase) CurrentViews(ctx context.Context, videoID, userID string) (int64, error) {
	v, err := u.repo.GetByID(ctx, videoID)
	if err != nil {
		return 0, err
	}
	if !v.ViewableBy(userID) {
		return 0, entity.ErrVideoNotFound
	}
	if u.viewRepo == nil {
		return v.Views, nil
	}
	return v.Views + u.viewRepo.GetPendingViewsForVideo(ctx, videoID), nil
}

// CreateImageUpload issues an upload URL for a user-facing image (avatar,
// thumbnail, post image). Unlike CreateUpload it creates no video row and the
// object lands in the public bucket, so the returned public_url is readable
// by anyone, logged in or not.
func (u *UseCase) CreateImageUpload(ctx context.Context, userID string, req request.CreateImageUpload) (response.ImageUploadResponse, error) {
	ext := strings.ToLower(filepath.Ext(req.FileName))
	if !entity.AllowedImageExtensions[ext] {
		return response.ImageUploadResponse{}, entity.ErrUnsupportedImageFormat
	}

	bucket := u.publicBucket
	if bucket == "" {
		bucket = defaultPublicBucket
	}
	base := u.uploadBaseURL
	if base == "" {
		base = defaultUploadBaseURL
	}

	key := fmt.Sprintf("images/%s/%s%s", userID, uuid.New().String(), ext)
	publicURL := fmt.Sprintf("%s/%s/%s", base, bucket, key)

	uploadURL := publicURL
	if u.uploadSigner != nil {
		signed, err := u.uploadSigner.PresignPut(ctx, bucket, key, imageUploadURLTTL)
		if err != nil {
			return response.ImageUploadResponse{}, fmt.Errorf("VideoUseCase - CreateImageUpload - PresignPut: %w", err)
		}
		uploadURL = signed
	}

	return response.ImageUploadResponse{UploadUrl: uploadURL, PublicUrl: publicURL}, nil
}

func (u *UseCase) CreateUpload(ctx context.Context, userID string, req request.CreateVideoUpload) (response.UploadUrlResponse, error) {
	ext := strings.ToLower(filepath.Ext(req.FileName))
	if ext == "" {
		ext = ".mp4"
	}
	if !entity.AllowedVideoExtensions[ext] {
		return response.UploadUrlResponse{}, entity.ErrUnsupportedVideoFormat
	}

	videoID := uuid.New().String()
	s3Key := fmt.Sprintf("raw-uploads/%s/raw%s", videoID, ext)

	v := mapper.ToVideoEntity(userID, req, videoID, s3Key)
	v.CreatedAt = time.Now().UTC()
	v.UpdatedAt = time.Now().UTC()

	if err := u.repo.Store(ctx, &v); err != nil {
		return response.UploadUrlResponse{}, fmt.Errorf("VideoUseCase - CreateUpload - Store: %w", err)
	}

	// Upload URL for the raw-videos bucket, as the browser reaches MinIO.
	var presignedUrl string
	if u.uploadSigner != nil {
		signed, err := u.uploadSigner.PresignPut(ctx, u.rawBucket, s3Key, uploadURLTTL)
		if err != nil {
			return response.UploadUrlResponse{}, fmt.Errorf("VideoUseCase - CreateUpload - PresignPut: %w", err)
		}
		presignedUrl = signed
	} else {
		base := u.uploadBaseURL
		if base == "" {
			base = defaultUploadBaseURL
		}
		presignedUrl = fmt.Sprintf("%s/raw-videos/%s", base, s3Key)
	}

	return response.UploadUrlResponse{
		VideoID:   videoID,
		UploadUrl: presignedUrl,
		RawS3Key:  s3Key,
	}, nil
}

func (u *UseCase) ConfirmUpload(ctx context.Context, userID string, req request.ConfirmUpload) (response.VideoResponse, error) {
	v, err := u.repo.GetByID(ctx, req.VideoID)
	if err != nil {
		return response.VideoResponse{}, err
	}
	if v.UserID != userID {
		return response.VideoResponse{}, entity.ErrVideoForbidden
	}
	// Only a freshly created upload can be confirmed; confirming again must
	// not enqueue a second transcode job for the same file.
	if v.Status != entity.VideoStatusPending {
		return mapper.ToVideoResponse(v), nil
	}

	// Enqueue only succeeds from pending, so two racing confirms still queue
	// one job; if the queue is down the video stays pending and the error
	// reaches the client, which can confirm again.
	if _, _, err := u.jobs.Enqueue(ctx, v.ID, v.RawS3Key, entity.VideoStatusPending); err != nil {
		return response.VideoResponse{}, fmt.Errorf("VideoUseCase - ConfirmUpload - Enqueue: %w", err)
	}

	return u.reload(ctx, v.ID)
}

// reload re-reads a video after a conditional update and maps it.
func (u *UseCase) reload(ctx context.Context, id string) (response.VideoResponse, error) {
	v, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return response.VideoResponse{}, err
	}
	return mapper.ToVideoResponse(v), nil
}

// GetByID fetches a single video for display. userID is the current
// viewer's ID if authenticated, or "" for an anonymous caller — it's used to
// resolve IsLiked (LikesCount itself is not viewer-specific).
func (u *UseCase) GetByID(ctx context.Context, id, userID string) (response.VideoResponse, error) {
	v, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return response.VideoResponse{}, err
	}
	// Private/banned/unfinished videos look exactly like missing ones to
	// anyone but the owner, so their existence isn't leaked either.
	if !v.ViewableBy(userID) {
		return response.VideoResponse{}, entity.ErrVideoNotFound
	}
	if u.viewRepo != nil {
		v.Views += u.viewRepo.GetPendingViewsForVideo(ctx, id)
	}

	resDTO := mapper.ToVideoResponse(v)
	if u.likeRepo != nil {
		if count, err := u.likeRepo.GetLikeCount(ctx, id); err == nil {
			resDTO.LikesCount = count
		}
		if liked, err := u.likeRepo.IsLikedByUser(ctx, id, userID); err == nil {
			resDTO.IsLiked = liked
		}
	}
	if u.savedRepo != nil {
		if saved, err := u.savedRepo.IsSavedByUser(ctx, id, userID); err == nil {
			resDTO.IsSaved = saved
		}
	}
	return resDTO, nil
}

// EnsureViewable returns entity.ErrVideoNotFound unless userID may watch the
// video (see entity.Video.ViewableBy). Guards every per-video endpoint that
// isn't GetByID itself: subtitles, comments, likes, saves, views.
func (u *UseCase) EnsureViewable(ctx context.Context, videoID, userID string) error {
	v, err := u.repo.GetByID(ctx, videoID)
	if err != nil {
		return err
	}
	if !v.ViewableBy(userID) {
		return entity.ErrVideoNotFound
	}
	return nil
}

// trendingScanLimit is how many top-ranked IDs GetTrending pulls from Redis
// before paginating in Go — generous enough to cover several pages without
// re-querying Redis per page.
const trendingScanLimit = 100

// GetTrending ranks public/complete videos by view count over a recent
// window ("24h" = today's UTC bucket, "7d" = the last 7 days) instead of
// the lifetime total, so a genuinely fresh burst of views can surface a
// video ahead of an old one that's merely accumulated more views over time.
// Falls back to ListPublicVideos (sorted by recency) when Redis has no
// trending data yet — e.g. a fresh deployment before any view has landed.
func (u *UseCase) GetTrending(ctx context.Context, window, userID string, page, limit int) (response.PageResponse[response.VideoResponse], error) {
	days := 1
	if window == "7d" {
		days = 7
	}

	var rankedIDs []string
	if u.viewRepo != nil {
		rankedIDs, _ = u.viewRepo.GetTrendingVideoIDs(ctx, days, trendingScanLimit)
	}

	if len(rankedIDs) == 0 {
		return u.ListPublicVideos(ctx, userID, "", "", page, limit)
	}

	videos, err := u.repo.GetByIDs(ctx, rankedIDs)
	if err != nil {
		return response.PageResponse[response.VideoResponse]{}, fmt.Errorf("VideoUseCase - GetTrending - GetByIDs: %w", err)
	}

	byID := make(map[string]entity.Video, len(videos))
	for _, v := range videos {
		byID[v.ID] = v
	}

	ranked := make([]entity.Video, 0, len(videos))
	for _, id := range rankedIDs {
		if v, ok := byID[id]; ok {
			ranked = append(ranked, v)
		}
	}

	total := len(ranked)
	offset := (page - 1) * limit
	if offset >= total {
		return mapper.ToVideoPageResponse(nil, total, page, limit), nil
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return mapper.ToVideoPageResponse(ranked[offset:end], total, page, limit), nil
}

// GetReelsFeed ranks short-form ("reel") videos by the same recent-window
// view-count mechanism as GetTrending, but restricted to entity.Video.IsReel.
// Falls back to the newest reels by recency (a plain SQL-paginated query,
// not limited by trendingScanLimit) whenever Redis has no ranked IDs yet, or
// when none of the currently-trending videos happen to be reels.
//
// Each reel carries LikesCount/IsLiked for userID: the reels UI toggles the
// heart from is_liked, so leaving it false on an already-liked reel made the
// next tap silently unlike it.
func (u *UseCase) GetReelsFeed(ctx context.Context, userID, window string, page, limit int) (response.PageResponse[response.VideoResponse], error) {
	res, err := u.getReelsFeed(ctx, window, page, limit)
	if err != nil {
		return res, err
	}
	u.withLikeInfo(ctx, res.Data, userID)
	return res, nil
}

// withLikeInfo fills LikesCount/IsLiked on a page of videos in place with
// one batched query. Best-effort, like GetByID: a failure leaves the zero
// values rather than failing the whole page.
func (u *UseCase) withLikeInfo(ctx context.Context, videos []response.VideoResponse, userID string) {
	if u.likeRepo == nil || len(videos) == 0 {
		return
	}
	ids := make([]string, len(videos))
	for i := range videos {
		ids[i] = videos[i].ID
	}
	counts, liked, err := u.likeRepo.GetLikeInfo(ctx, ids, userID)
	if err != nil {
		return
	}
	for i := range videos {
		videos[i].LikesCount = counts[videos[i].ID]
		videos[i].IsLiked = liked[videos[i].ID]
	}
}

func (u *UseCase) getReelsFeed(ctx context.Context, window string, page, limit int) (response.PageResponse[response.VideoResponse], error) {
	days := 1
	if window == "7d" {
		days = 7
	}

	var rankedIDs []string
	if u.viewRepo != nil {
		rankedIDs, _ = u.viewRepo.GetTrendingVideoIDs(ctx, days, trendingScanLimit)
	}

	reelIDsInRankOrder := make([]string, 0, len(rankedIDs))
	if len(rankedIDs) > 0 {
		videos, err := u.repo.GetByIDs(ctx, rankedIDs)
		if err == nil {
			byID := make(map[string]entity.Video, len(videos))
			for _, v := range videos {
				if v.IsReel {
					byID[v.ID] = v
				}
			}
			for _, id := range rankedIDs {
				if _, ok := byID[id]; ok {
					reelIDsInRankOrder = append(reelIDsInRankOrder, id)
				}
			}
		}
	}

	if len(reelIDsInRankOrder) == 0 {
		return u.listReelsByRecency(ctx, page, limit)
	}

	videos, err := u.repo.GetByIDs(ctx, reelIDsInRankOrder)
	if err != nil {
		return response.PageResponse[response.VideoResponse]{}, fmt.Errorf("VideoUseCase - GetReelsFeed - GetByIDs: %w", err)
	}

	byID := make(map[string]entity.Video, len(videos))
	for _, v := range videos {
		byID[v.ID] = v
	}

	ranked := make([]entity.Video, 0, len(videos))
	for _, id := range reelIDsInRankOrder {
		if v, ok := byID[id]; ok {
			ranked = append(ranked, v)
		}
	}

	total := len(ranked)
	offset := (page - 1) * limit
	if offset >= total {
		return mapper.ToVideoPageResponse(nil, total, page, limit), nil
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return mapper.ToVideoPageResponse(ranked[offset:end], total, page, limit), nil
}

func (u *UseCase) listReelsByRecency(ctx context.Context, page, limit int) (response.PageResponse[response.VideoResponse], error) {
	offset := (page - 1) * limit
	status := entity.VideoStatusComplete
	visibility := entity.VideoVisibilityPublic
	isReel := true

	videos, total, err := u.repo.List(ctx, repo.VideoFilter{
		Status:     &status,
		Visibility: &visibility,
		IsReel:     &isReel,
		Limit:      uint64(limit),
		Offset:     uint64(offset),
	})
	if err != nil {
		return response.PageResponse[response.VideoResponse]{}, fmt.Errorf("VideoUseCase - listReelsByRecency - List: %w", err)
	}

	return mapper.ToVideoPageResponse(videos, total, page, limit), nil
}

func (u *UseCase) ListPublicVideos(ctx context.Context, userID, category, query string, page, limit int) (response.PageResponse[response.VideoResponse], error) {
	offset := (page - 1) * limit
	status := entity.VideoStatusComplete
	visibility := entity.VideoVisibilityPublic

	filter := repo.VideoFilter{
		UserID:     userID,
		Category:   category,
		Query:      query,
		Status:     &status,
		Visibility: &visibility,
		Limit:      uint64(limit),
		Offset:     uint64(offset),
	}

	videos, total, err := u.repo.List(ctx, filter)
	if err != nil {
		return response.PageResponse[response.VideoResponse]{}, err
	}

	return mapper.ToVideoPageResponse(videos, total, page, limit), nil
}

func (u *UseCase) ListStudioVideos(ctx context.Context, userID, query string, page, limit int) (response.PageResponse[response.VideoResponse], error) {
	offset := (page - 1) * limit

	filter := repo.VideoFilter{
		UserID: userID,
		Query:  query,
		Limit:  uint64(limit),
		Offset: uint64(offset),
	}

	videos, total, err := u.repo.List(ctx, filter)
	if err != nil {
		return response.PageResponse[response.VideoResponse]{}, err
	}

	return mapper.ToVideoPageResponse(videos, total, page, limit), nil
}

func (u *UseCase) PublishVideo(ctx context.Context, userID, videoID string) (response.VideoResponse, error) {
	v, err := u.repo.GetByID(ctx, videoID)
	if err != nil {
		return response.VideoResponse{}, err
	}
	if v.UserID != userID {
		return response.VideoResponse{}, entity.ErrVideoForbidden
	}

	wasAlreadyPublic := v.Visibility == entity.VideoVisibilityPublic

	v.Visibility = entity.VideoVisibilityPublic
	v.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, &v); err != nil {
		return response.VideoResponse{}, fmt.Errorf("VideoUseCase - PublishVideo - Update: %w", err)
	}

	// Notify followers of the new video — but only the first time it goes
	// public, so re-saving an already-public video doesn't spam followers.
	if u.notifUc != nil && !wasAlreadyPublic {
		go func() {
			notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = u.notifUc.NotifyFollowers(
				notifyCtx,
				v.UserID, v.UserName, v.UserAvatar,
				entity.NotificationTypeNewVideo,
				fmt.Sprintf("%s posted a new video", v.UserName),
				v.Title,
				fmt.Sprintf("/watch/%s", v.ID),
			)
		}()
	}

	return mapper.ToVideoResponse(v), nil
}

func (u *UseCase) UpdateVideo(ctx context.Context, userID, videoID string, req request.UpdateVideo) (response.VideoResponse, error) {
	v, err := u.repo.GetByID(ctx, videoID)
	if err != nil {
		return response.VideoResponse{}, err
	}
	if v.UserID != userID {
		return response.VideoResponse{}, entity.ErrVideoForbidden
	}

	mapper.ApplyVideoUpdate(&v, req)
	v.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, &v); err != nil {
		return response.VideoResponse{}, fmt.Errorf("VideoUseCase - UpdateVideo - Update: %w", err)
	}

	return mapper.ToVideoResponse(v), nil
}

func (u *UseCase) UpdateThumbnail(ctx context.Context, userID, videoID string, req request.UpdateThumbnail) (response.VideoResponse, error) {
	v, err := u.repo.GetByID(ctx, videoID)
	if err != nil {
		return response.VideoResponse{}, err
	}
	if v.UserID != userID {
		return response.VideoResponse{}, entity.ErrVideoForbidden
	}

	mapper.ApplyThumbnailUpdate(&v, req)
	v.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, &v); err != nil {
		return response.VideoResponse{}, fmt.Errorf("VideoUseCase - UpdateThumbnail - Update: %w", err)
	}

	return mapper.ToVideoResponse(v), nil
}

// RetryTranscode re-queues a NATS transcode job for a video whose previous
// attempt failed, reusing the already-uploaded raw file — the owner doesn't
// need to re-upload from scratch.
func (u *UseCase) RetryTranscode(ctx context.Context, userID, videoID string) (response.VideoResponse, error) {
	v, err := u.repo.GetByID(ctx, videoID)
	if err != nil {
		return response.VideoResponse{}, err
	}
	if v.UserID != userID {
		return response.VideoResponse{}, entity.ErrVideoForbidden
	}
	if v.Status != entity.VideoStatusFailed {
		return response.VideoResponse{}, entity.ErrVideoNotFailed
	}

	_, queued, err := u.jobs.Enqueue(ctx, v.ID, v.RawS3Key, entity.VideoStatusFailed)
	if err != nil {
		return response.VideoResponse{}, fmt.Errorf("VideoUseCase - RetryTranscode - Enqueue: %w", err)
	}
	if !queued {
		// Someone else retried it between our read and the update.
		return response.VideoResponse{}, entity.ErrVideoNotFailed
	}

	return u.reload(ctx, v.ID)
}

func (u *UseCase) DeleteVideo(ctx context.Context, userID, videoID string) error {
	v, err := u.repo.GetByID(ctx, videoID)
	if err != nil {
		return err
	}
	if v.UserID != userID {
		return entity.ErrVideoForbidden
	}
	return u.repo.Delete(ctx, videoID)
}

// HandleTranscodeEvent applies one transcode event (webhook or DLQ) exactly
// once — see docs/message-contract.md §4.3. NATS delivers at least once, so
// duplicates, stale jobs and backwards moves come back as no-op results, not
// errors. res.Before.UserID is the owner, for routing the SSE status event.
func (u *UseCase) HandleTranscodeEvent(ctx context.Context, ev entity.TranscodeEvent) (entity.TranscodeEventResult, error) {
	if err := entity.ValidateTranscodeEvent(ev); err != nil {
		return entity.TranscodeEventResult{}, err
	}

	res, err := u.repo.ApplyTranscodeEvent(ctx, ev, time.Now().UTC())
	if err != nil {
		return entity.TranscodeEventResult{}, fmt.Errorf("VideoUseCase - HandleTranscodeEvent - ApplyTranscodeEvent: %w", err)
	}

	// Notify followers once a public video is actually watchable — not at
	// upload time (PublishVideo's own notify only fires when a private/
	// unlisted video is switched to public, which doesn't cover the default
	// public-on-upload flow this transcode completion always goes through).
	v := res.Before
	if u.notifUc != nil && res.Applied && ev.Status == entity.TranscodeEventCompleted && v.Visibility == entity.VideoVisibilityPublic {
		go func() {
			notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = u.notifUc.NotifyFollowers(
				notifyCtx,
				v.UserID, v.UserName, v.UserAvatar,
				entity.NotificationTypeNewVideo,
				fmt.Sprintf("%s posted a new video", v.UserName),
				v.Title,
				fmt.Sprintf("/watch/%s", v.ID),
			)
		}()
	}

	return res, nil
}
