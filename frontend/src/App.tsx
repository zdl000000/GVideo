import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import Hls from "hls.js";
import {
  Bookmark,
  Check,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  Clock3,
  Compass,
  CircleAlert,
  CircleCheck,
  Eye,
  Film,
  Heart,
  History,
  Home,
  ImagePlus,
  LogIn,
  LogOut,
  Menu,
  MessageCircle,
  Moon,
  Maximize2,
  Minimize2,
  Pause,
  Pencil,
  PictureInPicture2,
  Play,
  RectangleHorizontal,
  RefreshCw,
  Search,
  Send,
  Share2,
  Settings2,
  Sparkles,
  Sun,
  Trash2,
  Upload,
  UserRound,
  Users,
  Volume2,
  VolumeX,
  X
} from "lucide-react";
import { Link, NavLink, Navigate, Outlet, Route, Routes, useLocation, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { ApiError, api, setCSRFToken } from "./api";
import type { AuthPayload, Comment, CreatorProfile, SubtitleTrack, User, Video, VideoPage as VideoPageData } from "./types";

type AuthMode = "login" | "register";
type Theme = "light" | "dark";

interface AuthState {
  user: User | null;
  loading: boolean;
}

const formatCount = (value: number) =>
  new Intl.NumberFormat("zh-CN", { notation: value >= 10000 ? "compact" : "standard", maximumFractionDigits: 1 }).format(value);

const formatDuration = (seconds: number) => {
  if (!seconds) return "--:--";
  const minutes = Math.floor(seconds / 60);
  const rest = Math.floor(seconds % 60);
  return `${minutes}:${rest.toString().padStart(2, "0")}`;
};

const formatDate = (value: string) =>
  new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }).format(new Date(value));

const formatJoinDate = (value: string) =>
  new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "long", day: "numeric" }).format(new Date(value));

const formatFileSize = (bytes: number) => bytes >= 1024 * 1024
  ? `${(bytes / 1024 / 1024).toFixed(1)} MB`
  : `${Math.max(1, Math.round(bytes / 1024))} KB`;

const pageFrom = (params: URLSearchParams) => Math.max(1, Number.parseInt(params.get("page") || "1", 10) || 1);

const errorMessage = (error: unknown) => (error instanceof Error ? error.message : "请求失败，请稍后重试");

interface PlayerQuality {
  index: number;
  height: number;
  url?: string;
}

function parseHLSQualities(masterURL: string, playlist: string): PlayerQuality[] {
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

function nativeHLSQualities(video: Video): PlayerQuality[] {
  const masterURL = new URL(video.hls_url, window.location.href);
  const levels = [360, 480, 720].filter((height) => video.source_height >= height);
  return levels.map((height, index) => ({
    index,
    height,
    url: new URL(`./${height}p/index.m3u8`, masterURL).toString()
  }));
}

function VideoPlayer({ video }: { video: Video }) {
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
  useEffect(() => {
    const tracks = videoRef.current?.textTracks;
    if (!tracks) return;
    Array.from(tracks).forEach((track, index) => {
      track.mode = subtitleTracks[index]?.id === selectedSubtitle ? "showing" : "disabled";
    });
  }, [selectedSubtitle, video.id, subtitleTracks.length]);
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
                      <span>视频作者可以在下方上传 VTT 或 SRT 字幕</span>
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

function App() {
  const [auth, setAuth] = useState<AuthState>({ user: null, loading: true });
  const [theme, setTheme] = useState<Theme>(() => {
    const saved = localStorage.getItem("gvideo-theme");
    if (saved === "light" || saved === "dark") return saved;
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  });

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
    document.querySelector('meta[name="theme-color"]')?.setAttribute("content", theme === "dark" ? "#15171b" : "#f7f8fa");
    localStorage.setItem("gvideo-theme", theme);
  }, [theme]);

  useEffect(() => {
    api.me()
      .then((payload) => applyAuth(payload))
      .catch((error) => {
        if (!(error instanceof ApiError && error.status === 401)) console.error(error);
        setAuth({ user: null, loading: false });
      });
  }, []);

  const applyAuth = (payload: AuthPayload) => {
    setCSRFToken(payload.csrf_token);
    setAuth({ user: payload.user, loading: false });
  };

  const clearAuth = () => {
    setCSRFToken("");
    setAuth({ user: null, loading: false });
  };

  return (
    <Routes>
      <Route element={<Shell auth={auth} onLogout={clearAuth} theme={theme} onThemeChange={() => setTheme((current) => current === "light" ? "dark" : "light")} />}>
        <Route index element={<HomeRoute />} />
        <Route path="latest" element={<LatestPage />} />
        <Route path="popular" element={<PopularPage />} />
        <Route path="following" element={<Protected auth={auth}><FollowingPage /></Protected>} />
        <Route path="users/:id" element={<CreatorPage user={auth.user} />} />
        <Route path="video/:id" element={<VideoPage user={auth.user} />} />
        <Route path="auth" element={auth.user ? <Navigate to="/" replace /> : <AuthPage onAuth={applyAuth} />} />
        <Route path="upload" element={<Protected auth={auth}><UploadPage /></Protected>} />
        <Route path="me/videos" element={<Protected auth={auth}><MyVideosPage /></Protected>} />
        <Route path="*" element={<NotFound />} />
      </Route>
    </Routes>
  );
}

function Protected({ auth, children }: { auth: AuthState; children: React.ReactNode }) {
  const location = useLocation();
  if (auth.loading) return <LoadingBlock label="正在确认登录状态" />;
  if (!auth.user) return <Navigate to={`/auth?next=${encodeURIComponent(`${location.pathname}${location.search}`)}`} replace />;
  return children;
}

