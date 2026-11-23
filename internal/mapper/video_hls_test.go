package mapper_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Both ways a video leaves the API (mapper.VideoResponse and a raw
// entity.Video) must carry the signed link, never the stored /hls-streams path.
func TestVideoHLSLinkIsSignedEverywhere(t *testing.T) { //nolint:paralleltest // global signer
	entity.SetHLSLinkSigner(func(p string) string { return strings.Replace(p, "/hls-streams/videos/", "/s/T/1/", 1) })
	defer entity.SetHLSLinkSigner(nil)

	v := entity.Video{ID: "v1", HLSUrl: "/hls-streams/videos/v1/master.m3u8"}
	assert.Equal(t, "/s/T/1/v1/master.m3u8", mapper.ToVideoResponse(v).HLSUrl)

	b, err := json.Marshal(v)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"hls_url":"/s/T/1/v1/master.m3u8"`)
	assert.Equal(t, "/hls-streams/videos/v1/master.m3u8", v.HLSUrl, "the stored value is not modified")
}
