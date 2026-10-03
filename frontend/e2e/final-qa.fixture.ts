import { expect, type Page } from "@playwright/test";
import type { Comment, CreatorProfile, CreatorStats, Notification, SubtitleTrack, User, Video, VideoReport, VideoReportStatus } from "../src/types";
import { videoFixture } from "../src/shared/test/videoFixtures";

export type FinalQaRole = "anonymous" | "authenticated" | "self" | "other" | "admin";
export type FinalQaScenario = "normal" | "long" | "empty" | "error" | "slow";
export interface FinalQaOptions { role?: FinalQaRole; scenario?: FinalQaScenario; videoStatus?: Video["processing_status"] }
export interface FinalQaRequest { method: string; path: string; query: string }
export interface FinalQaState {
  video: Video;
  profile: CreatorProfile;
  requests: FinalQaRequest[];
  unknownRequests: string[];
  reads(path: string): number;
  gateRead(path: string): void;
  releaseRead(path: string): void;
  gateMutation(path: string): void;
  releaseMutation(path: string): void;
  expire(): void;
  setVideoStatus(status: Video["processing_status"]): void;
}

export const finalQaViewports = [[1536, 960], [1440, 900], [1180, 820], [920, 900], [768, 1024], [430, 932], [390, 844], [320, 720]] as const;
export const finalQaCover = "/phase6-fixture-cover.svg";
export const finalQaBrokenCover = "/phase6-missing-cover.jpg";
const stamp = "2026-10-03T08:00:00Z";
const csrf = "phase6-isolated-csrf";
const abstractCover = `<svg xmlns="http://www.w3.org/2000/svg" width="960" height="540" viewBox="0 0 960 540"><rect width="960" height="540" fill="#151b24"/><path d="M100 105h760v330H100zM160 155h640v230H160z" fill="none" stroke="#627787" stroke-width="2"/><path d="M160 155h320M640 385H320" stroke="#00b9ba" stroke-width="3"/><circle cx="480" cy="270" r="74" fill="none" stroke="#ff7191" stroke-width="2"/><text x="480" y="470" fill="#b4bbc6" font-size="18" text-anchor="middle" font-family="monospace">ISOLATED MEDIA FIXTURE / GVIDEO RC</text></svg>`;

