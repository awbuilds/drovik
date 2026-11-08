/*
 * Drovik site — theme bootstrap.
 *
 * Runs in <head>, before the stylesheet is applied, so a stored or system
 * theme never flashes the wrong surface. It deliberately does only this:
 * all other behaviour lives in site.js and must degrade without it.
 */
(function () {
  "use strict";

  var root = document.documentElement;

  try {
    root.classList.add("js");
  } catch (error) {
    /* The page still renders with the dark stylesheet. */
  }

  var theme = "dark";
  try {
    var stored = window.localStorage.getItem("drovik-site-theme");
    if (stored === "light" || stored === "dark") {
      theme = stored;
    } else if (
      window.matchMedia &&
      window.matchMedia("(prefers-color-scheme: light)").matches
    ) {
      theme = "light";
    }
  } catch (error) {
    /* Storage or matchMedia can fail in hardened browsers; dark is safe. */
    theme = "dark";
  }

  try {
    root.setAttribute("data-theme", theme);
  } catch (error) {
    root.setAttribute("data-theme", "dark");
    theme = "dark";
  }

  try {
    var meta = document.querySelector('meta[name="theme-color"]');
    if (meta) {
      meta.setAttribute("content", theme === "light" ? "#f4f1ea" : "#07090c");
    }
  } catch (error) {
    /* A missing meta tag must not prevent the theme from being applied. */
  }
})();
