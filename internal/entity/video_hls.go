package entity

import "encoding/json"

// hlsLinkSigner rewrites the stored hls_url of a VOD into the expiring signed
// path clients must use (pkg/hlslink). Nil = links are returned as stored
// (dev stack and benchmarks, where MinIO is read directly).
var hlsLinkSigner func(string) string

// SetHLSLinkSigner installs the signer once at startup (HLS_LINK_SECRET).
func SetHLSLinkSigner(f func(string) string) { hlsLinkSigner = f }

// SignHLSLink applies the installed signer, if any.
func SignHLSLink(path string) string {
	if hlsLinkSigner == nil {
		return path
	}
	return hlsLinkSigner(path)
}

// MarshalJSON signs hls_url on the way out, so every endpoint that returns a
// Video only ever hands out signed media links. Handlers already decide who
// may receive a Video (ViewableBy), which makes the signed link the media's
// access check too: the stored /hls-streams/... path is refused by nginx.
func (v Video) MarshalJSON() ([]byte, error) {
	type plain Video
	p := plain(v)
	p.HLSUrl = SignHLSLink(p.HLSUrl)
	return json.Marshal(p)
}
