/*
 * Drovik site — interaction layer.
 *
 * Everything here is an enhancement. The pages are readable and navigable
 * with JavaScript disabled; the `js` class added in <head> opts controls in.
 * Each feature initialises in its own try/catch so one failure cannot take
 * the rest of the page down, and nothing claims success it did not observe.
 */
(function () {
  "use strict";

  var STORAGE_KEY = "drovik-site-theme";
  var root = document.documentElement;

  function safely(label, fn) {
    try {
      fn();
    } catch (error) {
      try {
        console.error("[drovik-site] " + label + " failed", error);
      } catch (ignored) {
        /* Logging must never become the failure. */
      }
    }
  }

  function currentTheme() {
    return root.getAttribute("data-theme") === "light" ? "light" : "dark";
  }

  function applyTheme(theme, persist) {
    root.setAttribute("data-theme", theme);

    try {
      var meta = document.querySelector('meta[name="theme-color"]');
      if (meta) {
        meta.setAttribute("content", theme === "light" ? "#f4f1ea" : "#07090c");
      }
    } catch (error) {
      /* The theme itself is applied; the browser chrome colour is optional. */
    }

    if (persist) {
      try {
        window.localStorage.setItem(STORAGE_KEY, theme);
      } catch (error) {
        /* Private mode: the choice applies for this page, just not later. */
      }
    }

    try {
      var toggle = document.querySelector(".theme-toggle");
      if (toggle) {
        toggle.setAttribute("aria-checked", theme === "dark" ? "true" : "false");
      }
    } catch (error) {
      /* A missing toggle does not affect the applied theme. */
    }
  }

  function storedTheme() {
    try {
      var value = window.localStorage.getItem(STORAGE_KEY);
      return value === "light" || value === "dark" ? value : null;
    } catch (error) {
      return null;
    }
  }

  function initTheme() {
    var toggle = document.querySelector(".theme-toggle");
    if (toggle) {
      toggle.setAttribute("aria-checked", currentTheme() === "dark" ? "true" : "false");
      toggle.addEventListener("click", function () {
        var next = currentTheme() === "dark" ? "light" : "dark";
        applyTheme(next, true);
      });
    }

    var media = window.matchMedia ? window.matchMedia("(prefers-color-scheme: light)") : null;
    if (!media) return;

    var follow = function () {
      if (storedTheme()) return;
      applyTheme(media.matches ? "light" : "dark", false);
    };

    if (typeof media.addEventListener === "function") {
      media.addEventListener("change", follow);
    } else if (typeof media.addListener === "function") {
      media.addListener(follow);
    }
  }

  function initMenu() {
    var button = document.querySelector(".menu-toggle");
    var nav = document.getElementById("site-nav");
    if (!button || !nav) return;

    function setOpen(open) {
      button.setAttribute("aria-expanded", open ? "true" : "false");
      button.setAttribute("aria-label", open ? "Close menu" : "Open menu");
      nav.classList.toggle("is-open", open);
    }

    button.addEventListener("click", function () {
      setOpen(button.getAttribute("aria-expanded") !== "true");
    });

    nav.addEventListener("click", function (event) {
      var target = event.target;
      if (target && target.closest && target.closest("a")) {
        setOpen(false);
      }
    });

    document.addEventListener("keydown", function (event) {
      if (event.key !== "Escape") return;
      if (button.getAttribute("aria-expanded") !== "true") return;
      setOpen(false);
      button.focus();
    });

    document.addEventListener("click", function (event) {
      if (button.getAttribute("aria-expanded") !== "true") return;
      var target = event.target;
      if (!target) return;
      if (nav.contains(target) || button.contains(target)) return;
      setOpen(false);
    });
  }

  function initTabs() {
    var tablist = document.querySelector("[data-tabs]");
    if (!tablist) return;

    var tabs = Array.prototype.slice.call(tablist.querySelectorAll('[role="tab"]'));
    if (!tabs.length) return;

    var panels = tabs.map(function (tab) {
      return document.getElementById(tab.getAttribute("aria-controls") || "");
    });

    var railItems = Array.prototype.slice.call(
      document.querySelectorAll(".console__rail-item[data-panel]"),
    );

    function activate(index, moveFocus) {
      if (index < 0 || index >= tabs.length) return;

      tabs.forEach(function (tab, position) {
        var selected = position === index;
        tab.setAttribute("aria-selected", selected ? "true" : "false");
        tab.tabIndex = selected ? 0 : -1;
      });

      panels.forEach(function (panel, position) {
        if (!panel) return;
        var selected = position === index;
        panel.hidden = !selected;
        if (selected) {
          panel.classList.remove("is-entering");
          /* Force a restarted, restrained transition without inline styles. */
          void panel.offsetWidth;
          panel.classList.add("is-entering");
        }
      });

      var activePanel = tabs[index].getAttribute("data-panel") || "";
      railItems.forEach(function (item) {
        item.classList.toggle("is-active", item.getAttribute("data-panel") === activePanel);
      });

      if (moveFocus) {
        tabs[index].focus();
      }
    }

    tabs.forEach(function (tab, index) {
      tab.addEventListener("click", function () {
        activate(index, false);
      });
    });

    tablist.addEventListener("keydown", function (event) {
      var index = tabs.indexOf(document.activeElement);
      if (index === -1) return;

      var next = null;
      if (event.key === "ArrowRight" || event.key === "ArrowDown") {
        next = (index + 1) % tabs.length;
      } else if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
        next = (index - 1 + tabs.length) % tabs.length;
      } else if (event.key === "Home") {
        next = 0;
      } else if (event.key === "End") {
        next = tabs.length - 1;
      }

      if (next === null) return;
      event.preventDefault();
      activate(next, true);
    });

    activate(0, false);
  }

  function initCopy() {
    var buttons = Array.prototype.slice.call(document.querySelectorAll("[data-copy-target]"));
    if (!buttons.length) return;

    var clipboard =
      window.navigator &&
      window.navigator.clipboard &&
      typeof window.navigator.clipboard.writeText === "function" &&
      window.isSecureContext;

    if (!clipboard) {
      buttons.forEach(function (button) {
        button.hidden = true;
      });
      return;
    }

    buttons.forEach(function (button) {
      button.hidden = false;
      var targetId = button.getAttribute("data-copy-target") || "";
      var target = document.getElementById(targetId);
      var status = button.parentElement
        ? button.parentElement.querySelector(".cmd__status")
        : null;
      var resetTimer = null;

      if (!target) {
        button.hidden = true;
        return;
      }

      function report(message, state) {
        if (!status) return;
        status.textContent = message;
        status.setAttribute("data-state", state);
      }

      function reset() {
        button.textContent = "Copy";
        report("", "");
        if (resetTimer) {
          window.clearTimeout(resetTimer);
          resetTimer = null;
        }
      }

      button.addEventListener("click", function () {
        var text = (target.textContent || "").replace(/\s+$/, "");
        if (!text) {
          report("Nothing to copy.", "error");
          return;
        }

        try {
          window.navigator.clipboard
            .writeText(text)
            .then(function () {
              button.textContent = "Copied";
              report("Copied to clipboard.", "ok");
              if (resetTimer) window.clearTimeout(resetTimer);
              resetTimer = window.setTimeout(reset, 2200);
            })
            .catch(function () {
              report("Copy failed. Select the command and copy it manually.", "error");
            });
        } catch (error) {
          report("Copy failed. Select the command and copy it manually.", "error");
        }
      });
    });
  }

  function initHeroSequence() {
    /* The CSS handles the one orchestrated entrance; this only marks that
       the page has been painted, keeping the no-JS state fully visible. */
    try {
      root.classList.add("is-ready");
    } catch (error) {
      /* Purely decorative. */
    }
  }

  safely("theme", initTheme);
  safely("menu", initMenu);
  safely("tabs", initTabs);
  safely("copy", initCopy);
  safely("hero", initHeroSequence);
})();
