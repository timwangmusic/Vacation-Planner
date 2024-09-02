// Switch mode function
const switchTheme = () => {
  // Get the root element and the data-theme value
  const rootElem = document.documentElement;
  let dataTheme = rootElem.getAttribute("data-theme"),
    newTheme;
  newTheme = dataTheme === "light" ? "dark" : "light";

  // Set the new HTML attribute
  rootElem.setAttribute("data-theme", newTheme);

  // Set the new Local Storage item
  localStorage.setItem("theme", newTheme);
};

// Add event Listener for the theme switcher
document
  .querySelector("#theme-switcher")
  .addEventListener("click", switchTheme);
