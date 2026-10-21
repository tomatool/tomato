// The "split a flow where it crosses a boundary" picture in Getting Started:
// switch between one long scenario and a scenario per behaviour. It plays
// once on its own when it scrolls into view, unless the reader prefers less
// motion. Material's instant navigation swaps pages without a reload, so it
// sets itself up on every page through document$.
(function () {
  function setup(root) {
    if (root.dataset.ready) return;
    root.dataset.ready = "1";
    var timer = null, touched = false;
    function show(mode) {
      root.dataset.mode = mode;
      root.querySelectorAll("[data-mode]").forEach(function (b) {
        if (b !== root) b.setAttribute("aria-selected", b.dataset.mode === mode);
      });
      root.querySelectorAll("[data-for]").forEach(function (p) { p.hidden = p.dataset.for !== mode; });
    }
    root.querySelectorAll("button[data-mode]").forEach(function (b) {
      b.addEventListener("click", function () { touched = true; clearTimeout(timer); show(b.dataset.mode); });
    });
    show("long");
    var calm = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (!calm && "IntersectionObserver" in window) {
      var seen = new IntersectionObserver(function (entries) {
        entries.forEach(function (e) {
          if (!e.isIntersecting || touched) return;
          seen.disconnect();
          timer = setTimeout(function () { if (!touched) show("split"); }, 3500);
        });
      }, { threshold: 0.6 });
      seen.observe(root);
    }
  }
  function init() { document.querySelectorAll(".split-flow").forEach(setup); }
  if (window.document$) window.document$.subscribe(init); else document.addEventListener("DOMContentLoaded", init);
})();
