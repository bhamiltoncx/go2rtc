# Google Nest

[`new in v1.6.0`](https://github.com/AlexxIT/go2rtc/releases/tag/v1.6.0)

For simplicity, it is recommended to connect the Nest/WebRTC camera to the [Home Assistant](../hass/README.md). 
But if you can somehow get the below parameters, Nest/WebRTC source will work without Home Assistant.

```yaml
streams:
  nest-doorbell: nest:?client_id=***&client_secret=***&refresh_token=***&project_id=***&device_id=***
```

## Changes in this fork

This fork carries a set of fixes for Nest/WebRTC sources that are not (yet) in upstream go2rtc. They are gated on the `nest/webrtc` producer, so other WebRTC sources behave as before.

**Streams no longer die every 5 minutes.** Upstream stored the per-stream session (device, `mediaSessionId`, expiry, extend timer) on the one `*API` object shared by every camera using the same credentials. Each dial overwrote the previous camera's session and the extend timer fired only once, so no stream was ever extended and every Nest producer was re-dialed at Google's 5-minute expiry. Each stream now owns its session and keep-alive, which re-arms after every successful `ExtendStream` and refreshes the OAuth token if an extension gets a 401 (streams outlive the hour-long token). Fixes upstream #2108 and #2319.

**Periodic keyframes.** go2rtc's 2-second keyframe request (RTCP PLI) only applied to passive WebRTC producers such as WHIP. Nest is an active-pull producer, so its keyframe interval drifted long when idle and any consumer joining mid-GOP waited up to a whole interval to start. The request is now enabled for Nest, and a FIR (RFC 5104) is sent alongside the PLI because Google was measured ignoring bare PLI on some cameras (upstream #2365, #2368). PLI and FIR are media-plane RTCP, so this does not touch the SDM API quota. Measured on a Nest doorbell: keyframe interval ~3.3 s → ~1.8 s, on-demand `frame.jpeg` grabs ~1 s.

**SPS/PPS in the RTSP SDP.** Google's SDP answer carries `profile-level-id` but no `sprop-parameter-sets` (WebRTC sends SPS/PPS in-band), and go2rtc copied that line verbatim into its RTSP DESCRIBE. RTSP clients such as ffmpeg therefore could not learn the video dimensions until an in-band keyframe arrived, forcing a large `-probesize` and slow opens. SPS/PPS are now captured from the incoming RTP (including STAP-A bundles) and appended to the codec's fmtp line once, so RTSP, MSE and MP4 consumers all see them.

**A camera that is off fails fast.** Google answers `400 FAILED_PRECONDITION "The camera is not available for streaming"` for a camera the Google Home app has switched off (for example by presence sensing). Upstream retried that with 30 s and 60 s sleeps, so every consumer of the stream hung for 90 s before failing. A definitive 4xx now returns a typed `StatusError` immediately; only transport errors and 5xx are retried, and 401/409/429 keep their existing token-refresh and back-off handling. The response body, which is the only place Google states the reason, is included in the error. This pairs well with a fallback source so an off camera shows a placeholder instead of an error:

```yaml
streams:
  garage:
    - nest:?client_id=***&client_secret=***&refresh_token=***&project_id=***&device_id=***
    - ffmpeg:/config/standby.jpg#input=-loop 1 -framerate 2 -i /config/standby.jpg#video=h264#raw=-tune stillimage
```

**No more corrupt keyframes on attach.** pion's H264 depacketizer appended every FU-A fragment to its buffer whether or not it had seen the start bit, then synthesized a NAL header when the end bit arrived. A consumer attaching mid-NAL got a NAL with a valid IDR type byte and the tail of a slice as its body; `IsKeyframe` accepted it and `api/frame.jpeg` handed it to ffmpeg, which failed with `exit status 183` ("non-intra slice in an IDR NAL unit"). With 2-second keyframes about half of all snapshots failed this way. `RTPDepay` now drops FU-A fragments received before their start bit. Reported upstream as AlexxIT/go2rtc#2490 and pion/rtp#370.

**Readable snapshot errors.** When the `frame.jpeg` transcode fails, the error now includes the last lines of ffmpeg's stderr and the NAL unit types and sizes that were handed to it (for example `7:24 8:4 5:156010`) instead of a bare exit status.

**GOP cache.** Add `#gop=1` to the Nest source (or any source) to keep the last GOP in memory so a consumer attaching mid-stream is served the cached keyframe immediately. See the [streams README](../streams/README.md#gop-cache). This is a rebase of upstream PR #1887 by seydx.
