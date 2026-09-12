import { expect, test, type APIResponse } from "@playwright/test";

// Mirrors the SPA policy in frontend/nginx.conf: style-src without
// 'unsafe-inline' and img-src without data: are the tightened SEC-03c policy.
const SPA_CSP = [
  "default-src 'self'",
  "script-src 'self'",
  "style-src 'self'",
  "img-src 'self' blob:",
  "media-src 'self' blob:",
  "worker-src 'self' blob:",
  "child-src 'self' blob:",
  "connect-src 'self'",
  "frame-ancestors 'none'",
  "base-uri 'self'",
  "form-action 'self'",
  "object-src 'none'"
].join("; ");

// Mirrors the backend middleware policy: API and media responses keep the
// deny-all CSP; the gateway passes it through untouched.
const API_CSP = "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'";

const BASE_HEADERS = {
  "x-content-type-options": "nosniff",
  "x-frame-options": "DENY",
  "referrer-policy": "strict-origin-when-cross-origin",
  "permissions-policy": "camera=(), microphone=(), geolocation=()"
};

const SPA_HEADERS = { ...BASE_HEADERS, "content-security-policy": SPA_CSP };
const API_HEADERS = { ...BASE_HEADERS, "content-security-policy": API_CSP };

function expectSecurityHeaders(response: APIResponse, expected: Record<string, string>) {
  for (const [name, value] of Object.entries(expected)) {
    const values = response
      .headersArray()
      .filter((header) => header.name.toLowerCase() === name)
      .map((header) => header.value);
    expect(values, `${name} on ${response.url()}`).toHaveLength(1);
    expect(values[0]).toBe(value);
  }
}

test("SPA documents carry the tightened CSP exactly once", async ({ request }) => {
  const response = await request.get("/");
  expect(response.status()).toBe(200);
  expectSecurityHeaders(response, SPA_HEADERS);
  // server_tokens off: the Server header must not leak the nginx version.
  expect(response.headers()["server"] ?? "").not.toMatch(/\d/);
});

test("SPA fallback for deep links keeps the header set", async ({ request }) => {
  const response = await request.get("/upload");
  expect(response.status()).toBe(200);
  expectSecurityHeaders(response, SPA_HEADERS);
});

test("theme bootstrap script keeps the header set", async ({ request }) => {
  const response = await request.get("/theme-init.js");
  expect(response.status()).toBe(200);
  expectSecurityHeaders(response, SPA_HEADERS);
});

test("proxied API responses keep backend headers exactly once", async ({ request }) => {
  const response = await request.get("/api/v1/videos?page=1&page_size=1");
  expect(response.status()).toBe(200);
  expectSecurityHeaders(response, API_HEADERS);
});

test("proxied health endpoints keep backend headers exactly once", async ({ request }) => {
  for (const path of ["/healthz", "/livez", "/readyz"]) {
    const response = await request.get(path);
    expect(response.status(), path).toBe(200);
    expectSecurityHeaders(response, API_HEADERS);
  }
});

test("media not-found responses keep a single header set", async ({ request }) => {
  const response = await request.get("/media/e2e-missing-media.mp4");
  expect(response.status()).toBe(404);
  expectSecurityHeaders(response, API_HEADERS);
});