function Shell({ auth, onLogout, theme, onThemeChange }: { auth: AuthState; onLogout: () => void; theme: Theme; onThemeChange: () => void }) {
  const navigate = useNavigate();
  const location = useLocation();
  const [menuOpen, setMenuOpen] = useState(false);
  const [query, setQuery] = useState("");

  useEffect(() => setMenuOpen(false), [location.pathname, location.search]);

  const submitSearch = (event: FormEvent) => {
    event.preventDefault();
    navigate(query.trim() ? `/?q=${encodeURIComponent(query.trim())}` : "/");
  };

  const logout = async () => {
    await api.logout().catch(console.error);
    setMenuOpen(false);
    onLogout();
    navigate("/");
  };

  if (location.pathname === "/auth") {
    return (
      <div className="auth-shell">
        <header className="auth-topbar">
          <Link to="/" className="brand" aria-label="GVideo 首页">
            <span className="brand-mark"><Play size={17} fill="currentColor" /></span>
            <span>GVideo</span>
          </Link>
          <ThemeButton theme={theme} onChange={onThemeChange} />
        </header>
        <main className="auth-main">
          <Outlet />
        </main>
      </div>
    );
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        <Link to="/" className="brand" aria-label="GVideo 首页">
          <span className="brand-mark"><Play size={17} fill="currentColor" /></span>
          <span>GVideo</span>
        </Link>
        <form className="global-search" onSubmit={submitSearch}>
          <Search size={18} />
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索视频、创作者" aria-label="搜索" />
          <button type="submit" className="icon-button" title="搜索"><Search size={18} /></button>
        </form>
        <nav className="desktop-actions" aria-label="用户操作">
          <ThemeButton theme={theme} onChange={onThemeChange} />
          {auth.user ? (
            <>
              <Link to={`/users/${auth.user.id}`} className="user-chip"><span>{auth.user.username.slice(0, 1).toUpperCase()}</span>{auth.user.username}</Link>
              <Link to="/upload" className="primary-button compact"><Upload size={17} />投稿</Link>
              <button className="icon-button" onClick={logout} title="退出登录"><LogOut size={19} /></button>
            </>
          ) : (
            <Link to="/auth" className="primary-button compact"><LogIn size={17} />登录</Link>
          )}
        </nav>
        <button className="mobile-menu-button icon-button" onClick={() => setMenuOpen((open) => !open)} title={menuOpen ? "关闭菜单" : "打开菜单"} aria-expanded={menuOpen} aria-controls="mobile-sidebar">
          {menuOpen ? <X size={21} /> : <Menu size={21} />}
        </button>
      </header>

      <button className={`sidebar-scrim ${menuOpen ? "open" : ""}`} onClick={() => setMenuOpen(false)} aria-label="关闭菜单" tabIndex={menuOpen ? 0 : -1} />

      <aside id="mobile-sidebar" className={`sidebar ${menuOpen ? "open" : ""}`}>
        <nav aria-label="主导航">
          <NavItem to="/" icon={<Home size={19} />} label="首页" end />
          <NavItem to="/popular" icon={<Compass size={19} />} label="热门发现" />
          {auth.user && <NavItem to="/following" icon={<Users size={19} />} label="关注动态" />}
          <NavItem to={auth.user ? "/me/videos" : "/auth?next=/me/videos"} icon={<Film size={19} />} label="我的投稿" />
          <NavItem to={auth.user ? "/upload" : "/auth?next=/upload"} icon={<Upload size={19} />} label="发布视频" />
        </nav>
        <div className="mobile-account">
          <button className="theme-row" onClick={onThemeChange} aria-label={theme === "light" ? "切换到深色主题" : "切换到浅色主题"}>
            <span>{theme === "light" ? <Moon size={18} /> : <Sun size={18} />}{theme === "light" ? "深色主题" : "浅色主题"}</span>
            <span className={`theme-switch ${theme === "dark" ? "active" : ""}`} aria-hidden="true"><i /></span>
          </button>
          {auth.user ? (
            <>
              <Link to={`/users/${auth.user.id}`} className="user-chip"><span>{auth.user.username.slice(0, 1).toUpperCase()}</span><b>{auth.user.username}</b></Link>
              <button className="secondary-button" onClick={logout}><LogOut size={17} />退出登录</button>
            </>
          ) : (
            <Link to="/auth" className="primary-button"><LogIn size={17} />登录或注册</Link>
          )}
        </div>
        <div className="sidebar-note">
          <span><Sparkles size={16} />创作提示</span>
          <p>标题清楚、封面准确，更容易让观众找到你。</p>
        </div>
      </aside>

      <main className="main-content">
        <Outlet />
      </main>
    </div>
  );
}

function ThemeButton({ theme, onChange }: { theme: Theme; onChange: () => void }) {
  const nextLabel = theme === "light" ? "切换到深色主题" : "切换到浅色主题";
  return <button className="icon-button theme-button" onClick={onChange} title={nextLabel} aria-label={nextLabel}>{theme === "light" ? <Moon size={19} /> : <Sun size={19} />}</button>;
}

function NavItem({ to, icon, label, end = false }: { to: string; icon: React.ReactNode; label: string; end?: boolean }) {
  const location = useLocation();
  const currentParams = new URLSearchParams(location.search);
  const [, targetSearch = ""] = to.split("?", 2);
  const targetParams = new URLSearchParams(targetSearch);
  const targetNext = targetParams.get("next");
  const queryAwareActive = targetNext
    ? location.pathname === "/auth" && currentParams.get("next") === targetNext
    : null;

  return <NavLink to={to} end={end} className={({ isActive }) => `nav-item ${(queryAwareActive ?? isActive) ? "active" : ""}`}>{icon}<span>{label}</span></NavLink>;
}

function HomeRoute() {
  const [searchParams] = useSearchParams();
  const sort = searchParams.get("sort");
  if (sort !== "popular" && sort !== "latest") return <HomePage />;

  const next = new URLSearchParams(searchParams);
  next.delete("sort");
  const query = next.toString();
  return <Navigate to={`/${sort}${query ? `?${query}` : ""}`} replace />;
}

function HomePage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [result, setResult] = useState<VideoPageData>({ items: [], page: 1, page_size: 36, total: 0, has_next: false });
  const [categories, setCategories] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const query = searchParams.get("q") || "";
  const category = searchParams.get("category") || "";
  const page = pageFrom(searchParams);
  const videos = result.items;

  useEffect(() => {
    api.categories().then(setCategories).catch(console.error);
  }, []);

  useEffect(() => {
    setLoading(true);
    setError("");
    const params = new URLSearchParams({ sort: "latest", page: String(page), page_size: "36" });
    if (query) params.set("q", query);
    if (category) params.set("category", category);
    api.videos(params).then(setResult).catch((err) => setError(errorMessage(err))).finally(() => setLoading(false));
  }, [query, category, page]);

  const featured = page === 1 ? videos.slice(0, 5) : [];
  const leadVideo = featured[0];
  const secondaryVideos = featured.slice(1);
  const setFilter = (key: string, value: string) => {
    const next = new URLSearchParams(searchParams);
    value ? next.set(key, value) : next.delete(key);
    next.delete("page");
    setSearchParams(next);
  };
  const setPage = (value: number) => {
    const next = new URLSearchParams(searchParams);
    value > 1 ? next.set("page", String(value)) : next.delete("page");
    setSearchParams(next);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };
  const popularParams = new URLSearchParams(searchParams);
  popularParams.delete("sort");
  popularParams.delete("page");
  const popularTarget = `/popular${popularParams.size ? `?${popularParams}` : ""}`;
  const latestTarget = `/latest${popularParams.size ? `?${popularParams}` : ""}`;

  return (
    <div className="page home-page">
      {query || category ? (
        <section className="discovery-head compact-heading">
          <div><p className="eyebrow">内容检索</p><h1>{query ? `“${query}”的搜索结果` : category}</h1></div>
          <div className="head-stat"><strong>{result.total}</strong><span>条内容</span></div>
        </section>
      ) : leadVideo ? (
        <section className={`featured-showcase ${secondaryVideos.length ? "" : "single"}`} aria-label="推荐视频">
          <article className="featured-lead">
            <Link to={`/video/${leadVideo.id}`} className="featured-click-target" aria-label={`播放 ${leadVideo.title}`} />
            {leadVideo.cover_url ? <img src={leadVideo.cover_url} alt="" /> : <div className="cover-fallback"><Play size={42} fill="currentColor" /></div>}
            <span className="featured-shade" />
            <span className="featured-badge">今日推荐</span>
            <span className="featured-lead-copy">
              <Link to={`/video/${leadVideo.id}`} className="featured-lead-title">{leadVideo.title}</Link>
              <span><Link to={`/users/${leadVideo.user_id}`} className="featured-author-link">{leadVideo.username}</Link><span><Eye size={14} />{formatCount(leadVideo.views_count)}</span><span><MessageCircle size={14} />{formatCount(leadVideo.comments_count)}</span></span>
            </span>
            <span className="duration">{formatDuration(leadVideo.duration_seconds)}</span>
          </article>
          {secondaryVideos.length > 0 && (
            <div className="featured-secondary">
              {secondaryVideos.map((video) => (
                <article className="featured-mini" key={video.id}>
                  <Link to={`/video/${video.id}`} className="featured-mini-cover">
                    {video.cover_url ? <img src={video.cover_url} alt="" loading="lazy" /> : <div className="cover-fallback"><Play size={24} fill="currentColor" /></div>}
                    <span className="featured-mini-stats"><span><Eye size={13} />{formatCount(video.views_count)}</span><span><MessageCircle size={13} />{formatCount(video.comments_count)}</span></span>
                    <span className="duration">{formatDuration(video.duration_seconds)}</span>
                  </Link>
                  <Link to={`/video/${video.id}`} className="featured-mini-title">{video.title}</Link>
                  <span className="featured-mini-author"><Link to={`/users/${video.user_id}`}>{video.username}</Link> · {video.category}</span>
                </article>
              ))}
            </div>
          )}
        </section>
      ) : null}

      <section className="content-heading">
        <div><h2>{query || category ? "筛选结果" : "视频推荐"}</h2><span>共 {result.total} 条视频</span></div>
      </section>

      <section className="filter-row" aria-label="视频筛选">
        <div className="category-scroller">
          <button className={!category ? "active" : ""} onClick={() => setFilter("category", "")}>全部</button>
          {categories.map((item) => <button className={category === item ? "active" : ""} key={item} onClick={() => setFilter("category", item)}>{item}</button>)}
        </div>
        <div className="segmented-control">
          <Link to={latestTarget}><Clock3 size={15} />最新</Link>
          <Link to={popularTarget}><Sparkles size={15} />热门</Link>
        </div>
      </section>

      {loading ? <LoadingGrid /> : error ? <ErrorBlock message={error} /> : videos.length ? (
        <><section className="video-grid">{videos.map((video) => <VideoCard video={video} key={video.id} />)}</section><Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={setPage} /></>
      ) : (
        <EmptyState icon={<Film size={28} />} title="这里还没有视频" text={query || category ? "换个关键词或分类继续找找。" : "成为第一个发布作品的人。"} action={<Link to="/upload" className="primary-button"><Upload size={17} />发布视频</Link>} />
      )}
    </div>
  );
}

function LatestPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [result, setResult] = useState<VideoPageData>({ items: [], page: 1, page_size: 36, total: 0, has_next: false });
  const [categories, setCategories] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const query = searchParams.get("q") || "";
  const category = searchParams.get("category") || "";
  const page = pageFrom(searchParams);
  const videos = result.items;

  useEffect(() => {
    api.categories().then(setCategories).catch(console.error);
  }, []);

  useEffect(() => {
    setLoading(true);
    setError("");
    const params = new URLSearchParams({ sort: "latest", page: String(page), page_size: "36" });
    if (query) params.set("q", query);
    if (category) params.set("category", category);
    api.videos(params).then(setResult).catch((err) => setError(errorMessage(err))).finally(() => setLoading(false));
  }, [query, category, page]);

  const setCategory = (value: string) => {
    const next = new URLSearchParams(searchParams);
    value ? next.set("category", value) : next.delete("category");
    next.delete("sort");
    next.delete("page");
    setSearchParams(next);
  };
  const setPage = (value: number) => {
    const next = new URLSearchParams(searchParams);
    value > 1 ? next.set("page", String(value)) : next.delete("page");
    setSearchParams(next);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };
  const popularParams = new URLSearchParams(searchParams);
  popularParams.delete("sort");
  popularParams.delete("page");
  const popularTarget = `/popular${popularParams.size ? `?${popularParams}` : ""}`;

  return (
    <div className="page latest-page">
      <section className="latest-head">
        <div>
          <p className="eyebrow">最新发布</p>
          <h1>{query ? `“${query}”的最新内容` : category ? `${category}最新内容` : "刚刚与大家见面"}</h1>
          <p>按发布时间倒序排列，不错过社区里的新作品。</p>
        </div>
        <div className="latest-rule" aria-label="最新排序规则">
          <Clock3 size={20} />
          <span><strong>更新时间线</strong>最近发布优先展示</span>
        </div>
      </section>

      <section className="content-heading latest-title-row">
        <div><h2>最新视频</h2><span>共 {result.total} 条视频</span></div>
      </section>

      <section className="filter-row" aria-label="最新视频筛选">
        <div className="category-scroller">
          <button className={!category ? "active" : ""} onClick={() => setCategory("")}>全部</button>
          {categories.map((item) => <button className={category === item ? "active" : ""} key={item} onClick={() => setCategory(item)}>{item}</button>)}
        </div>
        <div className="segmented-control">
          <Link className="active" to={`/latest${searchParams.size ? `?${searchParams}` : ""}`}><Clock3 size={15} />最新</Link>
          <Link to={popularTarget}><Sparkles size={15} />热门</Link>
        </div>
      </section>

      {loading ? <LoadingGrid /> : error ? <ErrorBlock message={error} /> : videos.length ? (
        <><section className="video-grid">{videos.map((video) => <VideoCard video={video} key={video.id} />)}</section><Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={setPage} /></>
      ) : (
        <EmptyState icon={<Clock3 size={28} />} title="暂无最新内容" text={query || category ? "换个关键词或分区继续看看。" : "新作品发布后，会按时间出现在这里。"} action={<Link to="/upload" className="primary-button"><Upload size={17} />发布视频</Link>} />
      )}
    </div>
  );
}

function PopularPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [result, setResult] = useState<VideoPageData>({ items: [], page: 1, page_size: 36, total: 0, has_next: false });
  const [categories, setCategories] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const query = searchParams.get("q") || "";
  const category = searchParams.get("category") || "";
  const page = pageFrom(searchParams);
  const videos = result.items;

  useEffect(() => {
    api.categories().then(setCategories).catch(console.error);
  }, []);

  useEffect(() => {
    setLoading(true);
    setError("");
    const params = new URLSearchParams({ sort: "popular", page: String(page), page_size: "36" });
    if (query) params.set("q", query);
    if (category) params.set("category", category);
    api.videos(params).then(setResult).catch((err) => setError(errorMessage(err))).finally(() => setLoading(false));
  }, [query, category, page]);

  const setCategory = (value: string) => {
    const next = new URLSearchParams(searchParams);
    value ? next.set("category", value) : next.delete("category");
    next.delete("sort");
    next.delete("page");
    setSearchParams(next);
  };
  const setPage = (value: number) => {
    const next = new URLSearchParams(searchParams);
    value > 1 ? next.set("page", String(value)) : next.delete("page");
    setSearchParams(next);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };
  const latestParams = new URLSearchParams(searchParams);
  latestParams.delete("sort");
  latestParams.delete("page");
  const latestTarget = `/latest${latestParams.size ? `?${latestParams}` : ""}`;

  return (
    <div className="page popular-page">
      <section className="popular-head">
        <div>
          <p className="eyebrow">热门发现</p>
          <h1>{query ? `“${query}”的热门内容` : category ? `${category}热门内容` : "此刻最受关注"}</h1>
          <p>按播放量与点赞综合排序，发现社区里更受关注的作品。</p>
        </div>
        <div className="popular-rule" aria-label="热门排序规则">
          <Sparkles size={20} />
          <span><strong>热度榜</strong>播放与点赞综合排序</span>
        </div>
      </section>

      <section className="content-heading popular-title-row">
        <div><h2>热门榜单</h2><span>共 {result.total} 条视频</span></div>
      </section>

      <section className="filter-row" aria-label="热门视频筛选">
        <div className="category-scroller">
          <button className={!category ? "active" : ""} onClick={() => setCategory("")}>全部</button>
          {categories.map((item) => <button className={category === item ? "active" : ""} key={item} onClick={() => setCategory(item)}>{item}</button>)}
        </div>
        <div className="segmented-control">
          <Link to={latestTarget}><Clock3 size={15} />最新</Link>
          <Link className="active" to={`/popular${searchParams.size ? `?${searchParams}` : ""}`}><Sparkles size={15} />热门</Link>
        </div>
      </section>

      {loading ? <LoadingGrid /> : error ? <ErrorBlock message={error} /> : videos.length ? (
        <><section className="video-grid popular-grid">
          {videos.map((video, index) => { const rank = (result.page - 1) * result.page_size + index + 1; return <div className="ranked-video" key={video.id}><span className={`rank-number ${rank <= 3 ? "top" : ""}`}>{String(rank).padStart(2, "0")}</span><VideoCard video={video} /></div>; })}
        </section><Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={setPage} /></>
      ) : (
        <EmptyState icon={<Compass size={28} />} title="暂无热门内容" text={query || category ? "换个关键词或分区继续看看。" : "作品获得播放和点赞后，会出现在这里。"} action={<Link to="/" className="secondary-button"><Home size={17} />返回首页</Link>} />
      )}
    </div>
  );
}

function FollowingPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [result, setResult] = useState<VideoPageData>({ items: [], page: 1, page_size: 24, total: 0, has_next: false });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const page = pageFrom(searchParams);

  useEffect(() => {
    setLoading(true);
    setError("");
    api.followingVideos(new URLSearchParams({ page: String(page), page_size: "24" }))
      .then(setResult)
      .catch((err) => setError(errorMessage(err)))
      .finally(() => setLoading(false));
  }, [page]);

  const setPage = (value: number) => {
    const next = new URLSearchParams(searchParams);
    value > 1 ? next.set("page", String(value)) : next.delete("page");
    setSearchParams(next);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  return (
    <div className="page following-page">
      <section className="following-head">
        <div><p className="eyebrow">关注动态</p><h1>关注的人，刚刚发布</h1><p>按发布时间查看你关注的创作者新作。</p></div>
        <div className="following-rule"><Users size={20} /><span><strong>{result.total} 条动态</strong>只展示已关注作者的投稿</span></div>
      </section>
      <section className="content-heading following-title-row"><div><h2>最新投稿</h2><span>服务端分页 · 每页 24 条</span></div></section>
      {loading ? <LoadingGrid /> : error ? <ErrorBlock message={error} /> : result.items.length ? (
        <><section className="video-grid">{result.items.map((video) => <VideoCard video={video} key={video.id} />)}</section><Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={setPage} /></>
      ) : (
        <EmptyState icon={<Users size={28} />} title="关注动态还是空的" text="去视频页或作者空间关注喜欢的创作者，新投稿会出现在这里。" action={<Link to="/popular" className="primary-button"><Compass size={17} />发现创作者</Link>} />
      )}
    </div>
  );
}

