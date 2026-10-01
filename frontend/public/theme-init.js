// Applies the persisted theme before the app bundle loads so the first paint
// uses the saved light/dark scheme. Kept as an external file because the
// production Content-Security-Policy forbids inline scripts.
(() => {
  const saved = localStorage.getItem("gvideo-theme");
  const theme = saved === "light" || saved === "dark" ? saved : "light";
  document.documentElement.dataset.theme = theme;
  document.documentElement.style.colorScheme = theme;
  document.querySelector('meta[name="theme-color"]')?.setAttribute("content", theme === "dark" ? "#080a0d" : "#f7f8fa");
})();
