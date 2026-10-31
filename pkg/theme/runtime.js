/*
 * The runtime of an exported docz page (DESIGN-0025 §5). Plain ES2020, no
 * build step, one global: `docz`. A page works without it: every panel
 * shows, stacked. With it, the page gains:
 *
 *   - the light/dark toggle (.mode-toggle), remembered as docz:mode;
 *   - tabs (nav.tabs button[data-tab] over section.panel#p-<tab>), with
 *     click, #p-<tab> and data-tab links, arrow keys, Home and End, a fade
 *     on an overflowed edge, and the last tab remembered per document as
 *     docz:<id>:tab;
 *   - a copy button on every code block;
 *   - the IMPL task filter (.filters button[data-filter]), remembered as
 *     docz:<id>:filter;
 *   - in-page links that open the tab and the expandable they point into.
 *
 * The document id is <html data-doc>. Colours are read only through
 * docz.palette(), so the theme decides them. Every storage access is in a
 * try: file:// pages and private windows throw.
 */
(function () {
  "use strict";

  var root = document.documentElement;
  var doc = root.getAttribute("data-doc") || "doc";
  var dark = window.matchMedia
    ? window.matchMedia("(prefers-color-scheme: dark)")
    : null;
  var modeListeners = [];

  function $(sel, ctx) {
    return (ctx || document).querySelector(sel);
  }

  function $$(sel, ctx) {
    return Array.prototype.slice.call(
      (ctx || document).querySelectorAll(sel),
    );
  }

  function load(key) {
    try {
      return window.localStorage.getItem(key);
    } catch (e) {
      return null;
    }
  }

  function save(key, value) {
    try {
      window.localStorage.setItem(key, value);
    } catch (e) {
      /* storage is optional */
    }
  }

  /* Mode */

  function mode() {
    var set = root.getAttribute("data-mode");
    if (set === "light" || set === "dark") return set;
    return dark && dark.matches ? "dark" : "light";
  }

  function announce() {
    var m = mode();
    modeListeners.forEach(function (fn) {
      try {
        fn(m);
      } catch (e) {
        if (window.console) console.error(e);
      }
    });
    document.dispatchEvent(
      new CustomEvent("docz:mode", { detail: { mode: m } }),
    );
  }

  function setMode(m) {
    root.setAttribute("data-mode", m);
    save("docz:mode", m);
    labelToggle();
    announce();
  }

  function labelToggle() {
    var next = mode() === "dark" ? "Light" : "Dark";
    $$(".mode-toggle").forEach(function (b) {
      b.textContent = next;
      b.setAttribute("aria-label", "Switch to " + next.toLowerCase() + " mode");
    });
  }

  var stored = load("docz:mode");
  if (stored === "light" || stored === "dark") {
    root.setAttribute("data-mode", stored);
  }
  if (dark && dark.addEventListener) {
    dark.addEventListener("change", function () {
      if (!root.hasAttribute("data-mode")) {
        labelToggle();
        announce();
      }
    });
  }

  /* Palette: a token's value now. Call it while drawing; never cache it
     across a mode change. */

  function palette(names) {
    var style = getComputedStyle(root);
    var out = {};
    names.forEach(function (n) {
      out[n] = style.getPropertyValue(n).trim();
    });
    return out;
  }

  /* Tabs */

  var nav = $("nav.tabs");

  function tabButton(name) {
    return nav ? $('button[data-tab="' + cssEscape(name) + '"]', nav) : null;
  }

  function cssEscape(s) {
    return window.CSS && CSS.escape ? CSS.escape(s) : s.replace(/"/g, '\\"');
  }

  function edges() {
    if (!nav) return;
    var more = nav.scrollWidth > nav.clientWidth + 2;
    nav.classList.toggle("more", more);
    nav.classList.toggle("mid", nav.scrollLeft > 8);
    nav.classList.toggle(
      "end",
      nav.scrollLeft + nav.clientWidth >= nav.scrollWidth - 8,
    );
  }

  function showTab(name, keepScroll) {
    var button = tabButton(name);
    if (!button) return false;
    $$("button[data-tab]", nav).forEach(function (b) {
      var on = b === button;
      b.setAttribute("aria-selected", on ? "true" : "false");
      b.tabIndex = on ? 0 : -1;
    });
    $$("section.panel").forEach(function (p) {
      p.classList.toggle("on", p.id === "p-" + name);
    });
    save("docz:" + doc + ":tab", name);
    if (button.scrollIntoView) {
      button.scrollIntoView({ block: "nearest", inline: "center" });
    }
    if (!keepScroll) window.scrollTo(0, 0);
    edges();
    return true;
  }

  function panelOf(el) {
    while (el && !(el.matches && el.matches("section.panel"))) {
      el = el.parentElement;
    }
    return el;
  }

  function reveal(el) {
    var panel = panelOf(el);
    if (panel && panel.id.indexOf("p-") === 0) {
      showTab(panel.id.slice(2), true);
    }
    for (var p = el; p; p = p.parentElement) {
      if (p.tagName === "DETAILS") p.open = true;
    }
    el.scrollIntoView({ block: "start" });
  }

  function restore() {
    var hash = decodeURIComponent(location.hash.slice(1));
    if (hash) {
      if (hash.indexOf("p-") === 0 && showTab(hash.slice(2))) return;
      if (showTab(hash)) return;
      var target = document.getElementById(hash);
      if (target) {
        reveal(target);
        return;
      }
    }
    var last = load("docz:" + doc + ":tab");
    if (last && showTab(last)) return;
    var first = nav && $("button[data-tab]", nav);
    if (first) showTab(first.getAttribute("data-tab"));
  }

  if (nav) {
    nav.setAttribute("role", "tablist");
    $$("button[data-tab]", nav).forEach(function (b) {
      b.setAttribute("role", "tab");
      b.addEventListener("click", function () {
        showTab(b.getAttribute("data-tab"));
      });
    });
    nav.addEventListener("keydown", function (e) {
      var buttons = $$("button[data-tab]", nav);
      var i = buttons.indexOf(document.activeElement);
      if (i < 0) return;
      var j = -1;
      if (e.key === "ArrowRight") j = i + 1;
      else if (e.key === "ArrowLeft") j = i - 1;
      else if (e.key === "Home") j = 0;
      else if (e.key === "End") j = buttons.length - 1;
      if (j < 0 || j >= buttons.length) return;
      e.preventDefault();
      buttons[j].focus();
      showTab(buttons[j].getAttribute("data-tab"), true);
    });
    nav.addEventListener(
      "wheel",
      function (e) {
        if (
          Math.abs(e.deltaY) > Math.abs(e.deltaX) &&
          nav.scrollWidth > nav.clientWidth
        ) {
          nav.scrollLeft += e.deltaY;
          e.preventDefault();
        }
      },
      { passive: false },
    );
    nav.addEventListener("scroll", edges, { passive: true });
    window.addEventListener("resize", edges);
  }

  /* In-page links: a tab name switches tabs; any other #id opens the tab
     and the expandables that hold it. */

  document.addEventListener("click", function (e) {
    var a = e.target.closest ? e.target.closest("a[href^='#'], [data-tab]") : null;
    if (!a || (nav && nav.contains(a))) return;
    var name = a.getAttribute("data-tab");
    if (!name) {
      name = decodeURIComponent(a.getAttribute("href").slice(1));
    }
    if (name.indexOf("p-") === 0 && tabButton(name.slice(2))) {
      name = name.slice(2);
    }
    if (tabButton(name)) {
      e.preventDefault();
      showTab(name);
      return;
    }
    var target = document.getElementById(name);
    if (target) {
      e.preventDefault();
      reveal(target);
      if (history.replaceState) history.replaceState(null, "", "#" + name);
    }
  });

  /* Copy buttons */

  function addCopy(pre) {
    var code = $("code", pre);
    if (!code || $(".copy", pre)) return;
    var b = document.createElement("button");
    b.type = "button";
    b.className = "copy";
    b.textContent = "Copy";
    b.addEventListener("click", function () {
      var done = function (label) {
        b.textContent = label;
        setTimeout(function () {
          b.textContent = "Copy";
        }, 1500);
      };
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(code.textContent).then(
          function () {
            done("Copied");
          },
          function () {
            done("Failed");
          },
        );
      } else {
        done("Failed");
      }
    });
    pre.appendChild(b);
  }

  /* IMPL task filter */

  function setFilter(panel, value) {
    panel.setAttribute("data-task-filter", value);
    $$(".filters button[data-filter]", panel).forEach(function (b) {
      b.setAttribute(
        "aria-pressed",
        b.getAttribute("data-filter") === value ? "true" : "false",
      );
    });
  }

  function initFilters() {
    var saved = load("docz:" + doc + ":filter");
    $$(".filters").forEach(function (bar) {
      var panel = panelOf(bar) || bar.parentElement;
      bar.addEventListener("click", function (e) {
        var b = e.target.closest("button[data-filter]");
        if (!b) return;
        var value = b.getAttribute("data-filter");
        setFilter(panel, value);
        save("docz:" + doc + ":filter", value);
      });
      setFilter(panel, saved === "open" || saved === "done" ? saved : "all");
    });
  }

  /* Print shows every expandable open, and puts them back afterwards. */

  var openedForPrint = [];
  window.addEventListener("beforeprint", function () {
    openedForPrint = $$("details:not([open])");
    openedForPrint.forEach(function (d) {
      d.open = true;
    });
  });
  window.addEventListener("afterprint", function () {
    openedForPrint.forEach(function (d) {
      d.open = false;
    });
    openedForPrint = [];
  });

  /* Start */

  function start() {
    root.classList.add("js");
    $$(".mode-toggle").forEach(function (b) {
      b.addEventListener("click", function () {
        setMode(mode() === "dark" ? "light" : "dark");
      });
    });
    labelToggle();
    $$("pre:not(.mermaid)").forEach(addCopy);
    initFilters();
    restore();
    setTimeout(edges, 0);
  }

  window.docz = {
    mode: mode,
    setMode: setMode,
    onMode: function (fn) {
      modeListeners.push(fn);
    },
    palette: palette,
    showTab: showTab,
  };

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", start);
  } else {
    start();
  }
})();