function CreatorPage({ user }: { user: User | null }) {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const [profile, setProfile] = useState<CreatorProfile | null>(null);
  const [result, setResult] = useState<VideoPageData>({ items: [], page: 1, page_size: 12, total: 0, has_next: false });
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const page = pageFrom(searchParams);

  useEffect(() => {
    setLoading(true);
    setError("");
    Promise.all([
      api.creator(id),
      api.creatorVideos(id, new URLSearchParams({ page: String(page), page_size: "12" }))
    ]).then(([nextProfile, nextVideos]) => {
      setProfile(nextProfile);
      setResult(nextVideos);
    }).catch((err) => setError(errorMessage(err))).finally(() => setLoading(false));
  }, [id, page]);

  const setPage = (value: number) => {
    const next = new URLSearchParams(searchParams);
    value > 1 ? next.set("page", String(value)) : next.delete("page");
    setSearchParams(next);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const toggleFollow = async () => {
    if (!profile) return;
    if (!user) {
      navigate(`/auth?next=${encodeURIComponent(`${location.pathname}${location.search}`)}`);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const next = await api.toggleFollow(profile.id);
      setProfile((current) => current ? {
        ...current,
        followed: next.active,
        followers_count: Math.max(0, current.followers_count + (next.active ? 1 : -1))
      } : current);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  if (loading) return <LoadingBlock label="正在打开作者空间" />;
  if (error && !profile) return <ErrorBlock message={error} />;
  if (!profile) return null;
  const isSelf = user?.id === profile.id;

  return (
    <div className="page creator-page">
      <section className="creator-hero" aria-labelledby="creator-name">
        <span className="creator-hero-avatar" aria-hidden="true">{profile.username.slice(0, 1).toUpperCase()}</span>
        <div className="creator-identity">
          <p className="eyebrow">作者空间</p>
          <h1 id="creator-name">{profile.username}</h1>
          <p>{profile.bio || "这位创作者还没有填写个人简介。"}</p>
          <span className="creator-joined"><Clock3 size={14} />{formatJoinDate(profile.created_at)} 加入</span>
        </div>
        <dl className="creator-stats">
          <div><dt>投稿</dt><dd>{formatCount(profile.videos_count)}</dd></div>
          <div><dt>粉丝</dt><dd>{formatCount(profile.followers_count)}</dd></div>
          <div><dt>关注</dt><dd>{formatCount(profile.following_count)}</dd></div>
        </dl>
        <div className="creator-action">
          {isSelf ? (
            <Link to="/me/videos" className="primary-button"><Film size={17} />管理投稿</Link>
          ) : (
            <button type="button" className={profile.followed ? "secondary-button follow-button active" : "primary-button follow-button"} disabled={busy} onClick={toggleFollow} aria-pressed={profile.followed}>
              {profile.followed ? <Check size={17} /> : <UserRound size={17} />}{busy ? "处理中..." : profile.followed ? "已关注" : "关注"}
            </button>
          )}
        </div>
      </section>
      {error && <p className="inline-error creator-error">{error}</p>}
      <section className="creator-content-head"><div><p className="eyebrow">全部投稿</p><h2>{profile.username} 的作品</h2></div><span>共 {result.total} 条</span></section>
      {result.items.length ? (
        <><section className="video-grid">{result.items.map((video) => <VideoCard video={video} key={video.id} />)}</section><Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={setPage} /></>
      ) : (
        <EmptyState icon={<Film size={28} />} title="还没有公开投稿" text={isSelf ? "发布第一条作品，让个人空间丰富起来。" : "这位创作者发布作品后会显示在这里。"} action={isSelf ? <Link to="/upload" className="primary-button"><Upload size={17} />发布视频</Link> : <Link to="/popular" className="secondary-button"><Compass size={17} />浏览热门</Link>} />
      )}
    </div>
  );
}

function VideoCard({ video }: { video: Video }) {
  return (
    <article className="video-card">
      <Link to={`/video/${video.id}`} className="cover-link">
        {video.cover_url ? <img src={video.cover_url} alt="" loading="lazy" /> : <div className="cover-fallback"><Play size={28} fill="currentColor" /></div>}
        <span className="duration">{formatDuration(video.duration_seconds)}</span>
        <span className="play-overlay"><Play size={22} fill="currentColor" /></span>
      </Link>
      <div className="video-card-body">
        <Link to={`/video/${video.id}`} className="video-title">{video.title}</Link>
        <div className="video-author"><Link to={`/users/${video.user_id}`} className="video-author-link"><span className="mini-avatar">{video.username.slice(0, 1)}</span><span>{video.username}</span></Link><span className="category-label">{video.category}</span></div>
        <div className="video-stats"><span><Eye size={14} />{formatCount(video.views_count)}</span><span><MessageCircle size={14} />{formatCount(video.comments_count)}</span><time>{formatDate(video.created_at)}</time></div>
      </div>
    </article>
  );
}

const processingLabels: Record<Video["processing_status"], string> = {
  pending: "等待媒体探测",
  processing: "正在探测媒体",
  ready: "可播放",
  failed: "媒体处理失败"
};

function ProcessingBadge({ video }: { video: Video }) {
  const icon = video.processing_status === "ready"
    ? <CircleCheck size={13} />
    : video.processing_status === "failed"
      ? <CircleAlert size={13} />
      : <Clock3 size={13} />;
  return <span className={`processing-badge ${video.processing_status}`}>{icon}{processingLabels[video.processing_status]}</span>;
}

function Pagination({ page, pageSize, total, hasNext, onPageChange }: { page: number; pageSize: number; total: number; hasNext: boolean; onPageChange: (page: number) => void }) {
  if (total <= pageSize && page === 1) return null;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const candidates = Array.from(new Set([1, page - 1, page, page + 1, totalPages])).filter((value) => value >= 1 && value <= totalPages).sort((a, b) => a - b);
  return (
    <nav className="pagination" aria-label="视频分页">
      <button type="button" className="pagination-arrow" disabled={page <= 1} onClick={() => onPageChange(page - 1)} aria-label="上一页" title="上一页"><ChevronLeft size={18} /></button>
      <div className="pagination-pages">
        {candidates.map((value, index) => <span key={value}>{index > 0 && value - candidates[index - 1] > 1 && <i>...</i>}<button type="button" className={value === page ? "active" : ""} aria-current={value === page ? "page" : undefined} onClick={() => onPageChange(value)}>{value}</button></span>)}
      </div>
      <button type="button" className="pagination-arrow" disabled={!hasNext} onClick={() => onPageChange(page + 1)} aria-label="下一页" title="下一页"><ChevronRight size={18} /></button>
      <span className="pagination-total">共 {totalPages} 页</span>
    </nav>
  );
}

function VideoPage({ user }: { user: User | null }) {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const [video, setVideo] = useState<Video | null>(null);
  const [comments, setComments] = useState<Comment[]>([]);
  const [relatedVideos, setRelatedVideos] = useState<Video[]>([]);
  const [content, setContent] = useState("");
  const [loading, setLoading] = useState(true);
  const [relatedLoading, setRelatedLoading] = useState(true);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [shareStatus, setShareStatus] = useState("");

  useEffect(() => {
    setLoading(true);
    setRelatedLoading(true);
    setRelatedVideos([]);
    Promise.all([api.video(id), api.comments(id)])
      .then(([nextVideo, nextComments]) => {
        setVideo(nextVideo);
        setComments(nextComments);
        const params = new URLSearchParams({ sort: "popular", limit: "12", category: nextVideo.category });
        api.videos(params)
          .then((related) => setRelatedVideos(related.items.filter((item) => item.id !== nextVideo.id).slice(0, 8)))
          .catch(console.error)
          .finally(() => setRelatedLoading(false));
      })
      .catch((err) => setError(errorMessage(err)))
      .finally(() => setLoading(false));
  }, [id]);

  useEffect(() => {
    if (!video || (video.processing_status !== "pending" && video.processing_status !== "processing")) return;
    const timer = window.setInterval(() => {
      api.video(id, false).then(setVideo).catch(console.error);
    }, 2500);
    return () => window.clearInterval(timer);
  }, [id, video?.processing_status]);

  const requireUser = () => {
    if (!user) { navigate(`/auth?next=${encodeURIComponent(`/video/${id}`)}`); return false; }
    return true;
  };

  const toggle = async (kind: "like" | "favorite") => {
    if (!video || !requireUser()) return;
    setBusy(kind);
    try {
      const result = kind === "like" ? await api.toggleLike(video.id) : await api.toggleFavorite(video.id);
      setVideo((current) => current ? {
        ...current,
        [kind === "like" ? "liked" : "favorited"]: result.active,
        [kind === "like" ? "likes_count" : "favorites_count"]: current[kind === "like" ? "likes_count" : "favorites_count"] + (result.active ? 1 : -1)
      } : current);
    } catch (err) { setError(errorMessage(err)); } finally { setBusy(""); }
  };

  const submitComment = async (event: FormEvent) => {
    event.preventDefault();
    if (!requireUser() || !content.trim()) return;
    setBusy("comment");
    try {
      const created = await api.comment(id, content.trim());
      setComments((current) => [created, ...current]);
      setVideo((current) => current ? { ...current, comments_count: current.comments_count + 1 } : current);
      setContent("");
    } catch (err) { setError(errorMessage(err)); } finally { setBusy(""); }
  };

  const shareVideo = async () => {
    if (!video) return;
    const shareData = { title: video.title, url: window.location.href };
    try {
      if (navigator.share) {
        await navigator.share(shareData);
        setShareStatus("分享面板已打开");
      } else {
        await navigator.clipboard.writeText(shareData.url);
        setShareStatus("链接已复制");
      }
    } catch (err) {
      if (err instanceof DOMException && err.name === "AbortError") return;
      try {
        await navigator.clipboard.writeText(shareData.url);
        setShareStatus("链接已复制");
      } catch {
        setShareStatus("复制失败，请手动复制地址");
      }
    }
    window.setTimeout(() => setShareStatus(""), 2500);
  };

  if (loading) return <LoadingBlock label="正在准备播放器" />;
  if (error && !video) return <ErrorBlock message={error} />;
  if (!video) return null;

  return (
    <div className="page watch-page">
      <section className="watch-layout">
        <div className="watch-main">
          <div className="watch-title-row">
            <div><p className="eyebrow">{video.category}</p><h1>{video.title}</h1><div className="watch-meta"><span><Eye size={15} />{formatCount(video.views_count)} 播放</span><span><Clock3 size={15} />{formatDate(video.created_at)}</span>{video.processing_status === "ready" && video.source_width > 0 && <span>{video.source_width} × {video.source_height}</span>}</div></div>
          </div>
          <VideoPlayer video={video} />
          {video.processing_status !== "ready" && <div className={`processing-notice ${video.processing_status}`}><ProcessingBadge video={video} /><span>{video.processing_status === "failed" ? "HLS 处理失败，原始文件仍可播放；请检查 FFmpeg、FFprobe 配置和后台日志。" : "原始文件已保存并可播放，后台正在分析媒体并生成自适应清晰度。"}</span></div>}
          <div className="watch-action-row">
            <div className="watch-actions">
              <button className={video.liked ? "active" : ""} disabled={busy === "like"} onClick={() => toggle("like")}><Heart size={19} fill={video.liked ? "currentColor" : "none"} />{formatCount(video.likes_count)}</button>
              <button className={video.favorited ? "active" : ""} disabled={busy === "favorite"} onClick={() => toggle("favorite")}><Bookmark size={19} fill={video.favorited ? "currentColor" : "none"} />{formatCount(video.favorites_count)}</button>
              <button onClick={shareVideo}><Share2 size={19} />分享</button>
            </div>
            {shareStatus && <span className="share-feedback" role="status">{shareStatus}</span>}
          </div>
          {error && <p className="inline-error">{error}</p>}
          <div className="creator-strip"><Link to={`/users/${video.user_id}`} className="creator-avatar" aria-label={`查看 ${video.username} 的作者空间`}>{video.username.slice(0, 1)}</Link><div><Link to={`/users/${video.user_id}`} className="creator-name-link">{video.username}</Link><span>创作者</span></div></div>
          {video.description && <p className="video-description">{video.description}</p>}
          {user?.id === video.user_id && <SubtitleManager video={video} onAdded={(track) => setVideo((current) => current ? { ...current, subtitle_tracks: [...(current.subtitle_tracks ?? []), track] } : current)} />}
          <section className="comment-section" aria-labelledby="comments-title">
            <div className="comment-heading"><h2 id="comments-title">评论</h2><span>{video.comments_count}</span></div>
            <form className="comment-form" onSubmit={submitComment}>
              <textarea value={content} onChange={(event) => setContent(event.target.value)} maxLength={500} placeholder={user ? "说说你的想法" : "登录后参与讨论"} disabled={!user} />
              <button className="primary-button compact" disabled={!content.trim() || busy === "comment"}><Send size={16} />发布</button>
            </form>
            <div className="comment-list">
              {comments.length ? comments.map((comment) => <div className="comment-item" key={comment.id}><Link to={`/users/${comment.user_id}`} className="mini-avatar" aria-label={`查看 ${comment.username} 的作者空间`}>{comment.username.slice(0, 1)}</Link><div><Link to={`/users/${comment.user_id}`} className="comment-author-link">{comment.username}</Link><p>{comment.content}</p><time>{formatDate(comment.created_at)}</time></div></div>) : <p className="comment-empty">还没有评论，来聊第一句。</p>}
            </div>
          </section>
        </div>
        <aside className="related-panel" aria-labelledby="related-title">
          <div className="related-heading"><div><p className="eyebrow">继续观看</p><h2 id="related-title">相关推荐</h2></div><Link to={`/popular?category=${encodeURIComponent(video.category)}`}>更多</Link></div>
          {relatedLoading ? (
            <div className="related-list">{Array.from({ length: 5 }, (_, index) => <div className="related-skeleton" key={index}><span /><div><i /><i /></div></div>)}</div>
          ) : relatedVideos.length ? (
            <div className="related-list">{relatedVideos.map((item) => <RelatedVideoCard video={item} key={item.id} />)}</div>
          ) : (
            <div className="related-empty"><Film size={24} /><strong>暂无更多视频</strong><span>同分区的新作品会显示在这里。</span><Link to="/latest">看看最新发布</Link></div>
          )}
        </aside>
      </section>
    </div>
  );
}

function RelatedVideoCard({ video }: { video: Video }) {
  return (
    <article className="related-video">
      <Link to={`/video/${video.id}`} className="related-cover">
        {video.cover_url ? <img src={video.cover_url} alt="" loading="lazy" /> : <div className="cover-fallback"><Play size={22} fill="currentColor" /></div>}
        <span className="duration">{formatDuration(video.duration_seconds)}</span>
      </Link>
      <div className="related-copy">
        <Link to={`/video/${video.id}`} className="related-title">{video.title}</Link>
        <Link to={`/users/${video.user_id}`} className="related-author">{video.username}</Link>
        <span><Eye size={13} />{formatCount(video.views_count)} 播放</span>
      </div>
    </article>
  );
}

function SubtitleManager({ video, onAdded }: { video: Video; onAdded: (track: SubtitleTrack) => void }) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!file) return;
    setBusy(true); setError("");
    const form = new FormData();
    form.set("subtitle", file); form.set("subtitle_language", "zh-CN"); form.set("subtitle_label", "中文");
    try { onAdded(await api.uploadSubtitle(video.id, form)); setFile(null); if (inputRef.current) inputRef.current.value = ""; }
    catch (err) { setError(errorMessage(err)); } finally { setBusy(false); }
  };
  return <form className="subtitle-manager" onSubmit={submit}>
    <div><strong>字幕管理</strong><span>为作品补传 VTT 或 SRT 字幕，最大 2 MB</span></div>
    <input ref={inputRef} type="file" accept=".vtt,.srt,text/vtt,application/x-subrip" onChange={(event) => setFile(event.target.files?.[0] || null)} />
    <button className="secondary-button compact" disabled={!file || busy}>{busy ? "上传中..." : "上传字幕"}</button>
    {error && <p className="inline-error">{error}</p>}
  </form>;
}

