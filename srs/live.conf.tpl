# PipeVid SRS config TEMPLATE — rendered to conf/live.conf by the srs service
# entrypoint in docker-compose.yml. Based on the image's default conf/http.hls.conf, with
# http_hooks added so SRS calls back to the Go backend to authenticate stream
# keys (on_publish) and mark a stream offline when the encoder disconnects
# (on_unpublish). See BUSINESS_REQUIREMENTS.md §3.3 (RTMP Auth webhook).

listen              1935;
max_connections     1000;
daemon              off;
srs_log_tank        console;
http_server {
    enabled         on;
    listen          8080;
    dir             ./objs/nginx/html;
}
# Read-only management API SRS exposes on its own — the backend polls
# GET /api/v1/streams/ periodically to reconcile the `is_live` DB flag
# against what SRS is actually publishing, in case an on_unpublish webhook
# never arrives (SRS crash, network partition, etc). See
# ReconcileLiveStreams in internal/usecase/livestream/livestream.go.
http_api {
    enabled         on;
    listen          1985;
}
vhost __defaultVhost__ {
    # The hook base URL and secret placeholders below are substituted from the
    # container env at startup (see the srs service in docker-compose.yml).
    # The backend rejects hook calls without the matching ?secret= (SRS can't
    # send custom headers), so nobody else can fake a publish/unpublish.
    http_hooks {
        enabled         on;
        on_publish      __SRS_HOOK_BASE_URL__/v1/live/auth?secret=__INTERNAL_WEBHOOK_SECRET__;
        on_unpublish    __SRS_HOOK_BASE_URL__/v1/live/unpublish?secret=__INTERNAL_WEBHOOK_SECRET__;
        on_dvr          __SRS_HOOK_BASE_URL__/v1/live/dvr?secret=__INTERNAL_WEBHOOK_SECRET__;
    }
    transcode live {
        enabled __LIVE_TRANSCODE_ENABLED__;
        ffmpeg ./objs/ffmpeg/bin/ffmpeg;
        engine p720 {
            enabled __LIVE_720_ENABLED__;
            vcodec libx264;
            vbitrate 2400;
            vfps 25;
            vwidth 0;
            vheight 0;
            vthreads 1;
            vprofile main;
            vpreset veryfast;
            vparams {
                vf "scale=-2:trunc(min(ih\,720)/2)*2";
                g 50;
                keyint_min 50;
                sc_threshold 0;
                bf 0;
            }
            acodec aac;
            abitrate 96;
            asample_rate 48000;
            achannels 2;
            oformat flv;
            output rtmp://127.0.0.1:[port]/abr/[stream]_720?vhost=pipevid-internal;
        }
        engine p480 {
            enabled __LIVE_480_ENABLED__;
            vcodec libx264;
            vbitrate 1100;
            vfps 25;
            vwidth 0;
            vheight 0;
            vthreads 1;
            vprofile main;
            vpreset veryfast;
            vparams {
                vf "scale=-2:trunc(min(ih\,480)/2)*2";
                g 50;
                keyint_min 50;
                sc_threshold 0;
                bf 0;
            }
            acodec aac;
            abitrate 96;
            asample_rate 48000;
            achannels 2;
            oformat flv;
            output rtmp://127.0.0.1:[port]/abr/[stream]_480?vhost=pipevid-internal;
        }
    }
    hls {
        enabled         on;
        hls_fragment    2;
        hls_window      12;
        hls_path        ./objs/nginx/html;
        hls_m3u8_file   [app]/[stream].m3u8;
        hls_ts_file     [app]/[stream]-[timestamp]-[seq].ts;
        hls_cleanup on;
        hls_ctx off;
        hls_ts_ctx off;
        hls_dispose 15;
    }
    # Records each publish session to a single flat .flv file so the backend's
    # on_dvr handler can pick it up (deterministic path: no [timestamp], the
    # backend computes the same path from the stream key alone — see
    # HandleDVRComplete in internal/usecase/livestream/livestream.go).
    dvr {
        enabled         on;
        dvr_plan        session;
        dvr_path        ./objs/dvr/[app]/[stream].flv;
    }
}

# Only SRS-managed FFmpeg may publish here. No DVR, hooks or transcode.
vhost pipevid-internal {
    security {
        enabled on;
        allow publish 127.0.0.1;
        allow play 127.0.0.1;
    }
    hls {
        enabled on;
        hls_fragment 2;
        hls_window 12;
        hls_path ./objs/nginx/html;
        hls_m3u8_file [app]/[stream].m3u8;
        hls_ts_file [app]/[stream]-[timestamp]-[seq].ts;
        hls_cleanup on;
        hls_ctx off;
        hls_ts_ctx off;
        hls_dispose 15;
    }
}
