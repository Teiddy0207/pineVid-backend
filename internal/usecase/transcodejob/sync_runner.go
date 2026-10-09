package transcodejob

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/nats"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

const (
	SegmentSeconds  = 4
	maxBitrateRatio = 1.07
	bufsizeRatio    = 1.5
)

type Rendition struct {
	Name          string
	Height        int
	VideoBitrateK int
	AudioBitrateK int
}

var Ladder = map[string]Rendition{
	"1080p": {Name: "1080p", Height: 1080, VideoBitrateK: 5000, AudioBitrateK: 192},
	"720p":  {Name: "720p", Height: 720, VideoBitrateK: 2800, AudioBitrateK: 128},
	"480p":  {Name: "480p", Height: 480, VideoBitrateK: 1000, AudioBitrateK: 128},
	"360p":  {Name: "360p", Height: 360, VideoBitrateK: 800, AudioBitrateK: 96},
}

type SyncRunner struct {
	repo        repo.VideoRepo
	mc          *minio.Client
	rawBucket   string
	hlsBucket   string
	ffmpegPath  string
	ffprobePath string
	tempDir     string
	logger      *zap.Logger
}

func NewSyncRunner(r repo.VideoRepo, minioEndpoint, minioAccessKey, minioSecretKey string, minioUseSSL bool, rawBucket, hlsBucket, ffmpegPath, ffprobePath, tempDir string) (*SyncRunner, error) {
	mc, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: minioUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio.New: %w", err)
	}

	logger, _ := zap.NewProduction()

	return &SyncRunner{
		repo:        r,
		mc:          mc,
		rawBucket:   rawBucket,
		hlsBucket:   hlsBucket,
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
		tempDir:     tempDir,
		logger:      logger,
	}, nil
}

// PublishTranscodeJob is the control variant of the proposal's experiment
// ("no queue"): it runs FFmpeg inside the calling request and returns only
// when the video is transcoded and uploaded, so the API's latency includes the
// whole transcode. It must stay blocking — an earlier version ran FFmpeg in a
// goroutine and returned at once, which hid exactly the effect being measured.
func (s *SyncRunner) PublishTranscodeJob(_ context.Context, job nats.TranscodeJob) error {
	// Background context: a client that gives up must not abort the transcode
	// half way (the same job would be stuck in PROCESSING).
	bgCtx := context.Background()
	s.logger.Info("SyncRunner started job", zap.String("job_id", job.JobID), zap.String("video_id", job.VideoID))

	processing := entity.TranscodeEvent{
		IdempotencyKey: entity.TranscodeIdempotencyKey(job.JobID, entity.TranscodeEventProcessing),
		JobID:          job.JobID,
		VideoID:        job.VideoID,
		Status:         entity.TranscodeEventProcessing,
	}
	if _, err := s.repo.ApplyTranscodeEvent(bgCtx, processing, time.Now().UTC()); err != nil {
		s.logger.Error("Failed to apply processing event", zap.Error(err))
		return fmt.Errorf("apply processing event: %w", err)
	}

	if err := s.runTranscode(bgCtx, job); err != nil {
		s.logger.Error("SyncRunner failed", zap.Error(err))
		failed := entity.TranscodeEvent{
			IdempotencyKey: entity.TranscodeIdempotencyKey(job.JobID, entity.TranscodeEventFailed),
			JobID:          job.JobID,
			VideoID:        job.VideoID,
			Status:         entity.TranscodeEventFailed,
			ErrorMessage:   err.Error(),
		}
		if _, applyErr := s.repo.ApplyTranscodeEvent(bgCtx, failed, time.Now().UTC()); applyErr != nil {
			s.logger.Error("Failed to apply failed event", zap.Error(applyErr))
		}
	}
	return nil
}

func (s *SyncRunner) runTranscode(ctx context.Context, job nats.TranscodeJob) error {
	jobDir := filepath.Join(s.tempDir, job.JobID)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(jobDir)

	inputPath := filepath.Join(jobDir, "input.raw")
	err := s.mc.FGetObject(ctx, s.rawBucket, job.SourceKey, inputPath, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to download source: %w", err)
	}

	height, hasAudio, err := s.probe(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("probe failed: %w", err)
	}

	requested, err := resolveRenditions(job.Renditions)
	if err != nil {
		return fmt.Errorf("resolve renditions failed: %w", err)
	}

	rs := selectForSource(requested, height)

	args := buildHLSArgs(inputPath, jobDir, rs, hasAudio)
	cmd := exec.CommandContext(ctx, s.ffmpegPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, string(out))
	}

	// Make HLS bucket if not exists
	exists, err := s.mc.BucketExists(ctx, s.hlsBucket)
	if err == nil && !exists {
		s.mc.MakeBucket(ctx, s.hlsBucket, minio.MakeBucketOptions{})
	}

	// Upload HLS files
	files, err := os.ReadDir(jobDir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir() || f.Name() == "input.raw" {
			continue
		}

		localFilePath := filepath.Join(jobDir, f.Name())
		objectKey := job.VideoID + "/" + f.Name()

		contentType := "video/MP2T"
		if strings.HasSuffix(f.Name(), ".m3u8") {
			contentType = "application/vnd.apple.mpegurl"
		}

		_, err := s.mc.FPutObject(ctx, s.hlsBucket, objectKey, localFilePath, minio.PutObjectOptions{
			ContentType: contentType,
		})
		if err != nil {
			return fmt.Errorf("upload failed for %s: %w", f.Name(), err)
		}
	}

	hlsURL := fmt.Sprintf("/%s/%s/master.m3u8", s.hlsBucket, job.VideoID)

	compEvent := entity.TranscodeEvent{
		IdempotencyKey: entity.TranscodeIdempotencyKey(job.JobID, entity.TranscodeEventCompleted),
		JobID:          job.JobID,
		VideoID:        job.VideoID,
		Status:         entity.TranscodeEventCompleted,
		HLSMasterURL:   hlsURL,
	}
	if _, err := s.repo.ApplyTranscodeEvent(ctx, compEvent, time.Now().UTC()); err != nil {
		return fmt.Errorf("apply completed event failed: %w", err)
	}

	return nil
}