function AuthPage({ onAuth }: { onAuth: (payload: AuthPayload) => void }) {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const [mode, setMode] = useState<AuthMode>("login");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true); setError("");
    try {
      const payload = mode === "login" ? await api.login(username, password) : await api.register(username, password);
      onAuth(payload);
      navigate(params.get("next") || "/");
    } catch (err) { setError(errorMessage(err)); } finally { setBusy(false); }
  };

  return (
    <div className="page auth-page">
      <section className="auth-intro"><p className="eyebrow">加入片场</p><h1>每一次上传，都是一段时间被认真留下。</h1><div className="auth-steps"><span><strong>01</strong>发现创作</span><span><strong>02</strong>分享作品</span><span><strong>03</strong>参与讨论</span></div></section>
      <section className="auth-form-wrap">
        <div className="segmented-control full"><button className={mode === "login" ? "active" : ""} onClick={() => setMode("login")}>登录</button><button className={mode === "register" ? "active" : ""} onClick={() => setMode("register")}>注册</button></div>
        <form className="stack-form" onSubmit={submit}>
          <label>用户名<input value={username} onChange={(event) => setUsername(event.target.value)} minLength={3} maxLength={24} autoComplete="username" placeholder="3-24 位中文、字母或数字" required /></label>
          <label>密码<input value={password} onChange={(event) => setPassword(event.target.value)} type="password" minLength={8} maxLength={72} autoComplete={mode === "login" ? "current-password" : "new-password"} placeholder="至少 8 位" required /></label>
          {error && <p className="inline-error">{error}</p>}
          <button className="primary-button wide" disabled={busy}>{busy ? "处理中..." : mode === "login" ? "登录" : "创建账号"}</button>
        </form>
      </section>
    </div>
  );
}

