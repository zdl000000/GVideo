// Light build: the app renders subtitles through native TextTracks and does not
// use alternate audio, EME/DRM, CMCD or variable substitution, so the features
// the light build drops are unused while its smaller chunk keeps the HLS bundle
// inside the FE-PERF-01 budget.
export async function loadHLSModule() {
  return import("hls.js/light");
}