func (s *SyncRunner) probe(ctx context.Context, input string) (height int, hasAudio bool, err error) {
	out, err := exec.CommandContext(ctx, s.ffprobePath,
		"-v", "error", "-show_entries", "stream=codec_type,height", "-of", "json", input,
	).Output()
	if err != nil {
		return 0, false, err
	}

	var probe struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		return 0, false, err
	}

	for _, st := range probe.Streams {
		switch st.CodecType {
		case "video":
			if height == 0 {
				height = st.Height
			}
		case "audio":
			hasAudio = true
		}
	}
	if height == 0 {
		return 0, false, fmt.Errorf("no video stream")
	}
	return height, hasAudio, nil
}

func resolveRenditions(names []string) ([]Rendition, error) {
	seen := map[string]bool{}
	var out []Rendition
	for _, n := range names {
		r, ok := Ladder[strings.TrimSpace(n)]
		if !ok {
			return nil, fmt.Errorf("unknown rendition %q", n)
		}
		if !seen[r.Name] {
			seen[r.Name] = true
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no renditions requested")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Height > out[j].Height })
	return out, nil
}

func selectForSource(rs []Rendition, sourceHeight int) []Rendition {
	if sourceHeight <= 0 {
		return rs
	}
	var out []Rendition
	for _, r := range rs {
		if r.Height <= sourceHeight {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		lowest := rs[len(rs)-1]
		out = []Rendition{{
			Name:          fmt.Sprintf("%dp", sourceHeight),
			Height:        sourceHeight,
			VideoBitrateK: lowest.VideoBitrateK,
			AudioBitrateK: lowest.AudioBitrateK,
		}}
	}
	return out
}

func buildHLSArgs(input, outDir string, rs []Rendition, hasAudio bool) []string {
	args := []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-i", input,
		"-preset", "veryfast",
		"-sc_threshold", "0",
		"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", SegmentSeconds),
	}

	for range rs {
		args = append(args, "-map", "0:v:0")
		if hasAudio {
			args = append(args, "-map", "0:a:0")
		}
	}

	streamMap := make([]string, 0, len(rs))
	for i, r := range rs {
		args = append(args,
			fmt.Sprintf("-filter:v:%d", i), fmt.Sprintf("scale=-2:%d", r.Height),
			fmt.Sprintf("-b:v:%d", i), fmt.Sprintf("%dk", r.VideoBitrateK),
			fmt.Sprintf("-maxrate:v:%d", i), fmt.Sprintf("%dk", int(float64(r.VideoBitrateK)*maxBitrateRatio)),
			fmt.Sprintf("-bufsize:v:%d", i), fmt.Sprintf("%dk", int(float64(r.VideoBitrateK)*bufsizeRatio)),
		)
		entry := fmt.Sprintf("v:%d", i)
		if hasAudio {
			args = append(args, fmt.Sprintf("-b:a:%d", i), fmt.Sprintf("%dk", r.AudioBitrateK))
			entry += fmt.Sprintf(",a:%d", i)
		}
		streamMap = append(streamMap, entry+",name:"+r.Name)
	}

	args = append(args, "-c:v", "libx264", "-profile:v", "main", "-pix_fmt", "yuv420p")
	if hasAudio {
		args = append(args, "-c:a", "aac", "-ar", "48000")
	}

	return append(args,
		"-f", "hls",
		"-hls_time", fmt.Sprint(SegmentSeconds),
		"-hls_playlist_type", "vod",
		"-hls_flags", "independent_segments",
		"-hls_segment_filename", filepath.Join(outDir, "v%v_file%03d.ts"),
		"-master_pl_name", "master.m3u8",
		"-var_stream_map", strings.Join(streamMap, " "),
		filepath.Join(outDir, "v%v.m3u8"),
	)
}