function UploadPage() {
  const navigate = useNavigate();
  const videoRef = useRef<HTMLInputElement>(null);
  const coverRef = useRef<HTMLInputElement>(null);
  const subtitleRef = useRef<HTMLInputElement>(null);
  const [categories, setCategories] = useState<string[]>([]);
  const [videoFile, setVideoFile] = useState<File | null>(null);
  const [coverFile, setCoverFile] = useState<File | null>(null);
  const [subtitleFile, setSubtitleFile] = useState<File | null>(null);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => { api.categories().then((items) => { setCategories(items); setCategory(items[0] || ""); }).catch((err) => setError(errorMessage(err))); }, []);
  const coverPreview = useMemo(() => coverFile ? URL.createObjectURL(coverFile) : "", [coverFile]);
  useEffect(() => () => { if (coverPreview) URL.revokeObjectURL(coverPreview); }, [coverPreview]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!videoFile) { setError("请选择视频文件"); return; }
    setBusy(true); setError("");
    const form = new FormData();
    form.set("title", title); form.set("description", description); form.set("category", category); form.set("video", videoFile);
    if (coverFile) form.set("cover", coverFile);
    if (subtitleFile) { form.set("subtitle", subtitleFile); form.set("subtitle_language", "zh-CN"); form.set("subtitle_label", "中文"); }
    try { const created = await api.upload(form); navigate(`/video/${created.id}`); }
    catch (err) { setError(errorMessage(err)); } finally { setBusy(false); }
  };

  return (
    <div className="page upload-page">
      <section className="page-heading"><p className="eyebrow">创作中心</p><h1>发布新作品</h1><p>支持 MP4、WebM、Ogg，未选择封面时会自动截取视频画面。</p></section>
      <form className="upload-layout" onSubmit={submit}>
        <div className="upload-media-column">
          <button type="button" className={`file-drop ${videoFile ? "selected" : ""}`} onClick={() => videoRef.current?.click()}>
            <Upload size={30} /><strong>{videoFile ? videoFile.name : "选择视频文件"}</strong><span>{videoFile ? `${(videoFile.size / 1024 / 1024).toFixed(1)} MB` : "单个文件最大 512 MB"}</span>
          </button>
          <input ref={videoRef} className="sr-only" type="file" accept="video/mp4,video/webm,video/ogg" onChange={(event) => setVideoFile(event.target.files?.[0] || null)} />
          <button type="button" className="cover-picker" onClick={() => coverRef.current?.click()}>
            {coverPreview ? <img src={coverPreview} alt="封面预览" /> : <><Film size={24} /><span>选择自定义封面</span></>}
          </button>
          <input ref={coverRef} className="sr-only" type="file" accept="image/jpeg,image/png,image/webp" onChange={(event) => setCoverFile(event.target.files?.[0] || null)} />
          <button type="button" className={`subtitle-picker ${subtitleFile ? "selected" : ""}`} onClick={() => subtitleRef.current?.click()}>
            <MessageCircle size={20} /><span>{subtitleFile ? subtitleFile.name : "添加字幕文件（可选）"}</span><small>支持 VTT / SRT，最大 2 MB</small>
          </button>
          <input ref={subtitleRef} className="sr-only" type="file" accept=".vtt,.srt,text/vtt,application/x-subrip" onChange={(event) => setSubtitleFile(event.target.files?.[0] || null)} />
        </div>
        <div className="stack-form upload-fields">
          <label>标题<input value={title} onChange={(event) => setTitle(event.target.value)} minLength={2} maxLength={80} placeholder="准确说明视频内容" required /><span className="field-count">{title.length}/80</span></label>
          <label>分区<select value={category} onChange={(event) => setCategory(event.target.value)} required>{categories.map((item) => <option key={item}>{item}</option>)}</select></label>
          <label>简介<textarea value={description} onChange={(event) => setDescription(event.target.value)} maxLength={2000} rows={8} placeholder="补充创作背景、内容提要或相关链接" /><span className="field-count">{description.length}/2000</span></label>
          {error && <p className="inline-error">{error}</p>}
          <button className="primary-button wide" disabled={busy || !videoFile}>{busy ? "正在处理视频..." : "发布作品"}</button>
        </div>
      </form>
    </div>
  );
}

function MyVideosPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [result, setResult] = useState<VideoPageData>({ items: [], page: 1, page_size: 12, total: 0, has_next: false });
  const [categories, setCategories] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [editing, setEditing] = useState<Video | null>(null);
  const [deleting, setDeleting] = useState<Video | null>(null);
  const [busy, setBusy] = useState("");
  const page = pageFrom(searchParams);
  const videos = result.items;

  const requestPage = (pageNumber = page) => api.myVideos(new URLSearchParams({ page: String(pageNumber), page_size: "12" }));

  useEffect(() => { api.categories().then(setCategories).catch(console.error); }, []);
  useEffect(() => {
    setLoading(true);
    setError("");
    requestPage().then(setResult).catch((err) => setError(errorMessage(err))).finally(() => setLoading(false));
  }, [page]);

  const processingKey = videos.filter((video) => video.processing_status === "pending" || video.processing_status === "processing").map((video) => video.id).join(",");
  useEffect(() => {
    if (!processingKey) return;
    const timer = window.setInterval(() => requestPage().then(setResult).catch(console.error), 2500);
    return () => window.clearInterval(timer);
  }, [page, processingKey]);

  const setPage = (value: number) => {
    const next = new URLSearchParams(searchParams);
    value > 1 ? next.set("page", String(value)) : next.delete("page");
    setSearchParams(next);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };
  const showNotice = (message: string) => {
    setNotice(message);
    window.setTimeout(() => setNotice(""), 3000);
  };
  const retry = async (video: Video) => {
    setBusy(`retry-${video.id}`); setError("");
    try {
      await api.retryVideo(video.id);
      setResult((current) => ({ ...current, items: current.items.map((item) => item.id === video.id ? { ...item, processing_status: "pending", processing_error: undefined } : item) }));
      showNotice("已重新加入转码队列");
    } catch (err) { setError(errorMessage(err)); } finally { setBusy(""); }
  };
  const remove = async (video: Video) => {
    setBusy(`delete-${video.id}`); setError("");
    try {
      await api.deleteVideo(video.id);
      setDeleting(null);
      if (videos.length === 1 && page > 1) {
        setPage(page - 1);
      } else {
        setResult(await requestPage());
      }
      showNotice("投稿及其媒体文件已删除");
    } catch (err) { setError(errorMessage(err)); } finally { setBusy(""); }
  };
  const applyUpdated = (updated: Video) => {
    setResult((current) => ({ ...current, items: current.items.map((video) => video.id === updated.id ? updated : video) }));
    setEditing(null);
    showNotice("投稿信息已保存");
  };

  return (
    <div className="page management-page">
      <section className="page-heading heading-row"><div><p className="eyebrow">个人空间</p><h1>我的投稿</h1><p>查看作品表现，继续完善你的创作列表。</p></div><Link to="/upload" className="primary-button"><Upload size={17} />发布视频</Link></section>
      <div className="management-summary"><span><strong>{result.total}</strong> 条投稿</span><span>每页 12 条</span></div>
      {notice && <div className="management-notice" role="status"><CircleCheck size={17} />{notice}</div>}
      {error && <p className="inline-error management-error">{error}</p>}
      {loading ? <ManagementLoading /> : videos.length ? (
        <>
          <section className="management-list" aria-label="投稿列表">
            {videos.map((video) => (
              <article className="management-row" key={video.id}>
                <Link to={`/video/${video.id}`} className="management-cover">
                  {video.cover_url ? <img src={video.cover_url} alt="" loading="lazy" /> : <div className="cover-fallback"><Play size={24} fill="currentColor" /></div>}
                  <span className="duration">{formatDuration(video.duration_seconds)}</span>
                </Link>
                <div className="management-copy">
                  <div className="management-title-line"><Link to={`/video/${video.id}`}>{video.title}</Link><ProcessingBadge video={video} /></div>
                  <p>{video.description || "暂未填写视频简介"}</p>
                  <div className="management-meta"><span>{video.category}</span><span>{formatDate(video.created_at)}</span><span>{formatFileSize(video.size_bytes)}</span></div>
                  {video.processing_status === "failed" && <span className="management-processing-error">{video.processing_error || "媒体处理失败，可以手动重新转码"}</span>}
                  <div className="management-stats"><span><Eye size={14} />{formatCount(video.views_count)} 播放</span><span><Heart size={14} />{formatCount(video.likes_count)}</span><span><MessageCircle size={14} />{formatCount(video.comments_count)}</span></div>
                </div>
                <div className="management-actions" aria-label={`${video.title}的管理操作`}>
                  <button type="button" onClick={() => setEditing(video)} title="编辑投稿"><Pencil size={16} />编辑</button>
                  {video.processing_status === "failed" && <button type="button" disabled={busy === `retry-${video.id}`} onClick={() => retry(video)} title="重新转码"><RefreshCw size={16} className={busy === `retry-${video.id}` ? "spin-icon" : ""} />重试</button>}
                  <button type="button" className="danger" disabled={video.processing_status === "processing" || busy === `delete-${video.id}`} onClick={() => setDeleting(video)} title={video.processing_status === "processing" ? "转码完成后才能删除" : "删除投稿"}><Trash2 size={16} />删除</button>
                </div>
              </article>
            ))}
          </section>
          <Pagination page={result.page} pageSize={result.page_size} total={result.total} hasNext={result.has_next} onPageChange={setPage} />
        </>
      ) : <EmptyState icon={<History size={28} />} title="还没有投稿" text="第一条作品不必完美，先让它可以被看见。" action={<Link to="/upload" className="primary-button"><Upload size={17} />开始投稿</Link>} />}
      {editing && <EditVideoDialog video={editing} categories={categories} onClose={() => setEditing(null)} onSaved={applyUpdated} />}
      {deleting && <DeleteVideoDialog video={deleting} busy={busy === `delete-${deleting.id}`} onClose={() => setDeleting(null)} onConfirm={() => remove(deleting)} />}
    </div>
  );
}

