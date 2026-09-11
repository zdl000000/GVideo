import { useEffect, useRef, useState } from "react";
import type Hls from "hls.js";
import { Check, ChevronUp, Maximize2, Minimize2, Pause, PictureInPicture2, Play, RectangleHorizontal, Settings2, Volume2, VolumeX } from "lucide-react";
import { formatDuration } from "../../shared/lib/format";
import { loadHLSModule } from "./hlsLoader";
import type { SubtitleTrack, Video } from "../../types";

const SHORT_SEEK_SECONDS = 5;
const LONG_SEEK_SECONDS = 10;
const VOLUME_STEP = 0.05;

function isInteractiveKeyboardTarget(target: EventTarget | null) {
  return target instanceof Element && Boolean(target.closest(
    "button, a, input, textarea, select, option, [role='button'], [role='textbox'], [role='slider'], [contenteditable]:not([contenteditable='false']), [role='dialog']"
  ));
}

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

export function VideoPlayer({ video, theaterMode, onTheaterModeChange }: { video: Video; theaterMode: boolean; onTheaterModeChange: (enabled: boolean) => void }) {
  const playerRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const hlsRef = useRef<Hls | null>(null);
  const qualityControlRef = useRef<HTMLDivElement>(null);
  const speedControlRef = useRef<HTMLDivElement>(null);
  const subtitleControlRef = useRef<HTMLDivElement>(null);
  const qualityTriggerRef = useRef<HTMLButtonElement>(null);
  const speedTriggerRef = useRef<HTMLButtonElement>(null);
  const subtitleTriggerRef = useRef<HTMLButtonElement>(null);
  const settingsTriggerRef = useRef<HTMLButtonElement>(null);
  const qualityMenuRef = useRef<HTMLDivElement>(null);
  const speedMenuRef = useRef<HTMLDivElement>(null);
  const subtitleMenuRef = useRef<HTMLDivElement>(null);
  const [qualities, setQualities] = useState<PlayerQuality[]>([]);
  const [selectedQuality, setSelectedQuality] = useState(-1);
  const [usingHLS, setUsingHLS] = useState(false);
  const [qualityMenuOpen, setQualityMenuOpen] = useState(false);
  const [speedMenuOpen, setSpeedMenuOpen] = useState(false);
  const [subtitleMenuOpen, setSubtitleMenuOpen] = useState(false);
  const [selectedSubtitle, setSelectedSubtitle] = useState<number | null>(null);
  const lastSelectedSubtitleRef = useRef<number | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [playing, setPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [volume, setVolume] = useState(1);
  const [muted, setMuted] = useState(false);
  const [playbackRate, setPlaybackRate] = useState(1);
  const [fullscreen, setFullscreen] = useState(false);
  const [resumeNotice, setResumeNotice] = useState<number | null>(null);
  const [streamStatus, setStreamStatus] = useState<"idle" | "preparing" | "fallback" | "failed">("idle");
  const [streamAttempt, setStreamAttempt] = useState(0);
  const [shortcutNotice, setShortcutNotice] = useState("");
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

    let cancelled = false;
    let resumeNoticeTimer: number | undefined;
    let playlistController: AbortController | undefined;
    let activeHLS: Hls | null = null;
    let sourceMode: "direct" | "hls" | "fallback" = video.hls_url ? "hls" : "direct";

    hlsRef.current?.destroy();
    hlsRef.current = null;
    setQualities([]);
    setSelectedQuality(-1);
    setUsingHLS(false);
    setQualityMenuOpen(false);
    setSpeedMenuOpen(false);
    setSubtitleMenuOpen(false);
    const defaultSubtitle = (video.subtitle_tracks ?? []).find((track) => track.is_default)?.id ?? null;
    setSelectedSubtitle(defaultSubtitle);
    lastSelectedSubtitleRef.current = defaultSubtitle;
    setSettingsOpen(false);
    setResumeNotice(null);
    setPlaying(false);
    setCurrentTime(0);
    setDuration(0);
    setPlaybackRate(1);
    setStreamStatus(video.hls_url ? "preparing" : "idle");
    lastProgressSaveRef.current = 0;

    const restoreProgress = () => {
      const saved = Number(localStorage.getItem(`gvideo-progress-${video.id}`));
      if (!Number.isFinite(saved) || saved < 5 || (element.duration > 0 && saved >= element.duration - 5)) return;
      element.currentTime = saved;
      setResumeNotice(saved);
      resumeNoticeTimer = window.setTimeout(() => {
        if (!cancelled) setResumeNotice(null);
      }, 3500);
    };
    const fallbackToDirect = () => {
      sourceMode = "fallback";
      setQualities([]);
      setUsingHLS(false);
      setStreamStatus("fallback");
      element.src = video.video_url;
      element.load();
    };
    const handleMediaError = () => {
      if (sourceMode === "hls" && video.video_url) {
        fallbackToDirect();
        return;
      }
      if (sourceMode === "fallback") setStreamStatus("failed");
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
    element.addEventListener("error", handleMediaError);
    element.addEventListener("pause", handlePause);
    element.addEventListener("ended", handleEnded);
    element.addEventListener("play", handlePlay);
    element.addEventListener("pause", handlePauseState);
    element.addEventListener("durationchange", handleDurationChange);
    element.addEventListener("volumechange", handleVolumeChange);
    document.addEventListener("fullscreenchange", handleFullscreenChange);
    handleVolumeChange();

    const cleanup = () => {
      cancelled = true;
      if (resumeNoticeTimer !== undefined) window.clearTimeout(resumeNoticeTimer);
      playlistController?.abort();
      activeHLS?.destroy();
      if (hlsRef.current === activeHLS) hlsRef.current = null;
      element.removeEventListener("loadedmetadata", handleLoadedMetadata);
      element.removeEventListener("timeupdate", handleTimeUpdate);
      element.removeEventListener("error", handleMediaError);
      element.removeEventListener("pause", handlePause);
      element.removeEventListener("ended", handleEnded);
      element.removeEventListener("play", handlePlay);
      element.removeEventListener("pause", handlePauseState);
      element.removeEventListener("durationchange", handleDurationChange);
      element.removeEventListener("volumechange", handleVolumeChange);
      document.removeEventListener("fullscreenchange", handleFullscreenChange);
    };

    if (!video.hls_url) {
      element.src = video.video_url;
      return cleanup;
    }
    if (element.canPlayType("application/vnd.apple.mpegurl")) {
      element.src = video.hls_url;
      setQualities(nativeHLSQualities(video));
      setUsingHLS(true);
      setStreamStatus("idle");
      playlistController = new AbortController();
      fetch(video.hls_url, { signal: playlistController.signal })
        .then((response) => response.ok ? response.text() : Promise.reject(new Error("HLS playlist request failed")))
        .then((playlist) => {
          if (!cancelled) setQualities(parseHLSQualities(video.hls_url, playlist));
        })
        .catch(() => undefined);
      return cleanup;
    }
    void loadHLSModule()
      .then(({ default: Hls }) => {
        if (cancelled) return;
        if (!Hls.isSupported()) {
          fallbackToDirect();
          return;
        }

        const hls = new Hls({ enableWorker: true, startLevel: -1, backBufferLength: 60 });
        if (cancelled) {
          hls.destroy();
          return;
        }
        activeHLS = hls;
        hlsRef.current = hls;
        hls.attachMedia(element);
        hls.on(Hls.Events.MEDIA_ATTACHED, () => {
          if (!cancelled) hls.loadSource(video.hls_url);
        });
        hls.on(Hls.Events.MANIFEST_PARSED, (_, data) => {
          if (cancelled) return;
          const available = data.levels
            .map((level, index) => ({ index, height: level.height }))
            .filter((level) => level.height > 0)
            .filter((level, index, all) => all.findIndex((candidate) => candidate.height === level.height) === index)
            .sort((a, b) => a.height - b.height);
          setQualities(available);
          setUsingHLS(true);
          setStreamStatus("idle");
        });
        hls.on(Hls.Events.ERROR, (_, data) => {
          if (cancelled || !data.fatal) return;
          const currentTime = element.currentTime;
          hls.destroy();
          activeHLS = null;
          if (hlsRef.current === hls) hlsRef.current = null;
          fallbackToDirect();
          element.currentTime = currentTime;
          void element.play().catch(() => undefined);
        });
      })
      .catch(() => {
        if (!cancelled) fallbackToDirect();
      });

    return cleanup;
  }, [video.id, video.hls_url, video.video_url, streamAttempt]);

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
      if (event.key !== "Escape" || event.defaultPrevented) return;
      const returnTarget = qualityMenuOpen ? qualityTriggerRef.current
        : speedMenuOpen ? speedTriggerRef.current
          : subtitleMenuOpen ? subtitleTriggerRef.current
            : settingsOpen ? settingsTriggerRef.current
              : null;
      setQualityMenuOpen(false);
      setSpeedMenuOpen(false);
      setSubtitleMenuOpen(false);
      setSettingsOpen(false);
      window.requestAnimationFrame(() => returnTarget?.focus());
    };
    document.addEventListener("click", closeFromOutside);
    document.addEventListener("keydown", closeFromKeyboard);
    return () => {
      document.removeEventListener("click", closeFromOutside);
      document.removeEventListener("keydown", closeFromKeyboard);
    };
  }, [qualityMenuOpen, speedMenuOpen, subtitleMenuOpen, settingsOpen]);

  const changeQuality = (value: number) => {
    setSelectedQuality(value);
    setQualityMenuOpen(false);
    window.requestAnimationFrame(() => qualityTriggerRef.current?.focus());
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
  const togglePlay = async () => {
    const element = videoRef.current;
    if (!element) return false;
    if (!element.paused) {
      element.pause();
      return true;
    }
    try {
      await element.play();
      return true;
    } catch {
      return false;
    }
  };
  const changeSpeed = (value: number) => {
    if (!videoRef.current) return;
    videoRef.current.playbackRate = value;
    setPlaybackRate(value);
    setSpeedMenuOpen(false);
    window.requestAnimationFrame(() => speedTriggerRef.current?.focus());
  };
  const toggleMute = () => {
    const element = videoRef.current;
    if (!element) return;
    if (element.muted || element.volume === 0) {
      if (element.volume === 0) element.volume = Math.max(volume, VOLUME_STEP);
      element.muted = false;
    } else {
      element.muted = true;
    }
    setVolume(element.volume);
    setMuted(element.muted);
  };
  const changeVolume = (value: number) => {
    const element = videoRef.current;
    if (!element) return;
    const nextVolume = Math.min(1, Math.max(0, value));
    element.volume = nextVolume;
    element.muted = nextVolume === 0;
    setVolume(nextVolume);
    setMuted(element.muted);
  };
  const toggleFullscreen = async (): Promise<"entered" | "exited" | "unavailable" | "failed"> => {
    const player = playerRef.current;
    if (!player) return "unavailable";
    if (document.fullscreenElement === player) {
      if (typeof document.exitFullscreen !== "function") return "unavailable";
      try {
        await document.exitFullscreen();
        return "exited";
      } catch {
        return "failed";
      }
    }
    if (document.fullscreenElement || typeof player.requestFullscreen !== "function") return "unavailable";
    try {
      await player.requestFullscreen();
      return "entered";
    } catch {
      return "failed";
    }
  };
  const togglePictureInPicture = () => {
    const element = videoRef.current as (HTMLVideoElement & { requestPictureInPicture?: () => Promise<unknown> }) | null;
    if (!element || !document.pictureInPictureEnabled) return;
    if (document.pictureInPictureElement) void document.exitPictureInPicture?.().catch(() => undefined);
    else void element.requestPictureInPicture?.().catch(() => undefined);
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
  const openMenu = (
    menu: "quality" | "speed" | "subtitle",
    menuRef: React.RefObject<HTMLDivElement | null>
  ) => {
    const isOpen = menu === "quality" ? qualityMenuOpen : menu === "speed" ? speedMenuOpen : subtitleMenuOpen;
    setQualityMenuOpen(menu === "quality" && !isOpen);
    setSpeedMenuOpen(menu === "speed" && !isOpen);
    setSubtitleMenuOpen(menu === "subtitle" && !isOpen);
    setSettingsOpen(false);
    if (!isOpen) window.requestAnimationFrame(() => menuRef.current?.querySelector<HTMLButtonElement>("button")?.focus());
  };

  const selectSubtitle = (trackID: number | null) => {
    const tracks = videoRef.current?.textTracks;
    if (tracks) {
      Array.from(tracks).forEach((track, index) => {
        track.mode = video.subtitle_tracks[index]?.id === trackID ? "showing" : "disabled";
      });
    }
    if (trackID !== null) lastSelectedSubtitleRef.current = trackID;
    setSelectedSubtitle(trackID);
    setSubtitleMenuOpen(false);
    window.requestAnimationFrame(() => subtitleTriggerRef.current?.focus());
  };

  const handlePlayerKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (
      event.defaultPrevented || event.nativeEvent.isComposing || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey ||
      isInteractiveKeyboardTarget(event.target)
    ) return;

    const closeOpenPanel = () => {
      const returnTarget = qualityMenuOpen ? qualityTriggerRef.current
        : speedMenuOpen ? speedTriggerRef.current
          : subtitleMenuOpen ? subtitleTriggerRef.current
            : settingsOpen ? settingsTriggerRef.current
              : null;
      if (!returnTarget) return false;
      setQualityMenuOpen(false);
      setSpeedMenuOpen(false);
      setSubtitleMenuOpen(false);
      setSettingsOpen(false);
      window.requestAnimationFrame(() => returnTarget.focus());
      return true;
    };
    if (event.key === "Escape" && closeOpenPanel()) {
      event.preventDefault();
      return;
    }

    const element = videoRef.current;
    if (!element) return;
    const seekTo = (value: number) => {
      if (!Number.isFinite(element.duration) || element.duration <= 0) return;
      const nextTime = Math.min(element.duration, Math.max(0, value));
      element.currentTime = nextTime;
      setCurrentTime(nextTime);
      setShortcutNotice(`播放位置 ${formatDuration(nextTime)}`);
    };
    const seekBy = (seconds: number) => seekTo(element.currentTime + seconds);
    const adjustVolume = (delta: number) => {
      const nextVolume = Math.min(1, Math.max(0, element.volume + delta));
      changeVolume(nextVolume);
      setShortcutNotice(`音量 ${Math.round(nextVolume * 100)}%`);
    };
    const isToggleKey = ["Space", "KeyK", "KeyM", "KeyF", "KeyC", "KeyT"].includes(event.code);
    if (event.repeat && isToggleKey) return;

    switch (event.code) {
      case "Space":
      case "KeyK": {
        event.preventDefault();
        const willPlay = element.paused;
        void togglePlay().then((changed) => {
          if (changed) setShortcutNotice(willPlay ? "播放" : "暂停");
          else if (willPlay) setShortcutNotice("无法播放");
        });
        break;
      }
      case "ArrowLeft":
        event.preventDefault();
        seekBy(-SHORT_SEEK_SECONDS);
        break;
      case "ArrowRight":
        event.preventDefault();
        seekBy(SHORT_SEEK_SECONDS);
        break;
      case "KeyJ":
        event.preventDefault();
        seekBy(-LONG_SEEK_SECONDS);
        break;
      case "KeyL":
        event.preventDefault();
        seekBy(LONG_SEEK_SECONDS);
        break;
      case "Home":
        event.preventDefault();
        seekTo(0);
        break;
      case "End":
        event.preventDefault();
        seekTo(element.duration);
        break;
      case "ArrowUp":
        event.preventDefault();
        adjustVolume(VOLUME_STEP);
        break;
      case "ArrowDown":
        event.preventDefault();
        adjustVolume(-VOLUME_STEP);
        break;
      case "KeyM": {
        event.preventDefault();
        const willUnmute = element.muted || element.volume === 0;
        toggleMute();
        setShortcutNotice(willUnmute ? "已取消静音" : "已静音");
        break;
      }
      case "KeyF":
        event.preventDefault();
        void toggleFullscreen().then((result) => {
          if (result === "entered") setShortcutNotice("已进入全屏");
          else if (result === "exited") setShortcutNotice("已退出全屏");
          else if (result === "unavailable") setShortcutNotice("当前无法切换全屏");
          else setShortcutNotice("全屏切换失败");
        });
        break;
      case "KeyT":
        event.preventDefault();
        onTheaterModeChange(!theaterMode);
        setShortcutNotice(theaterMode ? "已退出宽屏模式" : "已进入宽屏模式");
        break;
      case "KeyC": {
        event.preventDefault();
        if (subtitleTracks.length === 0) {
          setShortcutNotice("此视频暂无字幕");
          break;
        }
        const rememberedSubtitle = subtitleTracks.find((track) => track.id === lastSelectedSubtitleRef.current);
        const nextSubtitle = selectedSubtitle === null
          ? (rememberedSubtitle ?? subtitleTracks.find((track) => track.is_default) ?? subtitleTracks[0]).id
          : null;
        selectSubtitle(nextSubtitle);
        setShortcutNotice(nextSubtitle === null ? "字幕已关闭" : "字幕已开启");
        break;
      }
    }
  };

  return (
    <div
      className={`player-wrap ${theaterMode ? "theater-mode" : ""}`}
      ref={playerRef}
      role="region"
      tabIndex={0}
      aria-label={`${video.title} 视频播放器`}
      aria-keyshortcuts="Space K ArrowLeft ArrowRight J L Home End ArrowUp ArrowDown M C T F"
      onKeyDown={handlePlayerKeyDown}
      aria-describedby={`player-keyboard-help-${video.id}`}
      aria-busy={streamStatus === "preparing"}
    >
      <p id={`player-keyboard-help-${video.id}`} className="sr-only">空格或 K 播放暂停，左右方向键快退快进 5 秒，J 或 L 快退快进 10 秒，上下方向键调节音量，M 静音，C 字幕，T 宽屏，F 全屏。</p>
      <div className="sr-only" data-testid="player-shortcut-status" aria-live="polite" aria-atomic="true">{shortcutNotice}</div>
      <video ref={videoRef} poster={video.cover_url || undefined} playsInline onClick={() => { playerRef.current?.focus(); togglePlay(); }}>
        {subtitleTracks.map((track) => (
        <track key={track.id} kind="subtitles" src={track.url} srcLang={track.language} label={track.label} default={track.is_default} />
        ))}
      </video>
      {streamStatus !== "idle" && (
        <div className={`stream-status stream-status-${streamStatus}`} role="status" aria-live="polite">
          <span>{streamStatus === "preparing" ? "正在准备高清流" : streamStatus === "fallback" ? "高清流不可用，已切换原始视频" : "视频加载失败，请重试"}</span>
          {streamStatus === "failed" && <button type="button" onClick={() => setStreamAttempt((attempt) => attempt + 1)}>重试播放</button>}
        </div>
      )}
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
            aria-valuetext={`${formatDuration(currentTime)} / ${formatDuration(duration)}`}
            onChange={(event) => {
              const value = Number(event.target.value);
              if (videoRef.current) videoRef.current.currentTime = value;
              setCurrentTime(value);
            }}
          />
        </div>
        <div className="player-control-row">
          <button type="button" className="player-icon-button" onClick={togglePlay} aria-label={playing ? "暂停" : "播放"} title={playing ? "暂停（空格或 K）" : "播放（空格或 K）"}>
            {playing ? <Pause size={17} fill="currentColor" /> : <Play size={17} fill="currentColor" />}
          </button>
          <span className="player-time">{formatDuration(currentTime)} / {formatDuration(duration)}</span>
          <div className="player-right-controls">
            {usingHLS && qualities.length > 0 && (
              <div className={`quality-control ${qualityMenuOpen ? "open" : ""}`} ref={qualityControlRef}>
          {qualityMenuOpen && (
            <div ref={qualityMenuRef} className="quality-menu" id={`quality-menu-${video.id}`} role="group" aria-label="选择视频清晰度">
              {qualityOptions.map((quality) => (
                <button
                  type="button"
                  aria-pressed={selectedQuality === quality.index}
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
                aria-pressed={selectedQuality === -1}
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
            ref={qualityTriggerRef}
            className="quality-trigger"
            aria-label={`选择视频清晰度，当前${selectedQualityLabel}`}
            aria-expanded={qualityMenuOpen}
            aria-controls={`quality-menu-${video.id}`}
            onClick={(event) => {
              event.stopPropagation();
              openMenu("quality", qualityMenuRef);
            }}
          >
            <span>{selectedQualityLabel}</span>
            <ChevronUp size={14} aria-hidden="true" />
          </button>
              </div>
            )}
            <div className={`speed-control ${speedMenuOpen ? "open" : ""}`} ref={speedControlRef}>
              {speedMenuOpen && <div ref={speedMenuRef} className="speed-menu" id={`speed-menu-${video.id}`} role="group" aria-label="选择播放速度">{speedOptions.map((value) => <button type="button" aria-pressed={playbackRate === value} className={playbackRate === value ? "active" : ""} onClick={() => changeSpeed(value)} key={value}>{value === 1 ? "正常" : `${value}x`}</button>)}</div>}
              <button type="button" ref={speedTriggerRef} className="player-text-button" onClick={(event) => { event.stopPropagation(); openMenu("speed", speedMenuRef); }} aria-label="播放速度" aria-expanded={speedMenuOpen} aria-controls={`speed-menu-${video.id}`} title="播放速度">{playbackRate === 1 ? "倍速" : `${playbackRate}x`}</button>
            </div>
            <div className={`subtitle-control-wrap ${subtitleMenuOpen ? "open" : ""}`} ref={subtitleControlRef}>
              {subtitleMenuOpen && (
                <div ref={subtitleMenuRef} className="subtitle-menu" id={`subtitle-menu-${video.id}`} role="group" aria-label="字幕轨道">
                  {subtitleTracks.length === 0 ? (
                    <div className="subtitle-empty" role="status">
                      <strong>暂无字幕</strong>
                      <span>视频作者可在投稿管理中添加 VTT 或 SRT 字幕</span>
                    </div>
                  ) : (
                    <>
                      <button type="button" aria-pressed={selectedSubtitle === null} className={selectedSubtitle === null ? "active" : ""} onClick={() => selectSubtitle(null)}>关闭</button>
                      {subtitleTracks.map((track) => (
                        <button type="button" aria-pressed={selectedSubtitle === track.id} className={selectedSubtitle === track.id ? "active" : ""} onClick={() => selectSubtitle(track.id)} key={track.id}>{track.label}</button>
                      ))}
                    </>
                  )}
                </div>
              )}
              <button
                type="button"
                ref={subtitleTriggerRef}
                className="player-text-button subtitle-control"
                title={subtitleTracks.length === 0 ? "查看字幕状态" : "选择字幕"}
                aria-label={subtitleTracks.length === 0 ? "查看字幕状态" : "选择字幕"}
                aria-expanded={subtitleMenuOpen}
                aria-controls={`subtitle-menu-${video.id}`}
                aria-pressed={selectedSubtitle !== null}

                onClick={(event) => {
                  event.stopPropagation();
                  openMenu("subtitle", subtitleMenuRef);
                }}
              >字幕</button>
            </div>
            <div className="volume-control">
              <button type="button" className="player-icon-button" onClick={toggleMute} aria-label={muted || volume === 0 ? "取消静音" : "静音"} title={muted || volume === 0 ? "取消静音（M）" : "静音（M）"}>{muted || volume === 0 ? <VolumeX size={18} /> : <Volume2 size={18} />}</button>
              <input type="range" min="0" max="1" step="0.01" value={muted ? 0 : volume} aria-label="音量" onChange={(event) => changeVolume(Number(event.target.value))} />
            </div>
            <div className="player-settings">
              {settingsOpen && <div className="settings-menu" id={`settings-panel-${video.id}`}><strong>播放设置</strong><span>画质 <b>{selectedQualityLabel}</b></span><span>倍速 <b>{playbackRate}x</b></span></div>}
              <button type="button" ref={settingsTriggerRef} className="player-icon-button" onClick={(event) => { event.stopPropagation(); setSettingsOpen((open) => !open); setQualityMenuOpen(false); setSpeedMenuOpen(false); setSubtitleMenuOpen(false); }} aria-label="播放设置" aria-expanded={settingsOpen} aria-controls={`settings-panel-${video.id}`} title="播放设置"><Settings2 size={18} /></button>
            </div>
            <button type="button" className="player-icon-button theater-control" onClick={() => onTheaterModeChange(!theaterMode)} aria-label="宽屏模式" aria-pressed={theaterMode} title={theaterMode ? "退出宽屏（T）" : "宽屏模式（T）"}><RectangleHorizontal size={18} /></button>
            <button type="button" className="player-icon-button" onClick={togglePictureInPicture} disabled={!document.pictureInPictureEnabled} aria-label="画中画" title="画中画"><PictureInPicture2 size={18} /></button>
            <button type="button" className="player-icon-button" onClick={() => { void toggleFullscreen(); }} disabled={typeof playerRef.current?.requestFullscreen !== "function" && !fullscreen} aria-label={fullscreen ? "退出全屏" : "全屏"} title={fullscreen ? "退出全屏（F）" : "全屏（F）"}>{fullscreen ? <Minimize2 size={18} /> : <Maximize2 size={18} />}</button>
          </div>
        </div>
      </div>
    </div>
  );
}