// The single real API read is explicitly read-only and supplies playable media.
// All browser business API requests, including mutations and unknown endpoints,
// are fulfilled locally. This fixture never registers/uploads/moderates for real.
export async function installFinalQaFixture(page: Page, options: FinalQaOptions = {}): Promise<FinalQaState> {
  const mediaID = process.env.E2E_MEDIA_VIDEO_ID || "1";
  const response = await page.request.get(`/api/v1/videos/${encodeURIComponent(mediaID)}?count_view=false`);
  expect(response.ok(), "A ready seed is required in the independent E2E environment").toBe(true);
  const media = (await response.json()).data as Video;
  expect(media.processing_status).toBe("ready");
  const role = options.role || "self", scenario = options.scenario || "normal";
  const long = scenario === "long", empty = scenario === "empty";
  let signedIn = role !== "anonymous";
  let expired = false;
  const profile: CreatorProfile = { id: 42, username: long ? "超长创作者_ASCII_0123456789_".repeat(6) : "RC 创作者", avatar_url: "", bio: long ? "创作者简介_ASCII_".repeat(110) : "认真发现与创作，留住值得分享的内容。", videos_count: empty ? 0 : 13, followers_count: empty ? 0 : 128, following_count: empty ? 0 : 9, followed: false, created_at: stamp };
  let user: User = { id: role === "authenticated" || role === "other" ? 7 : 42, username: long ? "RC_USER_ASCII_0123456789_".repeat(5) : "RC 创作者", avatar_url: "", bio: profile.bio, is_admin: role === "admin", created_at: stamp };
  const first = videoFixture(1, { ...media, id: 1, user_id: 42, username: profile.username, category: "音乐", title: long ? "超长中文作品标题".repeat(32) : "光影之间，看见创作的日常", description: long ? "作品简介_ABCDEFGHIJKLMNOPQRSTUVWXYZ_0123456789_".repeat(48) : "一段影像，一次认真观察。\n本页内容来自独立验收夹具。", cover_url: finalQaCover, views_count: empty ? 0 : 12540, likes_count: empty ? 0 : 128, favorites_count: empty ? 0 : 42, comments_count: empty ? 0 : 2, liked: false, favorited: false, processing_status: options.videoStatus || "ready" });
  let items = empty ? [] : Array.from({ length: 13 }, (_, index) => {
    const status = ["ready", "ready", "processing", "failed", "pending"][(index % 5)] as Video["processing_status"];
    return { ...first, id: index + 1, category: ["音乐", "生活", "纪录"][index % 3], title: index === 0 ? first.title : long ? (index % 2 ? "LONG_ASCII_0123456789_UNDERSCORE_".repeat(16) : "超长中文标题".repeat(32)) : `RC 作品 ${index + 1} · 记录与发现`, cover_url: index % 4 === 0 ? finalQaCover : index % 4 === 1 ? "" : finalQaBrokenCover, processing_status: status, processing_stage: status === "processing" ? "transcoding" : status === "pending" ? "queued" : status === "failed" ? "failed" : "done", processing_progress: status === "processing" ? 45 : status === "pending" ? 0 : 100, processing_error: status === "failed" ? "隔离验收：媒体处理失败，请重试。" : undefined, views_count: 12540 - index * 10 };
  });
  const kinds: Notification["type"][] = ["follow", "like", "favorite", "comment", "processing_ready", "processing_failed"];
  let notifications: Notification[] = empty ? [] : Array.from({ length: long ? 21 : 6 }, (_, index) => ({ id: index + 1, type: kinds[index % 6], actor_id: 9, actor_username: long ? "ACTOR_LONG_ASCII_".repeat(9) : "另一位创作者", video_id: index % 6 ? 1 : undefined, video_title: long ? "通知长标题_ASCII_0123456789_".repeat(14) : first.title, comment_preview: long ? "COMMENT_ASCII_0123456789_".repeat(18) : "谢谢分享，喜欢这段内容的节奏。", created_at: stamp }));
  let comments: Comment[] = empty ? [] : [{ id: 1, video_id: 1, user_id: user.id, username: user.username, avatar_url: "", content: long ? "COMMENT_LONG_ASCII_中文_0123456789_".repeat(22) : "每一次认真观察，都让日常有了新的发现。", created_at: stamp }];
  const statuses: VideoReportStatus[] = ["pending", "reviewed", "resolved", "dismissed"];
  let reports: VideoReport[] = empty ? [] : statuses.map((status, index) => ({ id: index + 1, video_id: index + 1, video_title: long ? "REPORT_LONG_ASCII_0123456789_".repeat(18) : `RC 举报作品 ${index + 1}`, video_author_id: 42, video_author: profile.username, user_id: 9, reporter_username: long ? "REPORTER_LONG_ASCII_".repeat(8) : "举报人", reason: ["spam", "inappropriate", "copyright", "other"][index], detail: long ? "DETAIL_ASCII_0123456789_中文_".repeat(30) : "该举报仅用于隔离治理界面验收。", status, created_at: stamp, updated_at: stamp }));
  const requests: FinalQaRequest[] = [], unknownRequests: string[] = [];
  type Gate = { promise: Promise<void>; release: () => void };
  const readGates = new Map<string, Gate>(), mutationGates = new Map<string, Gate>();
  const canonical = (path: string) => { const [name, query] = path.split("?"); if (!query) return name; const params = new URLSearchParams(query); params.sort(); return `${name}?${params}`; };
  const hold = (map: Map<string, Gate>, path: string) => { path = canonical(path); if (map.has(path)) return; let release!: () => void; const promise = new Promise<void>((done) => { release = done; }); map.set(path, { promise, release }); };
  const release = (map: Map<string, Gate>, path: string) => { path = canonical(path); map.get(path)?.release(); map.delete(path); };
  await page.route(`**${finalQaCover}`, (route) => route.fulfill({ contentType: "image/svg+xml", body: abstractCover }));
  await page.route(`**${finalQaBrokenCover}`, (route) => route.fulfill({ status: 404, contentType: "text/plain", body: "expected isolated missing cover" }));
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request(), url = new URL(request.url()), path = url.pathname, method = request.method();
    requests.push({ method, path, query: url.search });
    const known = method === "GET" ? ["/api/v1/auth/me", "/api/v1/categories", "/api/v1/videos", "/api/v1/me/videos", "/api/v1/me/following/videos", "/api/v1/me/favorites", "/api/v1/me/creator/stats", "/api/v1/me/notifications", "/api/v1/admin/reports"].includes(path) || /^\/api\/v1\/(users\/\d+(\/videos)?|videos\/\d+(\/comments)?)$/.test(path)
      : method === "POST" ? ["/api/v1/auth/login", "/api/v1/auth/register", "/api/v1/auth/logout", "/api/v1/videos", "/api/v1/me/password", "/api/v1/me/notifications/read-all"].includes(path) || /^\/api\/v1\/(users\/\d+\/follow|videos\/\d+\/(comments|like|favorite|reports|retry|subtitles))$/.test(path)
      : method === "PATCH" ? path === "/api/v1/me/profile" || /^\/api\/v1\/(videos\/\d+(\/subtitles\/\d+\/default)?|me\/notifications\/\d+\/read|admin\/reports\/\d+)$/.test(path)
      : method === "DELETE" && /^\/api\/v1\/videos\/\d+(\/(comments|subtitles)\/\d+)?$/.test(path);
    if (!known) { unknownRequests.push(`${method} ${path}${url.search}`); await route.fulfill({ status: 404, json: { error: "Unknown fail-closed Phase 6 API fixture path" } }); return; }
    const gates = method === "GET" ? readGates : mutationGates;
    const gate = gates.get(canonical(path + url.search)) || gates.get(path);
    if (gate) await gate.promise;
    if (request.failure()) return;
    if (scenario === "slow" && path !== "/api/v1/auth/me") await new Promise((resolve) => setTimeout(resolve, 200));
    let data: unknown;
    const errorRead = scenario === "error" && method === "GET" && path !== "/api/v1/auth/me" && path !== "/api/v1/categories" && !(path === "/api/v1/me/notifications" && url.searchParams.get("page_size") === "1");
    if (errorRead) { await route.fulfill({ status: 500, json: { error: "隔离验收：读取失败_LONG_ASCII_0123456789_".repeat(6) } }); return; }
    if (method !== "GET" && path !== "/api/v1/auth/login" && path !== "/api/v1/auth/register") expect(request.headers()["x-csrf-token"]).toBe(csrf);
    const pageNumber = Math.max(1, Number(url.searchParams.get("page") || 1));
    const size = Math.max(1, Number(url.searchParams.get("page_size") || url.searchParams.get("limit") || "36"));
    const paged = <T,>(values: T[]) => ({ items: values.slice((pageNumber - 1) * size, pageNumber * size), page: pageNumber, page_size: size, total: values.length, has_next: pageNumber * size < values.length });
    if (path === "/api/v1/auth/me" && method === "GET") {
      if (!signedIn || expired) { await route.fulfill({ status: 401, json: { error: "未登录" } }); return; }
      data = { user, csrf_token: csrf };
    } else if ((path === "/api/v1/auth/login" || path === "/api/v1/auth/register") && method === "POST") {
      signedIn = true; expired = false; user = { ...user, username: request.postDataJSON().username }; data = { user, csrf_token: csrf };
    } else if (path === "/api/v1/auth/logout" && method === "POST") { signedIn = false; data = { logged_out: true }; }
    else if (expired) { await route.fulfill({ status: 401, json: { error: "登录已失效" } }); return; }
    else if (path === "/api/v1/categories" && method === "GET") data = ["音乐", "生活", "纪录"];
    else if (path === "/api/v1/videos" && method === "GET") data = paged(items.filter((item) => (!url.searchParams.get("category") || item.category === url.searchParams.get("category")) && (!url.searchParams.get("q") || item.title.includes(url.searchParams.get("q")!))));
    else if (path === "/api/v1/me/videos" && method === "GET") data = paged(items);
    else if ((path === "/api/v1/me/following/videos" || path === "/api/v1/me/favorites") && method === "GET") data = paged(items);
    else if (path === "/api/v1/me/creator/stats" && method === "GET") {
      const stats: CreatorStats = { videos_count: items.length, followers_count: profile.followers_count, views_count: empty ? 0 : long ? 1234567890 : 128000, likes_count: empty ? 0 : 1280, favorites_count: empty ? 0 : 640, comments_count: empty ? 0 : 320, public_count: items.length, unlisted_count: 0, private_count: 0, processing_count: items.filter((item) => item.processing_status === "pending" || item.processing_status === "processing").length, recent_videos: items.slice(0, 5) }; data = stats;
    } else if (/^\/api\/v1\/users\/\d+\/videos$/.test(path) && method === "GET") data = paged(items);
    else if (/^\/api\/v1\/users\/\d+$/.test(path) && method === "GET") data = { ...profile, id: Number(path.split("/").at(-1)) };
    else if (/^\/api\/v1\/users\/\d+\/follow$/.test(path) && method === "POST") { profile.followed = !profile.followed; data = { active: profile.followed }; }
    else if (/^\/api\/v1\/videos\/\d+\/comments$/.test(path)) {
      if (method === "GET") data = comments;
      else if (method === "POST") { const comment: Comment = { id: comments.length + 10, video_id: Number(path.split("/")[4]), user_id: user.id, username: user.username, avatar_url: "", content: request.postDataJSON().content, created_at: stamp }; comments = [comment, ...comments]; data = comment; }
    } else if (/^\/api\/v1\/videos\/\d+\/comments\/\d+$/.test(path) && method === "DELETE") { comments = comments.filter((item) => item.id !== Number(path.split("/").at(-1))); data = { deleted: true }; }
    else if (/^\/api\/v1\/videos\/\d+\/(like|favorite)$/.test(path) && method === "POST") { const key = path.endsWith("like") ? "liked" : "favorited"; first[key] = !first[key]; data = { active: first[key] }; }
    else if (/^\/api\/v1\/videos\/\d+\/reports$/.test(path) && method === "POST") data = { ...reports[0], ...request.postDataJSON(), id: 99, status: "pending" };
    else if (/^\/api\/v1\/videos\/\d+\/retry$/.test(path) && method === "POST") { const id = Number(path.split("/")[4]); items = items.map((item) => item.id === id ? { ...item, processing_status: "pending", processing_stage: "queued", processing_progress: 0 } : item); data = { processing_status: "pending" }; }
    else if (/^\/api\/v1\/videos\/\d+\/subtitles/.test(path)) {
      const id = Number(path.split("/")[4]), item = items.find((value) => value.id === id)!;
      if (method === "POST") { const track: SubtitleTrack = { id: 99, language: "zh-CN", label: "中文", url: "/phase6-test.vtt", is_default: !item.subtitle_tracks.length }; item.subtitle_tracks = [...item.subtitle_tracks, track]; data = track; }
      else if (method === "PATCH" && path.endsWith("/default")) { const trackID = Number(path.split("/")[6]); item.subtitle_tracks = item.subtitle_tracks.map((track) => ({ ...track, is_default: track.id === trackID })); data = item.subtitle_tracks; }
      else if (method === "DELETE") { const trackID = Number(path.split("/")[6]); item.subtitle_tracks = item.subtitle_tracks.filter((track) => track.id !== trackID); data = item.subtitle_tracks; }
    } else if (/^\/api\/v1\/videos\/\d+$/.test(path)) {
      const id = Number(path.split("/").at(-1));
      if (method === "GET") data = id === 1 ? first : { ...first, id, title: long ? "SECOND_ASCII_0123456789_".repeat(18) : `RC 作品 ${id}` };
      else if (method === "PATCH") data = items.find((item) => item.id === id) || first;
      else if (method === "DELETE") { items = items.filter((item) => item.id !== id); data = { deleted: true }; }
    } else if (path === "/api/v1/videos" && method === "POST") data = { ...first, id: 100, processing_status: "pending", processing_stage: "queued", processing_progress: 0 };
    else if (path === "/api/v1/me/profile" && method === "PATCH") data = user;
    else if (path === "/api/v1/me/password" && method === "POST") data = { changed: true };
    else if (path === "/api/v1/me/notifications" && method === "GET") data = { ...paged(notifications), unread_count: notifications.filter((item) => !item.read_at).length };
    else if (path === "/api/v1/me/notifications/read-all" && method === "POST") { notifications = notifications.map((item) => ({ ...item, read_at: item.read_at || stamp })); data = { read: true }; }
    else if (/^\/api\/v1\/me\/notifications\/\d+\/read$/.test(path) && method === "PATCH") { const id = Number(path.split("/")[5]); notifications = notifications.map((item) => item.id === id ? { ...item, read_at: stamp } : item); data = { read: true }; }
    else if (path === "/api/v1/admin/reports" && method === "GET") data = paged(reports.filter((item) => !url.searchParams.get("status") || item.status === url.searchParams.get("status")));
    else if (/^\/api\/v1\/admin\/reports\/\d+$/.test(path) && method === "PATCH") { const id = Number(path.split("/").at(-1)); reports = reports.map((item) => item.id === id ? { ...item, status: request.postDataJSON().status, updated_at: stamp } : item); data = reports.find((item) => item.id === id); }
    if (data === undefined) { unknownRequests.push(`${method} ${path}${url.search}`); await route.fulfill({ status: 404, json: { error: "Unknown fail-closed Phase 6 API fixture path" } }); return; }
    await route.fulfill({ json: { data } });
  });
  await page.route("**/phase6-test.vtt", (route) => route.fulfill({ contentType: "text/vtt", body: "WEBVTT\n\n00:00:00.000 --> 00:00:20.000\n隔离字幕验收\n" }));
  return { video: first, profile, requests, unknownRequests, reads: (path) => requests.filter((item) => item.method === "GET" && item.path === path).length, gateRead: (path) => hold(readGates, path), releaseRead: (path) => release(readGates, path), gateMutation: (path) => hold(mutationGates, path), releaseMutation: (path) => release(mutationGates, path), expire: () => { expired = true; }, setVideoStatus: (status) => { first.processing_status = status; first.processing_stage = status === "ready" ? "done" : status === "processing" ? "transcoding" : status === "pending" ? "queued" : "failed"; first.processing_progress = status === "ready" ? 100 : status === "processing" ? 45 : 0; if (status === "failed") first.processing_error = "隔离验收：媒体处理失败。"; } };
}
