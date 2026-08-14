import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, api, setCSRFToken } from "./api";

type ProgressListener = (event: ProgressEvent<XMLHttpRequestEventTarget>) => void;
type EventListener = (event?: Event) => void;

class MockXMLHttpRequest {
  static last: MockXMLHttpRequest;

  readonly upload = {
    addEventListener: vi.fn((event: string, listener: ProgressListener) => {
      if (event === "progress") this.progressListener = listener;
    })
  };

  readonly open = vi.fn((method: string, path: string) => {
    this.method = method;
    this.path = path;
  });

  readonly setRequestHeader = vi.fn((name: string, value: string) => {
    this.headers[name] = value;
  });

  readonly send = vi.fn((_body: FormData) => undefined);
  readonly abort = vi.fn(() => {
    this.aborted = true;
    this.emit("abort");
  });

  status = 0;
  responseText = "";
  method = "";
  path = "";
  withCredentials = false;
  headers: Record<string, string> = {};
  aborted = false;
  private progressListener?: ProgressListener;
  private readonly listeners = new Map<string, EventListener>();

  constructor() {
    MockXMLHttpRequest.last = this;
  }

  addEventListener(event: string, listener: EventListener) {
    this.listeners.set(event, listener);
  }

  emit(event: string, payload?: Event) {
    this.listeners.get(event)?.(payload);
  }

  emitProgress(loaded: number, total: number, lengthComputable = true) {
    this.progressListener?.({ loaded, total, lengthComputable } as ProgressEvent<XMLHttpRequestEventTarget>);
  }

  complete(status: number, payload: unknown) {
    this.status = status;
    this.responseText = JSON.stringify(payload);
    this.emit("load");
  }
}

function jsonResponse(payload: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: vi.fn().mockResolvedValue(payload)
  };
}

describe("frontend API client", () => {
  let fetchMock: ReturnType<typeof vi.fn>;
  let dispatchEvent: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    dispatchEvent = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("window", { dispatchEvent });
    vi.stubGlobal("XMLHttpRequest", MockXMLHttpRequest);
    setCSRFToken("");
  });

  afterEach(() => {
    setCSRFToken("");
    vi.unstubAllGlobals();
  });

  it("serializes JSON requests, includes CSRF, and unwraps the data envelope", async () => {
    setCSRFToken("csrf-123");
    fetchMock.mockResolvedValue(jsonResponse({ data: { changed: true }, request_id: "req-1" }));

    await expect(api.changePassword("old", "new")).resolves.toEqual({ changed: true });

    const [path, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(path).toBe("/api/v1/me/password");
    expect(init.method).toBe("POST");
    expect(init.credentials).toBe("include");
    expect(new Headers(init.headers).get("Content-Type")).toBe("application/json");
    expect(new Headers(init.headers).get("X-CSRF-Token")).toBe("csrf-123");
    expect(JSON.parse(String(init.body))).toEqual({ current_password: "old", new_password: "new" });
  });

  it("keeps FormData requests free of a forced JSON content type and forwards abort signals", async () => {
    const form = new FormData();
    form.append("title", "demo");
    const controller = new AbortController();
    fetchMock.mockResolvedValue(jsonResponse({ data: { deleted: true }, request_id: "req-2" }));

    await expect(api.upload(form, undefined, controller.signal)).resolves.toEqual({ deleted: true });

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(init.body).toBe(form);
    expect(new Headers(init.headers).has("Content-Type")).toBe(false);
    expect(init.signal).toBe(controller.signal);
  });

  it("passes encoded IDs and query parameters to endpoint helpers", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ data: [], request_id: "req-3" }));
    const params = new URLSearchParams({ page: "2", category: "动画片" });

    await api.creatorVideos("author/name", params);

    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/v1/users/author%2Fname/videos?page=2&category=%E5%8A%A8%E7%94%BB%E7%89%87");
  });

  it("raises ApiError with server details and announces expired authentication", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ error: "session expired", request_id: "req-401" }, 401));

    const error = await api.categories().catch((value: unknown) => value);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ message: "session expired", status: 401, requestId: "req-401" });
    expect(dispatchEvent).toHaveBeenCalledOnce();
    expect(dispatchEvent.mock.calls[0]?.[0].type).toBe("gvideo-auth-expired");
  });

  it("does not announce expiry for the auth bootstrap endpoint", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ error: "session expired", request_id: "req-401" }, 401));

    await expect(api.me()).rejects.toBeInstanceOf(ApiError);

    expect(dispatchEvent).not.toHaveBeenCalled();
  });

  it("handles non-JSON fetch responses without leaking a parsing error", async () => {
    fetchMock.mockResolvedValue({
      ok: false,
      status: 502,
      json: vi.fn().mockRejectedValue(new SyntaxError("invalid JSON"))
    });

    await expect(api.categories()).rejects.toMatchObject({ message: "请求失败，请稍后重试", status: 502 });
  });

  it("uploads with progress and resolves the response envelope", async () => {
    setCSRFToken("csrf-upload");
    const form = new FormData();
    const progress = vi.fn();
    const promise = api.upload(form, progress);

    MockXMLHttpRequest.last.emitProgress(40, 100);
    MockXMLHttpRequest.last.complete(201, { data: { id: 9 }, request_id: "req-upload" });

    await expect(promise).resolves.toEqual({ id: 9 });
    expect(progress).toHaveBeenCalledWith(40, 100);
    expect(MockXMLHttpRequest.last.open).toHaveBeenCalledWith("POST", "/api/v1/videos");
    expect(MockXMLHttpRequest.last.setRequestHeader).toHaveBeenCalledWith("X-CSRF-Token", "csrf-upload");
    expect(MockXMLHttpRequest.last.send).toHaveBeenCalledWith(form);
  });

  it("rejects uploads with ApiError when the server returns invalid JSON", async () => {
    const promise = api.upload(new FormData(), vi.fn());
    MockXMLHttpRequest.last.status = 502;
    MockXMLHttpRequest.last.responseText = "not-json";
    MockXMLHttpRequest.last.emit("load");

    await expect(promise).rejects.toMatchObject({ status: 502 });
  });

  it("rejects an in-flight upload when its AbortSignal fires", async () => {
    const controller = new AbortController();
    const promise = api.upload(new FormData(), vi.fn(), controller.signal);

    controller.abort();

    await expect(promise).rejects.toMatchObject({ name: "AbortError" });
    expect(MockXMLHttpRequest.last.abort).toHaveBeenCalledOnce();
    expect(MockXMLHttpRequest.last.aborted).toBe(true);
  });
});
