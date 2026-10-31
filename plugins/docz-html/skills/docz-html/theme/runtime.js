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
 *   - in-page links that open the tab and the expandable they point into;
 *   - the docz-flow, docz-stepper, and docz-scorecard widgets, each drawn
 *     from a JSON island and redrawn when the mode changes.
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
    return Array.prototype.slice.call((ctx || document).querySelectorAll(sel));
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
    var a = e.target.closest
      ? e.target.closest("a[href^='#'], [data-tab]")
      : null;
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

  /* Widgets (DESIGN-0025 §6): a figure.widget holding a JSON island
     (script[type="application/json"][data-widget]), an empty
     .widget-mount, and a .widget-fallback. A drawer builds its DOM with
     createElement and textContent only, so no document text is ever
     parsed as markup, and marks the figure drawn, which hides the
     fallback. An island that fails to parse or to draw leaves the
     fallback in place. SVG colours are read through palette() on every
     draw, so a mode change redraws them. */

  var SVG_NS = "http://www.w3.org/2000/svg";
  var widgetCount = 0;

  var TOKENS = [
    "--color-bg-raised",
    "--color-bg-elevated",
    "--color-border-default",
    "--color-fg-primary",
    "--color-fg-secondary",
    "--color-accent",
    "--color-on-accent",
    "--color-ok",
    "--color-bad",
  ];

  function colours() {
    var p = palette(TOKENS);
    return {
      raised: p["--color-bg-raised"],
      elevated: p["--color-bg-elevated"],
      line: p["--color-border-default"],
      ink: p["--color-fg-primary"],
      ink2: p["--color-fg-secondary"],
      accent: p["--color-accent"],
      onAccent: p["--color-on-accent"],
      ok: p["--color-ok"],
      bad: p["--color-bad"],
    };
  }

  function h(tag, attrs, text) {
    var e = document.createElement(tag);
    setAttrs(e, attrs);
    if (text !== undefined && text !== null) e.textContent = String(text);
    return e;
  }

  function s(tag, attrs, text) {
    var e = document.createElementNS(SVG_NS, tag);
    setAttrs(e, attrs);
    if (text !== undefined && text !== null) e.textContent = String(text);
    return e;
  }

  function setAttrs(e, attrs) {
    if (!attrs) return;
    Object.keys(attrs).forEach(function (k) {
      if (attrs[k] !== undefined && attrs[k] !== null) {
        e.setAttribute(k, String(attrs[k]));
      }
    });
  }

  function clear(e) {
    while (e.firstChild) e.removeChild(e.firstChild);
  }

  function button(label, attrs) {
    var b = h("button", attrs, label);
    b.type = "button";
    return b;
  }

  function activate(e, fn) {
    e.addEventListener("click", fn);
    e.addEventListener("keydown", function (ev) {
      if (ev.key === "Enter" || ev.key === " ") {
        ev.preventDefault();
        fn();
      }
    });
  }

  function isList(v, min) {
    return Array.isArray(v) && v.length >= (min || 0);
  }

  function str(v) {
    return typeof v === "string" && v !== "";
  }

  /* docz-flow */

  function drawFlow(mount, data) {
    var stages = data.stages;
    if (!isList(stages, 2))
      throw new Error("docz-flow: stages needs two or more");
    stages.forEach(function (st) {
      if (!st || !str(st.name) || !str(st.does)) {
        throw new Error("docz-flow: every stage needs name and does");
      }
    });
    var gates = isList(data.gates) ? data.gates : [];
    var id = "docz-w" + ++widgetCount;
    var hot = -1;

    var holder = h("div", { class: "widget-canvas" });
    var detail = h("div", { class: "widget-detail", "aria-live": "polite" });
    if (str(data.label))
      mount.appendChild(h("p", { class: "note" }, data.label));
    mount.appendChild(holder);
    mount.appendChild(detail);

    function showDetail() {
      clear(detail);
      if (hot < 0) {
        detail.appendChild(h("p", { class: "note" }, "Select a stage."));
        return;
      }
      var st = stages[hot];
      detail.appendChild(h("h4", null, st.name));
      detail.appendChild(h("p", null, st.does));
      if (str(st.note)) detail.appendChild(h("p", { class: "note" }, st.note));
    }

    function draw() {
      var c = colours();
      var n = stages.length;
      var W = 640;
      var gap = W / n;
      var H = 100 + (str(data.legend) ? 20 : 0) + (str(data.alert) ? 20 : 0);
      var svg = s("svg", {
        viewBox: "0 0 " + W + " " + H,
        role: "group",
        "aria-label": str(data.title) ? data.title : "Flow",
      });
      var marker = s("marker", {
        id: id + "-arrow",
        markerWidth: 8,
        markerHeight: 8,
        refX: 7,
        refY: 4,
        orient: "auto",
      });
      marker.appendChild(s("path", { d: "M0,0 L8,4 L0,8 z", fill: c.ink2 }));
      var defs = s("defs");
      defs.appendChild(marker);
      svg.appendChild(defs);

      stages.forEach(function (st, i) {
        var x = gap * i + 6;
        var w = gap - 12;
        var on = hot === i;
        var gate = gates.indexOf(i + 1) >= 0;
        var g = s("g", {
          class: "stage",
          tabindex: 0,
          role: "button",
          "aria-pressed": on ? "true" : "false",
          "aria-label": st.name + (gate ? " (gate)" : ""),
        });
        g.appendChild(
          s("rect", {
            x: x,
            y: 14,
            width: w,
            height: 54,
            rx: 9,
            fill: on ? c.accent : c.raised,
            stroke: on ? c.accent : gate ? c.ok : c.line,
            "stroke-width": gate && !on ? 2 : 1,
          }),
        );
        var words = st.name.split(" ");
        g.appendChild(
          s(
            "text",
            {
              x: x + w / 2,
              y: 37,
              "font-size": 11,
              "font-weight": 600,
              "text-anchor": "middle",
              fill: on ? c.onAccent : c.ink,
            },
            words[0],
          ),
        );
        if (words.length > 1) {
          g.appendChild(
            s(
              "text",
              {
                x: x + w / 2,
                y: 53,
                "font-size": 10,
                "text-anchor": "middle",
                fill: on ? c.onAccent : c.ink2,
              },
              words.slice(1).join(" "),
            ),
          );
        }
        activate(g, function () {
          hot = i;
          draw();
          showDetail();
          var again = holder.querySelectorAll("g.stage")[i];
          if (again) again.focus();
        });
        svg.appendChild(g);
        if (i < n - 1) {
          svg.appendChild(
            s("line", {
              x1: x + w,
              y1: 41,
              x2: x + gap + 4,
              y2: 41,
              stroke: c.ink2,
              "marker-end": "url(#" + id + "-arrow)",
            }),
          );
        }
      });

      var y = 92;
      if (str(data.legend)) {
        svg.appendChild(
          s("text", { x: 6, y: y, "font-size": 11, fill: c.ink2 }, data.legend),
        );
        y += 20;
      }
      if (str(data.alert)) {
        svg.appendChild(
          s("text", { x: 6, y: y, "font-size": 11, fill: c.bad }, data.alert),
        );
      }
      clear(holder);
      holder.appendChild(svg);
    }

    draw();
    showDetail();
    return draw;
  }

  /* docz-stepper */

  function drawStepper(mount, data) {
    var steps = data.steps;
    var scenarios = data.scenarios;
    if (!isList(steps, 2))
      throw new Error("docz-stepper: steps needs two or more");
    if (!isList(scenarios, 1))
      throw new Error("docz-stepper: scenarios needs one or more");
    steps.forEach(function (st) {
      if (!st || !str(st.key) || !str(st.name) || !str(st.does)) {
        throw new Error("docz-stepper: every step needs key, name, and does");
      }
    });
    scenarios.forEach(function (sc) {
      if (!sc || !str(sc.name))
        throw new Error("docz-stepper: every scenario needs a name");
    });
    var scen = 0;
    var step = 0;

    var choices = h("div", {
      class: "widget-choices",
      role: "group",
      "aria-label": "Scenario",
    });
    var canvas = h("div", { class: "widget-canvas" });
    var row = h("div", {
      class: "widget-steps",
      role: "group",
      "aria-label": "Step",
    });
    var body = h("div", { class: "widget-detail", "aria-live": "polite" });
    mount.appendChild(choices);
    mount.appendChild(canvas);
    mount.appendChild(row);
    mount.appendChild(body);

    scenarios.forEach(function (sc, i) {
      var b = button(sc.name, { "data-i": i });
      b.addEventListener("click", function () {
        scen = i;
        step = 0;
        render();
      });
      choices.appendChild(b);
    });

    function failIndex() {
      var key = scenarios[scen].fails_at;
      if (!str(key)) return -1;
      for (var i = 0; i < steps.length; i++) {
        if (steps[i].key === key) return i;
      }
      return -1;
    }

    function pipe() {
      var c = colours();
      var fail = failIndex();
      var n = steps.length;
      var W = 640;
      var gap = W / n;
      var svg = s("svg", {
        viewBox: "0 0 " + W + " 80",
        "aria-hidden": "true",
      });
      steps.forEach(function (st, i) {
        var cx = gap * i + gap / 2;
        var dead = fail > -1 && i > fail;
        var isFail = i === fail;
        var cur = i === step;
        if (i < n - 1) {
          svg.appendChild(
            s("line", {
              x1: cx + 16,
              y1: 30,
              x2: cx + gap - 16,
              y2: 30,
              stroke: dead || isFail ? c.line : c.ok,
              "stroke-width": 2,
              "stroke-dasharray": dead || isFail ? "4 4" : "0",
            }),
          );
        }
        var fill = isFail ? c.bad : cur ? c.accent : dead ? c.elevated : c.ok;
        svg.appendChild(
          s("circle", {
            cx: cx,
            cy: 30,
            r: 14,
            fill: fill,
            stroke: isFail ? c.bad : cur ? c.accent : c.line,
          }),
        );
        svg.appendChild(
          s(
            "text",
            {
              x: cx,
              y: 34,
              "font-size": 11,
              "text-anchor": "middle",
              class: "mono",
              fill: dead && !cur ? c.ink2 : c.onAccent,
            },
            isFail ? "✕" : String(i + 1),
          ),
        );
        svg.appendChild(
          s(
            "text",
            {
              x: cx,
              y: 64,
              "font-size": 10,
              "text-anchor": "middle",
              fill: c.ink2,
            },
            st.key,
          ),
        );
      });
      clear(canvas);
      canvas.appendChild(svg);
    }

    function render() {
      var sc = scenarios[scen];
      var fail = failIndex();
      var notes = sc.notes && typeof sc.notes === "object" ? sc.notes : {};
      $$("button", choices).forEach(function (b, i) {
        b.setAttribute("aria-pressed", i === scen ? "true" : "false");
      });
      clear(row);
      steps.forEach(function (st, i) {
        var b = button(st.name + (i === fail ? " ✕" : ""), {
          "aria-pressed": i === step ? "true" : "false",
          class: i === fail ? "fail" : i > fail && fail > -1 ? "dead" : null,
        });
        b.addEventListener("click", function () {
          step = i;
          render();
        });
        row.appendChild(b);
      });

      var st = steps[step];
      clear(body);
      body.appendChild(h("h4", null, str(st.title) ? st.title : st.name));
      body.appendChild(h("p", null, st.does));
      var note = notes[st.key];
      if (str(note)) {
        var failing = step === fail;
        var box = h("div", { class: failing ? "callout bad" : "callout ok" });
        box.appendChild(
          h(
            "p",
            { class: "callout-title" },
            failing
              ? str(sc.fail_label)
                ? sc.fail_label
                : "Denied"
              : "In this scenario",
          ),
        );
        box.appendChild(h("p", null, note));
        body.appendChild(box);
      }
      if (fail > -1 && step > fail) {
        var never = h("div", { class: "callout warning" });
        never.appendChild(
          h("p", null, "Never reached: stopped at step " + (fail + 1) + "."),
        );
        body.appendChild(never);
      }
      var nav = h("div", { class: "widget-nav" });
      var back = button("Back", { class: "btn" });
      var next = button("Next", { class: "btn" });
      back.disabled = step === 0;
      next.disabled = step === steps.length - 1;
      back.addEventListener("click", function () {
        step = Math.max(0, step - 1);
        render();
      });
      next.addEventListener("click", function () {
        step = Math.min(steps.length - 1, step + 1);
        render();
      });
      nav.appendChild(back);
      nav.appendChild(next);
      nav.appendChild(
        h("span", { class: "progress" }, step + 1 + " / " + steps.length),
      );
      body.appendChild(nav);
      pipe();
    }

    render();
    return pipe;
  }

  /* docz-scorecard */

  function score(checks, results) {
    var byId = {};
    (isList(results) ? results : []).forEach(function (r) {
      if (r && str(r.id)) byId[r.id] = r;
    });
    var points = 0;
    var weights = 0;
    var rows = checks.map(function (c) {
      var w = typeof c.weight === "number" && c.weight > 0 ? c.weight : 1;
      var r = byId[c.id];
      var measured =
        r && typeof r.score === "number" && r.score >= 0 && r.score <= 1;
      if (measured) {
        points += r.score * w;
        weights += w;
      }
      return { check: c, weight: w, result: r, measured: measured };
    });
    return { rows: rows, points: points, weights: weights };
  }

  function drawScorecard(mount, data, index) {
    var checks = data.checks;
    if (!isList(checks, 1))
      throw new Error("docz-scorecard: checks needs one or more");
    checks.forEach(function (c) {
      if (!c || !str(c.id) || !str(c.name) || !str(c.target)) {
        throw new Error(
          "docz-scorecard: every check needs id, name, and target",
        );
      }
    });
    var key = "docz:" + doc + ":scorecard:" + index;

    var input = h("textarea", {
      class: "widget-input",
      rows: 6,
      spellcheck: "false",
      "aria-label": "Results as JSON",
      placeholder: '{"results": [{"id": "' + checks[0].id + '", "score": 1}]}',
    });
    var bar = h("div", { class: "widget-nav" });
    var run = button("Score", { class: "btn" });
    var clearB = button("Clear", { class: "btn" });
    var out = h("div", { class: "widget-detail", "aria-live": "polite" });
    bar.appendChild(run);
    if (data.example && typeof data.example === "object") {
      var ex = button("Load the example", { class: "btn" });
      ex.addEventListener("click", function () {
        input.value = JSON.stringify(data.example, null, 2);
        render(data.example, "example, not real results");
      });
      bar.appendChild(ex);
    }
    bar.appendChild(clearB);
    mount.appendChild(input);
    mount.appendChild(bar);
    mount.appendChild(out);

    function fixes(text) {
      if (!str(text)) return null;
      var name = text.toLowerCase().replace(/\s+/g, "-");
      if (tabButton(name)) {
        return h("a", { href: "#p-" + name, "data-tab": name }, text);
      }
      return document.createTextNode(text);
    }

    function render(results, label) {
      clear(out);
      var sc = score(checks, results.results);
      var pct = sc.weights ? Math.round((sc.points / sc.weights) * 100) : 0;
      var measured = sc.rows.filter(function (r) {
        return r.measured;
      }).length;

      var total = h("div", { class: "card widget-score" });
      var head = h("div", { class: "phase-head" });
      head.appendChild(
        h("h3", null, sc.points.toFixed(1) + " / " + sc.weights.toFixed(1)),
      );
      head.appendChild(h("span", { class: "pill" }, label));
      if (str(results.subject))
        head.appendChild(h("span", { class: "pill" }, results.subject));
      if (str(results.generated_at)) {
        head.appendChild(h("span", { class: "pill" }, results.generated_at));
      }
      total.appendChild(head);
      var meter = h("div", { class: "bar" });
      var fill = h("i");
      fill.style.width = pct + "%";
      meter.appendChild(fill);
      total.appendChild(meter);
      var summary =
        pct +
        "% of the target, measured on " +
        measured +
        " of " +
        checks.length +
        " checks.";
      if (str(data.note)) summary += " " + data.note;
      total.appendChild(h("p", { class: "note" }, summary));
      out.appendChild(total);

      var wrap = h("div", { class: "table-wrap" });
      var table = h("table");
      var thead = h("thead");
      var tr = h("tr");
      ["#", "Check", "Observed", "Target", "Points", "Fixes it"].forEach(
        function (t) {
          tr.appendChild(h("th", null, t));
        },
      );
      thead.appendChild(tr);
      table.appendChild(thead);
      var tbody = h("tbody");
      sc.rows.forEach(function (row) {
        var c = row.check;
        var r = row.result || {};
        var tr2 = h("tr", { class: row.measured ? null : "unmeasured" });
        tr2.appendChild(h("td", null, c.id));
        var name = h("td");
        name.appendChild(h("b", null, c.name));
        if (str(c.source)) {
          name.appendChild(document.createTextNode(" "));
          name.appendChild(h("span", { class: "pill" }, c.source));
        }
        tr2.appendChild(name);
        var obs = h("td", null, str(r.observed) ? r.observed : "");
        if (str(r.evidence))
          obs.appendChild(h("span", { class: "note" }, r.evidence));
        tr2.appendChild(obs);
        tr2.appendChild(h("td", null, c.target));
        var cls = !row.measured
          ? null
          : r.score >= 1
            ? "low"
            : r.score > 0
              ? "medium"
              : "high";
        tr2.appendChild(
          h(
            "td",
            { class: cls },
            row.measured
              ? (r.score * row.weight).toFixed(1) + " of " + row.weight
              : "not measured",
          ),
        );
        var fx = h("td");
        var link = fixes(c.fixes);
        if (link) fx.appendChild(link);
        tr2.appendChild(fx);
        tbody.appendChild(tr2);
      });
      table.appendChild(tbody);
      wrap.appendChild(table);
      out.appendChild(wrap);
    }

    function fromInput(label) {
      var parsed;
      try {
        parsed = JSON.parse(input.value);
      } catch (e) {
        clear(out);
        var box = h("div", { class: "callout caution" });
        box.appendChild(h("p", null, "Not valid JSON: " + e.message));
        out.appendChild(box);
        return;
      }
      if (!parsed || !isList(parsed.results)) {
        clear(out);
        var miss = h("div", { class: "callout caution" });
        miss.appendChild(h("p", null, "The JSON needs a results list."));
        out.appendChild(miss);
        return;
      }
      save(key, input.value);
      render(parsed, label);
    }

    run.addEventListener("click", function () {
      fromInput("pasted");
    });
    clearB.addEventListener("click", function () {
      input.value = "";
      clear(out);
      try {
        window.localStorage.removeItem(key);
      } catch (e) {
        /* storage is optional */
      }
    });
    var saved = load(key);
    if (saved) {
      input.value = saved;
      fromInput("saved in this browser");
    }
    return null;
  }

  var DRAWERS = {
    flow: drawFlow,
    stepper: drawStepper,
    scorecard: drawScorecard,
  };

  function initWidgets() {
    var redraws = [];
    $$('script[type="application/json"][data-widget]').forEach(
      function (island, i) {
        var name = island.getAttribute("data-widget");
        var fig = island.parentElement;
        var mount = fig && $(".widget-mount", fig);
        var drawer = DRAWERS[name];
        if (!mount || !drawer) return;
        try {
          var data = JSON.parse(island.textContent);
          if (!data || typeof data !== "object")
            throw new Error(name + ": not an object");
          var redraw = drawer(mount, data, i);
          fig.classList.add("drawn");
          if (redraw) redraws.push(redraw);
        } catch (e) {
          clear(mount);
          if (window.console)
            console.warn("docz " + name + " widget:", e.message);
        }
      },
    );
    if (redraws.length) {
      modeListeners.push(function () {
        redraws.forEach(function (fn) {
          fn();
        });
      });
    }
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
    initWidgets();
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
