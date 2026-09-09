import { gzipSync } from "node:zlib";
import { readdir, readFile, stat } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const MAX_HLS_BYTES = 550_000;
const MAX_HLS_GZIP_BYTES = 170_000;
const frontendDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const assetsDir = path.join(frontendDir, "dist", "assets");
const entries = await readdir(assetsDir);
const hlsChunks = entries.filter((name) => /^hls-[A-Za-z0-9_-]+\.js$/.test(name));

if (hlsChunks.length !== 1) {
  throw new Error(`expected exactly one hls-*.js chunk in ${assetsDir}, found ${hlsChunks.length}: ${hlsChunks.join(", ") || "none"}`);
}

const chunkPath = path.join(assetsDir, hlsChunks[0]);
const [{ size }, contents] = await Promise.all([stat(chunkPath), readFile(chunkPath)]);
const gzipBytes = gzipSync(contents, { level: 9 }).byteLength;
const format = (bytes) => `${(bytes / 1000).toFixed(2)} kB`;

console.log(`HLS bundle: ${hlsChunks[0]} raw=${format(size)} gzip=${format(gzipBytes)}`);

const failures = [];
if (size > MAX_HLS_BYTES) failures.push(`raw size ${format(size)} exceeds ${format(MAX_HLS_BYTES)}`);
if (gzipBytes > MAX_HLS_GZIP_BYTES) failures.push(`gzip size ${format(gzipBytes)} exceeds ${format(MAX_HLS_GZIP_BYTES)}`);
if (failures.length > 0) {
  throw new Error(`HLS bundle budget exceeded: ${failures.join("; ")}`);
}
