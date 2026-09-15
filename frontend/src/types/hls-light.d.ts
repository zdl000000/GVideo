// The runtime imports the light build to stay inside the HLS bundle budget;
// it shares the public API surface of the main entry, so reuse those types
// instead of falling back to `any`.
//
// Caveat: these types mirror the full build. The light build ships stubs for
// subtitle/alternate-audio/EME/CMCD controllers, so touching those APIs still
// type-checks but does nothing at runtime. The app does not use them (native
// TextTracks handle subtitles); keep it that way or switch back to "hls.js".
declare module "hls.js/light" {
  export * from "hls.js";
  export { default } from "hls.js";
}
