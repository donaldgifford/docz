// Shared behaviour for INV-0036's sketches. Nothing here ships.

// Icons: Tabler outlines, the set the package uses.
const ICONS = {
  selector: '<path d="m8 9 4-4 4 4"/><path d="m16 15-4 4-4-4"/>',
  search: '<circle cx="10" cy="10" r="7"/><path d="m21 21-6-6"/>',
  panel: '<path d="M4 6a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2zM9 4v16"/>',
  chevron: '<path d="m9 6 6 6-6 6"/>',
  check: '<path d="m5 12 5 5L20 7"/>',
  file: '<path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M17 21H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h7l5 5v11a2 2 0 0 1-2 2z"/><path d="M9 9h1M9 13h6M9 17h6"/>',
  files: '<path d="M15 3v4a1 1 0 0 0 1 1h4"/><path d="M18 17h-7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4l5 5v7a2 2 0 0 1-2 2z"/><path d="M16 17v2a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V9a2 2 0 0 1 2-2h2"/>',
  repo: '<path d="M6 4h11a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H6a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1"/><path d="M9 4v16M13 8h2M13 12h2"/>',
  home: '<path d="M5 12H3l9-9 9 9h-2"/><path d="M5 12v7a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-7"/><path d="M9 21v-6a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v6"/>',
  history: '<path d="M12 8v4l2 2"/><path d="M3.05 11a9 9 0 1 1 .5 4M3 20v-5h5"/>',
  folder: '<path d="M5 4h4l3 3h7a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2"/>',
  alert: '<path d="M12 9v4M12 16v.01"/><path d="M10.4 3.6 2.3 17.1A1.9 1.9 0 0 0 3.9 20h16.2a1.9 1.9 0 0 0 1.6-2.9L13.600 3.600a1.900 1.900 0 0 0-3.200 0"/>',
  inbox: '<path d="M4 6a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2z"/><path d="M4 13h3l3 3h4l3-3h3"/>',
  refresh: '<path d="M20 11A8.1 8.1 0 0 0 4.5 9M4 5v4h4"/><path d="M4 13a8.1 8.1 0 0 0 15.5 2m.5 4v-4h-4"/>',
  external: '<path d="M12 6H6a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-6"/><path d="m11 13 9-9"/><path d="M15 4h5v5"/>',
  logout: '<path d="M14 8V6a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h7a2 2 0 0 0 2-2v-2"/><path d="M9 12h12l-3-3M18 15l3-3"/>',
  moon: '<path d="M12 3h.4a7.5 7.5 0 0 0 7.900 9.400A9 9 0 1 1 12 3"/>',
  sun: '<circle cx="12" cy="12" r="4"/><path d="M3 12h1M12 3v1M20 12h1M12 20v1M5.600 5.600l.7.7M18.400 5.600l-.7.7M17.700 17.700l.7.7M6.300 17.700l-.7.7"/>',
  shield: '<path d="M12 3a12 12 0 0 0 8.5 3A12 12 0 0 1 12 21 12 12 0 0 1 3.500 6 12 12 0 0 0 12 3"/><path d="m9 12 2 2 4-4"/>',
  pin: '<path d="m15 4.500-4 4L7 10l-1.500 1.500 7 7L14 17l1.500-4 4-4"/><path d="m9 15-4.500 4.500M14.500 4 20 9.500"/>',
};

const sprite = document.createElementNS("http://www.w3.org/2000/svg", "svg");
sprite.setAttribute("width", "0");
sprite.setAttribute("height", "0");
sprite.setAttribute("aria-hidden", "true");
sprite.style.position = "absolute";
sprite.innerHTML = `<defs>${Object.entries(ICONS)
  .map(([name, paths]) => `<g id="i-${name}">${paths}</g>`)
  .join("")}</defs>`;
document.body.prepend(sprite);

// <i data-icon="search"></i> becomes the icon, so the markup stays short.
for (const holder of document.querySelectorAll("i[data-icon]")) {
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("aria-hidden", "true");
  const use = document.createElementNS("http://www.w3.org/2000/svg", "use");
  use.setAttribute("href", `#i-${holder.dataset.icon}`);
  svg.append(use);
  if (holder.className) svg.setAttribute("class", holder.className);
  holder.replaceWith(svg);
}

// Theme: the package switches by the .dark class on <html>. Every toggle on
// the page (the sketch's own, and the ones drawn inside a frame) follows.
function syncThemeButtons() {
  const dark = document.documentElement.classList.contains("dark");
  for (const button of document.querySelectorAll("[data-theme-toggle]")) {
    if (button.dataset.themeToggle === "label") {
      button.textContent = dark ? "Light theme" : "Dark theme";
    } else {
      button.setAttribute("aria-label", dark ? "Switch to light mode" : "Switch to dark mode");
      button.querySelector("use")?.setAttribute("href", dark ? "#i-sun" : "#i-moon");
    }
  }
}
document.addEventListener("click", (event) => {
  if (!event.target.closest("[data-theme-toggle]")) return;
  document.documentElement.classList.toggle("dark");
  syncThemeButtons();
});
syncThemeButtons();

// Segmented controls and chip groups: one pressed button per group.
for (const group of document.querySelectorAll("[data-seg]")) {
  group.addEventListener("click", (event) => {
    const button = event.target.closest("button");
    if (!button) return;
    for (const other of group.querySelectorAll("button[aria-pressed]")) {
      other.setAttribute("aria-pressed", String(other === button));
    }
  });
}

// Tree carets and other expanders: toggle the element named by aria-controls.
document.addEventListener("click", (event) => {
  const caret = event.target.closest("button[aria-controls]");
  if (!caret || caret.disabled) return;
  const target = document.getElementById(caret.getAttribute("aria-controls"));
  if (!target) return;
  const open = caret.getAttribute("aria-expanded") !== "true";
  caret.setAttribute("aria-expanded", String(open));
  target.hidden = !open;
});

// Outlines: clicking a row marks it, standing in for the scroll spy.
for (const outline of document.querySelectorAll(".outline")) {
  outline.addEventListener("click", (event) => {
    const link = event.target.closest("a");
    if (!link) return;
    event.preventDefault();
    for (const other of outline.querySelectorAll("a")) other.removeAttribute("aria-current");
    link.setAttribute("aria-current", "location");
  });
}
