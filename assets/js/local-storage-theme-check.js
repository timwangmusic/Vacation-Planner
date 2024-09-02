document.addEventListener("DOMContentLoaded", checkTheme);

function checkTheme() {
  // Check local Storage here for theme
  let localStore = localStorage.getItem("theme");
  if (localStore === "dark") {
    document.documentElement.setAttribute("data-theme", "dark");
  }
}
