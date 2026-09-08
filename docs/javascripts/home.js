// Keep existing quick-start fragment links working inside the native tabs.
function revealQuickStartExample() {
  const anchor = document.getElementById(location.hash.slice(1));
  const panel = anchor?.closest(".gnmic-quickstart-examples .tabbed-block");
  if (!panel) return;

  const index = Array.from(panel.parentElement.children).indexOf(panel);
  const input = panel.closest(".tabbed-set").querySelectorAll(":scope > input")[index];
  input?.click();
  panel.scrollIntoView({ block: "center" });
}

revealQuickStartExample();
window.addEventListener("hashchange", revealQuickStartExample);
