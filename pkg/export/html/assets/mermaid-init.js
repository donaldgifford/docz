/*
 * Draws every pre.mermaid with the mermaid global, in the theme's tokens,
 * and draws again when the mode changes (DESIGN-0025 §7). The theme
 * variables follow ui/'s mermaidThemeFromTokens. A diagram whose source
 * fails to parse keeps its source, with the error beneath it, rather than
 * leaving a blank box. securityLevel strict keeps a diagram's labels from
 * carrying script or links.
 */
(function () {
  "use strict";
  if (!window.mermaid || !window.docz) return;

  var blocks = Array.prototype.slice.call(document.querySelectorAll("pre.mermaid"));
  blocks.forEach(function (pre) {
    pre.setAttribute("data-source", pre.textContent);
  });

  function themeVariables() {
    var p = window.docz.palette([
      "--color-bg-raised",
      "--color-bg-elevated",
      "--color-border-strong",
      "--color-fg-primary",
      "--color-fg-tertiary",
    ]);
    var raised = p["--color-bg-raised"];
    var elevated = p["--color-bg-elevated"];
    var border = p["--color-border-strong"];
    var fg = p["--color-fg-primary"];
    return {
      primaryColor: elevated,
      primaryBorderColor: border,
      primaryTextColor: fg,
      secondaryColor: raised,
      secondaryBorderColor: border,
      tertiaryColor: raised,
      tertiaryBorderColor: border,
      mainBkg: elevated,
      nodeBorder: border,
      lineColor: p["--color-fg-tertiary"],
      clusterBkg: raised,
      clusterBorder: border,
      titleColor: fg,
      edgeLabelBackground: raised,
      fontFamily: "monospace",
      fontSize: "14px",
    };
  }

  var run = 0;

  function draw() {
    run++;
    window.mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      theme: "base",
      themeVariables: themeVariables(),
    });
    blocks.forEach(function (pre, i) {
      var source = pre.getAttribute("data-source");
      var next = pre.nextElementSibling;
      if (next && next.classList.contains("diagram-error")) next.remove();
      window.mermaid
        .render("docz-mermaid-" + run + "-" + i, source)
        .then(function (out) {
          pre.innerHTML = out.svg;
          pre.setAttribute("data-processed", "true");
        })
        .catch(function (err) {
          pre.textContent = source;
          pre.removeAttribute("data-processed");
          var msg = document.createElement("p");
          msg.className = "diagram-error";
          msg.textContent = "Diagram failed to render: " + (err && err.message ? err.message : String(err));
          pre.insertAdjacentElement("afterend", msg);
        });
    });
  }

  draw();
  window.docz.onMode(draw);
})();