function EditVideoDialog({ video, categories, onClose, onSaved }: { video: Video; categories: string[]; onClose: () => void; onSaved: (video: Video) => void }) {
  const coverRef = useRef<HTMLInputElement>(null);
  const [title, setTitle] = useState(video.title);
  const [description, setDescription] = useState(video.description);
  const [category, setCategory] = useState(video.category);
  const [cover, setCover] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const coverPreview = useMemo(() => cover ? URL.createObjectURL(cover) : video.cover_url, [cover, video.cover_url]);
  useEffect(() => () => { if (cover) URL.revokeObjectURL(coverPreview); }, [cover, coverPreview]);
  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === "Escape" && !busy) onClose(); };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [busy, onClose]);
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true); setError("");
    const form = new FormData();
    form.set("title", title); form.set("description", description); form.set("category", category);
    if (cover) form.set("cover", cover);
    try { onSaved(await api.updateVideo(video.id, form)); }
    catch (err) { setError(errorMessage(err)); } finally { setBusy(false); }
  };
  return (
    <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) onClose(); }}>
      <section className="dialog-panel edit-video-dialog" role="dialog" aria-modal="true" aria-labelledby="edit-video-title">
        <header><div><p className="eyebrow">投稿管理</p><h2 id="edit-video-title">编辑视频信息</h2></div><button type="button" className="icon-button" onClick={onClose} disabled={busy} aria-label="关闭编辑窗口" title="关闭"><X size={19} /></button></header>
        <form className="edit-video-form" onSubmit={submit}>
          <button type="button" className="edit-cover" onClick={() => coverRef.current?.click()}>
            {coverPreview ? <img src={coverPreview} alt="当前封面" /> : <div className="cover-fallback"><Film size={28} /></div>}
            <span><ImagePlus size={16} />{cover ? "已选择新封面" : "替换封面"}</span>
          </button>
          <input ref={coverRef} className="sr-only" type="file" accept="image/jpeg,image/png,image/webp" onChange={(event) => setCover(event.target.files?.[0] || null)} />
          <div className="stack-form edit-video-fields">
            <label>标题<input value={title} onChange={(event) => setTitle(event.target.value)} minLength={2} maxLength={80} required /><span className="field-count">{title.length}/80</span></label>
            <label>分区<select value={category} onChange={(event) => setCategory(event.target.value)} required>{(categories.length ? categories : [video.category]).map((item) => <option key={item}>{item}</option>)}</select></label>
            <label>简介<textarea value={description} onChange={(event) => setDescription(event.target.value)} maxLength={2000} rows={6} placeholder="补充视频内容和创作背景" /><span className="field-count">{description.length}/2000</span></label>
            {error && <p className="inline-error">{error}</p>}
            <div className="dialog-actions"><button type="button" className="secondary-button" onClick={onClose} disabled={busy}>取消</button><button className="primary-button" disabled={busy}>{busy ? "保存中..." : "保存修改"}</button></div>
          </div>
        </form>
      </section>
    </div>
  );
}

function DeleteVideoDialog({ video, busy, onClose, onConfirm }: { video: Video; busy: boolean; onClose: () => void; onConfirm: () => void }) {
  return (
    <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) onClose(); }}>
      <section className="dialog-panel delete-video-dialog" role="alertdialog" aria-modal="true" aria-labelledby="delete-video-title" aria-describedby="delete-video-description">
        <span className="danger-icon"><Trash2 size={22} /></span>
        <h2 id="delete-video-title">删除这条投稿？</h2>
        <p id="delete-video-description">“{video.title}”的数据库记录、原视频、封面、字幕和转码文件都会被永久删除，此操作无法撤销。</p>
        <div className="dialog-actions"><button type="button" className="secondary-button" onClick={onClose} disabled={busy}>取消</button><button type="button" className="danger-button" onClick={onConfirm} disabled={busy}>{busy ? "正在删除..." : "确认删除"}</button></div>
      </section>
    </div>
  );
}

function ManagementLoading() { return <div className="management-list">{Array.from({ length: 5 }, (_, index) => <div className="management-skeleton" key={index}><span /><div><i /><i /><i /></div></div>)}</div>; }

function LoadingGrid() { return <div className="video-grid">{Array.from({ length: 8 }, (_, index) => <div className="skeleton-card" key={index}><span /><i /><i /></div>)}</div>; }
function LoadingBlock({ label }: { label: string }) { return <div className="state-block"><span className="spinner" /><p>{label}</p></div>; }
function ErrorBlock({ message }: { message: string }) { return <div className="state-block error-state"><X size={28} /><h2>内容加载失败</h2><p>{message}</p><button className="secondary-button" onClick={() => window.location.reload()}>重新加载</button></div>; }
function EmptyState({ icon, title, text, action }: { icon: React.ReactNode; title: string; text: string; action: React.ReactNode }) { return <div className="state-block empty-state">{icon}<h2>{title}</h2><p>{text}</p>{action}</div>; }
function NotFound() { return <div className="state-block"><UserRound size={30} /><h1>页面没有找到</h1><p>这个地址可能已经改变。</p><Link className="primary-button" to="/">返回首页</Link></div>; }

export default App;
