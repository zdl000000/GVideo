import { useEffect, useRef, useState } from "react";
import Hls from "hls.js";
import { Check, ChevronUp, Maximize2, Minimize2, Pause, PictureInPicture2, Play, RectangleHorizontal, Settings2, Volume2, VolumeX } from "lucide-react";
import { formatDuration } from "../../shared/lib/format";
import type { SubtitleTrack, Video } from "../../types";

export interface PlayerQuality {
  index: number;
  height: number;
  url?: string;
}

export function parseHLSQualities(masterURL: string, playlist: string): PlayerQuality[] {
  const absoluteMasterURL = new URL(masterURL, window.location.href);
  const lines = playlist.split(/\r?\n/);
  const parsed: PlayerQuality[] = [];
  for (let index = 0; index < lines.length; index += 1) {
    const metadata = lines[index];
    if (!metadata.startsWith("#EXT-X-STREAM-INF:")) continue;
    const resolution = metadata.match(/(?:^|,)RESOLUTION=\d+x(\d+)/)?.[1];
    const uri = lines.slice(index + 1).find((line) => line.trim() && !line.startsWith("#"))?.trim();
    if (!resolution || !uri) continue;
    parsed.push({ index: parsed.length, height: Number(resolution), url: new URL(uri, absoluteMasterURL).toString() });
  }
  return parsed
    .filter((quality) => quality.height > 0)
    .filter((quality, index, all) => all.findIndex((candidate) => candidate.height === quality.height) === index)
    .sort((a, b) => a.height - b.height)
    .map((quality, index) => ({ ...quality, index }));
}

export function nativeHLSQualities(video: Video): PlayerQuality[] {
  const masterURL = new URL(video.hls_url, window.location.href);
  const levels = [360, 480, 720].filter((height) => video.source_height >= height);
  return levels.map((height, index) => ({
    index,
    height,
    url: new URL(`./${height}p/index.m3u8`, masterURL).toString()
  }));
}

export function VideoPlayer({ video }: { video: Video }) {
  const playerRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const hlsRef = useRef<Hls | null>(null);
  const qualityControlRef = useRef<HTMLDivElement>(null);
  const speedControlRef = useRef<HTMLDivElement>(null);
  const subtitleControlRef = useRef<HTMLDivElement>(null);
  const [qualities, setQualities] = useState<PlayerQuality[]>([]);
  const [selectedQuality, setSelectedQuality] = useState(-1);
  const [usingHLS, setUsingHLS] = useState(false);
  const [qualityMenuOpen, setQualityMenuOpen] = useState(false);
  const [speedMenuOpen, setSpeedMenuOpen] = useState(false);
  const [subtitleMenuOpen, setSubtitleMenuOpen] = useState(false);
  const [selectedSubtitle, setSelectedSubtitle] = useState<number | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [playing, setPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [volume, setVolume] = useState(1);
  const [muted, setMuted] = useState(false);
  const [playbackRate, setPlaybackRate] = useState(1);
  const [theaterMode, setTheaterMode] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);
  const [resumeNotice, setResumeNotice] = useState<number | null>(null);
  const lastProgressSaveRef = useRef(0);

  const saveProgress = (element: HTMLVideoElement) => {
    if (!Number.isFinite(element.currentTime) || element.currentTime < 5) return;
    if (element.duration > 0 && element.currentTime >= element.duration - 5) {
      localStorage.removeItem(`gvideo-progress-${video.id}`);
      return;
    }
    localStorage.setItem(`gvideo-progress-${video.id}`, String(Math.floor(element.currentTime)));
  };

  useEffect(() => {
    const element = videoRef.current;
    if (!element) return;

    hlsRef.current?.destroy();
    hlsRef.current = null;
    setQualities([]);
    setSelectedQuality(-1);
    setUsingHLS(false);
    setQualityMenuOpen(false);
    setSpeedMenuOpen(false);
    setSubtitleMenuOpen(false);
    setSelectedSubtitle((video.subtitle_tracks ?? []).find((track) => track.is_default)?.id ?? null);
    setSettingsOpen(false);
    setResumeNotice(null);
    setPlaying(false);
    setCurrentTime(0);
    setDuration(0);
    setPlaybackRate(1);
    lastProgressSaveRef.current = 0;

    const restoreProgress = () => {
      const saved = Number(localStorage.getItem(`gvideo-progress-${video.id}`));
      if (!Number.isFinite(saved) || saved < 5 || (element.duration > 0 && saved >= element.duration - 5)) return;
      element.currentTime = saved;
      setResumeNotice(saved);
      window.setTimeout(() => setResumeNotice(null), 3500);
    };
    const handleTimeUpdate = () => {
      setCurrentTime(element.currentTime);
      const now = Date.now();
      if (now - lastProgressSaveRef.current < 5000) return;
      lastProgressSaveRef.current = now;
      saveProgress(element);
    };
    const handlePause = () => saveProgress(element);
    const handleEnded = () => localStorage.removeItem(`gvideo-progress-${video.id}`);
    const handleLoadedMetadata = () => {
      setDuration(Number.isFinite(element.duration) ? element.duration : 0);
      restoreProgress();
    };
    const handlePlay = () => setPlaying(true);
    const handlePauseState = () => setPlaying(false);
    const handleDurationChange = () => setDuration(Number.isFinite(element.duration) ? element.duration : 0);
    const handleVolumeChange = () => {
      setVolume(element.volume);
      setMuted(element.muted);
    };
    const handleFullscreenChange = () => setFullscreen(document.fullscreenElement === playerRef.current);
    element.addEventListener("loadedmetadata", handleLoadedMetadata, { once: true });
    element.addEventListener("timeupdate", handleTimeUpdate);
    element.addEventListener("pause", handlePause);
    element.addEventListener("ended", handleEnded);
    element.addEventListener("play", handlePlay);
    element.addEventListener("pause", handlePauseState);
    element.addEventListener("durationchange", handleDurationChange);
    element.addEventListener("volumechange", handleVolumeChange);
    document.addEventListener("fullscreenchange", handleFullscreenChange);
    handleVolumeChange();

    if (!video.hls_url) {
      element.src = video.video_url;
      return () => {
        element.removeEventListener("loadedmetadata", handleLoadedMetadata);
        element.removeEventListener("timeupdate", handleTimeUpdate);
        element.removeEventListener("pause", handlePause);
        element.removeEventListener("ended", handleEnded);
        element.removeEventListener("play", handlePlay);
        element.removeEventListener("pause", handlePauseState);
        element.removeEventListener("durationchange", handleDurationChange);
        element.removeEventListener("volumechange", handleVolumeChange);
        document.removeEventListener("fullscreenchange", handleFullscreenChange);
      };
    }
    if (element.canPlayType("application/vnd.apple.mpegurl")) {
      element.src = video.hls_url;
      setQualities(nativeHLSQualities(video));
      setUsingHLS(true);
      const controller = new AbortController();
      fetch(video.hls_url, { signal: controller.signal })
        .then((response) => response.ok ? response.text() : Promise.reject(new Error("HLS playlist request failed")))
        .then((playlist) => setQualities(parseHLSQualities(video.hls_url, playlist)))
        .catch(() => undefined);
      return () => {
        controller.abort();
        element.removeEventListener("loadedmetadata", handleLoadedMetadata);
        element.removeEventListener("timeupdate", handleTimeUpdate);
        element.removeEventListener("pause", handlePause);
        element.removeEventListener("ended", handleEnded);
        element.removeEventListener("play", handlePlay);
        element.removeEventListener("pause", handlePauseState);
        element.removeEventListener("durationchange", handleDurationChange);
        element.removeEventListener("volumechange", handleVolumeChange);
        document.removeEventListener("fullscreenchange", handleFullscreenChange);
      };
    }
    if (!Hls.isSupported()) {
      element.src = video.video_url;
      return () => {
        element.removeEventListener("loadedmetadata", handleLoadedMetadata);
        element.removeEventListener("timeupdate", handleTimeUpdate);
        element.removeEventListener("pause", handlePause);
        element.removeEventListener("ended", handleEnded);
        element.removeEventListener("play", handlePlay);
        element.removeEventListener("pause", handlePauseState);
        element.removeEventListener("durationchange", handleDurationChange);
        element.removeEventListener("volumechange", handleVolumeChange);
        document.removeEventListener("fullscreenchange", handleFullscreenChange);
      };
    }

    const hls = new Hls({ enableWorker: true, startLevel: -1, backBufferLength: 60 });
    hlsRef.current = hls;
    hls.attachMedia(element);
    hls.on(Hls.Events.MEDIA_ATTACHED, () => hls.loadSource(video.hls_url));
    hls.on(Hls.Events.MANIFEST_PARSED, (_, data) => {
      const available = data.levels
        .map((level, index) => ({ index, height: level.height }))
        .filter((level) => level.height > 0)
        .filter((level, index, all) => all.findIndex((candidate) => candidate.height === level.height) === index)
        .sort((a, b) => a.height - b.height);
      setQualities(available);
      setUsingHLS(true);
    });
    hls.on(Hls.Events.ERROR, (_, data) => {
      if (!data.fatal) return;
      const currentTime = element.currentTime;
      hls.destroy();
      hlsRef.current = null;
      setQualities([]);
      setUsingHLS(false);
      element.src = video.video_url;
      element.currentTime = currentTime;
      void element.play().catch(() => undefined);
    });

    return () => {
      hls.destroy();
      if (hlsRef.current === hls) hlsRef.current = null;
      element.removeEventListener("loadedmetadata", handleLoadedMetadata);
      element.removeEventListener("timeupdate", handleTimeUpdate);
      element.removeEventListener("pause", handlePause);
      element.removeEventListener("ended", handleEnded);
      element.removeEventListener("play", handlePlay);
      element.removeEventListener("pause", handlePauseState);
      element.removeEventListener("durationchange", handleDurationChange);
      element.removeEventListener("volumechange", handleVolumeChange);
      document.removeEventListener("fullscreenchange", handleFullscreenChange);
    };
  }, [video.id, video.hls_url, video.video_url]);

  useEffect(() => {
    if (!qualityMenuOpen && !speedMenuOpen && !subtitleMenuOpen && !settingsOpen) return;
    const closeFromOutside = (event: MouseEvent) => {
      const target = event.target as Node;
      if (!qualityControlRef.current?.contains(target)) setQualityMenuOpen(false);
      if (!speedControlRef.current?.contains(target)) setSpeedMenuOpen(false);
      if (!subtitleControlRef.current?.contains(target)) setSubtitleMenuOpen(false);
      if (!(target instanceof Element && target.closest(".player-settings"))) setSettingsOpen(false);
    };
    const closeFromKeyboard = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setQualityMenuOpen(false);
        setSpeedMenuOpen(false);
        setSubtitleMenuOpen(false);
        setSettingsOpen(false);
      }
    };
    document.addEventListener("click", closeFromOutside);
    document.addEventListener("keydown", closeFromKeyboard);
    return () => {
      document.removeEventListener("click", closeFromOutside);
      document.removeEventListener("keydown", closeFromKeyboard);
    };
  }, [qualityMenuOpen, speedMenuOpen, subtitleMenuOpen, settingsOpen]);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement) return;
      if (event.key === " ") {
        event.preventDefault();
        const element = videoRef.current;
        if (element) void (element.paused ? element.play() : element.pause());
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, []);

  const changeQuality = (value: number) => {
    setSelectedQuality(value);
    setQualityMenuOpen(false);
    if (hlsRef.current) {
      hlsRef.current.currentLevel = value;
      return;
    }
    if (!videoRef.current) return;
    const quality = qualities.find((candidate) => candidate.index === value);
    if (!quality?.url) {
      if (value !== -1 || !video.hls_url) return;
      const currentTime = videoRef.current.currentTime;
      videoRef.current.src = video.hls_url;
      videoRef.current.currentTime = currentTime;
      void videoRef.current.play().catch(() => undefined);
      return;
    }
    const currentTime = videoRef.current.currentTime;
    videoRef.current.src = quality.url;
    videoRef.current.currentTime = currentTime;
    void videoRef.current.play().catch(() => undefined);
  };

  const selectedQualityLabel = selectedQuality === -1
    ? "自动"
    : `${qualities.find((quality) => quality.index === selectedQuality)?.height || ""}p`;
  const qualityOptions = [...qualities].sort((a, b) => b.height - a.height);
  const speedOptions = [2, 1.5, 1, 0.75, 0.5];
  const togglePlay = () => {
    const element = videoRef.current;
    if (!element) return;
    void (element.paused ? element.play() : element.pause());
  };
  const changeSpeed = (value: number) => {
    if (!videoRef.current) return;
    videoRef.current.playbackRate = value;
    setPlaybackRate(value);
    setSpeedMenuOpen(false);
  };
  const toggleMute = () => {
    if (!videoRef.current) return;
    videoRef.current.muted = !videoRef.current.muted;
  };
  const changeVolume = (value: number) => {
    if (!videoRef.current) return;
    videoRef.current.volume = value;
    videoRef.current.muted = value === 0;
  };
  const toggleFullscreen = () => {
    if (!playerRef.current) return;
    if (document.fullscreenElement) void document.exitFullscreen();
    else void playerRef.current.requestFullscreen();
  };
  const togglePictureInPicture = () => {
    const element = videoRef.current as (HTMLVideoElement & { requestPictureInPicture?: () => Promise<unknown> }) | null;
    if (!element || !document.pictureInPictureEnabled) return;
    if (document.pictureInPictureElement) void document.exitPictureInPicture();
    else void element.requestPictureInPicture?.();
  };
  const subtitleTracks: SubtitleTrack[] = video.subtitle_tracks ?? [];
  const subtitleTrackSignature = subtitleTracks.map((track) => `${track.id}:${track.is_default ? 1 : 0}:${track.url}`).join("|");
  useEffect(() => {
    setSelectedSubtitle((current) => {
      if (current === null || subtitleTracks.some((track) => track.id === current)) return current;
      return subtitleTracks.find((track) => track.is_default)?.id ?? null;
    });
  }, [subtitleTrackSignature]);
  useEffect(() => {
    const tracks = videoRef.current?.textTracks;
    if (!tracks) return;
    Array.from(tracks).forEach((track, index) => {
      track.mode = subtitleTracks[index]?.id === selectedSubtitle ? "showing" : "disabled";
    });
  }, [selectedSubtitle, video.id, subtitleTrackSignature]);
  const selectSubtitle = (trackID: number | null) => {
    const tracks = videoRef.current?.textTracks;
    if (tracks) {
      Array.from(tracks).forEach((track, index) => {
        track.mode = video.subtitle_tracks[index]?.id === trackID ? "showing" : "disabled";
      });
    }
    setSelectedSubtitle(trackID);
    setSubtitleMenuOpen(false);
  };

  return (
    <div className={`player-wrap ${theaterMode ? "theater-mode" : ""}`} ref={playerRef}>
      <video ref={videoRef} poster={video.cover_url || undefined} playsInline onClick={togglePlay}>
        {subtitleTracks.map((track) => (
        <track key={track.id} kind="subtitles" src={track.url} srcLang={track.language} label={track.label} default={track.is_default} />
        ))}
      </video>
      {resumeNotice !== null && (
        <div className="resume-notice" role="status">
          <span>已从 {formatDuration(resumeNotice)} 继续播放</span>
          <button type="button" onClick={() => { if (videoRef.current) videoRef.current.currentTime = 0; setResumeNotice(null); }}>从头播放</button>
        </div>
      )}
      <div className="player-controls">
        <div className="player-progress-row">
          <input
            className="player-progress"
            type="range"
            min="0"
            max={duration || 0}
            step="0.1"
            value={Math.min(currentTime, duration || 0)}
            aria-label="播放进度"
            onChange={(event) => {
              const value = Number(event.target.value);
              if (videoRef.current) videoRef.current.currentTime = value;
              setCurrentTime(value);
            }}
          />
        </div>
        <div className="player-control-row">
          <button type="button" className="player-icon-button" onClick={togglePlay} aria-label={playing ? "暂停" : "播放"} title={playing ? "暂停" : "播放"}>
            {playing ? <Pause size={17} fill="currentColor" /> : <Play size={17} fill="currentColor" />}
          </button>
          <span className="player-time">{formatDuration(currentTime)} / {formatDuration(duration)}</span>
          <div className="player-right-controls">
            {usingHLS && qualities.length > 0 && (
              <div className={`quality-control ${qualityMenuOpen ? "open" : ""}`} ref={qualityControlRef}>
          {qualityMenuOpen && (
            <div className="quality-menu" id={`quality-menu-${video.id}`} role="listbox" aria-label="选择视频清晰度">
              {qualityOptions.map((quality) => (
                <button
                  type="button"
                  role="option"
                  aria-selected={selectedQuality === quality.index}
                  className={selectedQuality === quality.index ? "active" : ""}
                  onClick={() => changeQuality(quality.index)}
                  key={quality.index}
                >
                  <Check size={14} />
                  <span>{quality.height}p</span>
                </button>
              ))}
              <button
                type="button"
                role="option"
                aria-selected={selectedQuality === -1}
                className={selectedQuality === -1 ? "active" : ""}
                onClick={() => changeQuality(-1)}
              >
                <Check size={14} />
                <span>自动</span>
              </button>
            </div>
          )}
          <button
            type="button"
            className="quality-trigger"
            aria-label={`选择视频清晰度，当前${selectedQualityLabel}`}
            aria-haspopup="listbox"
            aria-expanded={qualityMenuOpen}
            aria-controls={`quality-menu-${video.id}`}
            onClick={(event) => {
              event.stopPropagation();
              setQualityMenuOpen((open) => !open);
            }}
          >
            <span>{selectedQualityLabel}</span>
            <ChevronUp size={14} aria-hidden="true" />
          </button>
              </div>
            )}
            <div className={`speed-control ${speedMenuOpen ? "open" : ""}`} ref={speedControlRef}>
              {speedMenuOpen && <div className="speed-menu">{speedOptions.map((value) => <button type="button" className={playbackRate === value ? "active" : ""} onClick={() => changeSpeed(value)} key={value}>{value === 1 ? "正常" : `${value}x`}</button>)}</div>}
              <button type="button" className="player-text-button" onClick={(event) => { event.stopPropagation(); setSpeedMenuOpen((open) => !open); setQualityMenuOpen(false); setSettingsOpen(false); }} aria-label="播放速度" title="播放速度">{playbackRate === 1 ? "倍速" : `${playbackRate}x`}</button>
            </div>
            <div className={`subtitle-control-wrap ${subtitleMenuOpen ? "open" : ""}`} ref={subtitleControlRef}>
              {subtitleMenuOpen && (
                <div className="subtitle-menu" role="menu" aria-label="字幕轨道">
                  {subtitleTracks.length === 0 ? (
                    <div className="subtitle-empty" role="status">
                      <strong>暂无字幕</strong>
                      <span>视频作者可在投稿管理中添加 VTT 或 SRT 字幕</span>
                    </div>
                  ) : (
                    <>
                      <button type="button" className={selectedSubtitle === null ? "active" : ""} onClick={() => selectSubtitle(null)}>关闭</button>
                      {subtitleTracks.map((track) => (
                        <button type="button" className={selectedSubtitle === track.id ? "active" : ""} onClick={() => selectSubtitle(track.id)} key={track.id}>{track.label}</button>
                      ))}
                    </>
                  )}
                </div>
              )}
              <button
                type="button"
                className="player-text-button subtitle-control"
                title={subtitleTracks.length === 0 ? "查看字幕状态" : "选择字幕"}
                aria-label={subtitleTracks.length === 0 ? "查看字幕状态" : "选择字幕"}
                aria-haspopup="menu"
                aria-expanded={subtitleMenuOpen}
                onClick={(event) => {
                  event.stopPropagation();
                  setSubtitleMenuOpen((open) => !open);
                  setQualityMenuOpen(false);
                  setSpeedMenuOpen(false);
                  setSettingsOpen(false);
                }}
              >字幕</button>
            </div>
            <div className="volume-control">
              <button type="button" className="player-icon-button" onClick={toggleMute} aria-label={muted || volume === 0 ? "取消静音" : "静音"} title={muted || volume === 0 ? "取消静音" : "静音"}>{muted || volume === 0 ? <VolumeX size={18} /> : <Volume2 size={18} />}</button>
              <input type="range" min="0" max="1" step="0.01" value={muted ? 0 : volume} aria-label="音量" onChange={(event) => changeVolume(Number(event.target.value))} />
            </div>
            <div className="player-settings">
              {settingsOpen && <div className="settings-menu"><strong>播放设置</strong><span>画质 <b>{selectedQualityLabel}</b></span><span>倍速 <b>{playbackRate}x</b></span></div>}
              <button type="button" className="player-icon-button" onClick={(event) => { event.stopPropagation(); setSettingsOpen((open) => !open); setQualityMenuOpen(false); setSpeedMenuOpen(false); }} aria-label="播放设置" title="播放设置"><Settings2 size={18} /></button>
            </div>
            <button type="button" className="player-icon-button theater-control" onClick={() => setTheaterMode((mode) => !mode)} aria-label={theaterMode ? "退出宽屏" : "宽屏模式"} title={theaterMode ? "退出宽屏" : "宽屏模式"}><RectangleHorizontal size={18} /></button>
            <button type="button" className="player-icon-button" onClick={togglePictureInPicture} disabled={!document.pictureInPictureEnabled} aria-label="画中画" title="画中画"><PictureInPicture2 size={18} /></button>
            <button type="button" className="player-icon-button" onClick={toggleFullscreen} aria-label={fullscreen ? "退出全屏" : "全屏"} title={fullscreen ? "退出全屏" : "全屏"}>{fullscreen ? <Minimize2 size={18} /> : <Maximize2 size={18} />}</button>
          </div>
        </div>
      </div>
    </div>
  );
}
