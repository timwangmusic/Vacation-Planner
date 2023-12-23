// methods for the search page
import {
  locateMe,
  locationAutocomplete,
  preciseLocation,
  setDateToday,
} from "./utils.js";
import { logOut, updateUsername } from "./user.js";

const username = updateUsername();
function randomPriceRange() {
  const item = document.getElementById("priceToSelect");
  const valueSel = item.options[item.selectedIndex].text;
  if (valueSel === "Surprise") {
    const index = Math.floor(Math.random() * 5);
    item.value = [0, 1, 2, 3, 4][index];
  }
}

setDateToday();

// auto-completes location input
locationAutocomplete(jQuery);

document.getElementById("logout-confirm-btn").addEventListener("click", logOut);

const FORM = document.getElementById("main-form");
const STORAGE_ITEM = "location";
const LOCATION_INPUT = document.getElementById("location");

// resets flag value whenever user manually modifies input
LOCATION_INPUT.addEventListener("change", () => {
  document.querySelector("#precise-location-flag").value = "false";
});

$(document).ready(() => {
  const val = sessionStorage.getItem(STORAGE_ITEM);
  if (val) {
    console.log(`Set the Location based on PageLoad...` + val);
    LOCATION_INPUT.value = val;
  }

  const usePreciseLocation = sessionStorage.getItem("use-precise-location");
  if (usePreciseLocation) {
    document.querySelector("#precise-location-flag").value = usePreciseLocation;
  }
  let is_cracked_house_clicked = false;
  function crackedHouseClickedState() {
    console.log(`we click!!`)
    is_cracked_house_clicked = true;
    return is_cracked_house_clicked
  }

  document.querySelector("#crack-house").addEventListener("click", crackedHouseClickedState);
  document.addEventListener('DOMContentLoaded', function (event) {
    // get the footer element
    let footer = document.querySelector('footer');

    // Attach a click Event Listener to the footer
    footer.addEventListener('click', function (event) {
      console.log(`Did we click!!`)
    })
    //is_cracked_house_clicked = true;
  })
  // HOME_FOOTER.addEventListener("change", function () {
  //   var footer = document.querySelector('footer');
  //   footer.addEventListener('click', function (event) {
  //     if (event.target.classList.contains('fa-solid fa-house-chimney-crack fa-xl')) {
  //       console.log('The link inside the footer has been clicked..')
  //     }
  //   })
  // })

  // FIXME: not a clean solution, improve this after we use front-end rendering
  const url = new URL(document.referrer);
  if (url.pathname === "/v1/" && pageIsNavigated() && !is_cracked_house_clicked) {
    console.debug("no planning solution is found");
    $("#no-plan-error-alert").removeClass("d-none");
  }
});

FORM.addEventListener("submit", () => {
  if (LOCATION_INPUT.value) {
    sessionStorage.setItem(STORAGE_ITEM, LOCATION_INPUT.value);
    console.log(`The location is ${LOCATION_INPUT.value}`);

    // updates stored precise location flag value upon search form submission
    sessionStorage.setItem(
      "use-precise-location",
      document.querySelector("#precise-location-flag").value
    );
  }
});

function pageIsNavigated() {
  const entries = performance.getEntriesByType("navigation");
  let result = false;
  entries.forEach((entry) => {
    if (entry.type === "navigate") {
      console.log("page is navigated");
      result = true;
    }
    console.log(`page is ${entry.type}`);
  });
  return result;
}

document.querySelector("#autofill").addEventListener("click", locateMe);

document
  .querySelector("#use-precise-location")
  .addEventListener("click", preciseLocation);

document
  .querySelector("#priceToSelect")
  .addEventListener("change", randomPriceRange);

const locationSearchInput = document.getElementById("location");
const spinner = document.getElementById("searchSpinner");
const searchBtn = document.getElementById("searchBtn");

// Renders the search spinner in two cases
// 1. Enter key is pressed in the location search box
// 2. Search button is clicked
locationSearchInput.addEventListener("keyup", (evt) => {
  if (evt.key === "Enter") {
    console.log("Pressed Enter in location input!");
    spinner.classList.remove("visually-hidden");
  }
});
searchBtn.addEventListener("click", () => {
  spinner.classList.remove("visually-hidden");
});

// hide spinner when switching pages
const hideSpinner = function () {
  spinner.classList.add("visually-hidden");
};
document.addEventListener("visibilitychange", hideSpinner);

document
  .getElementById("profile")
  .addEventListener(
    "click",
    () => (window.location = `/v1/profile?username=` + username)
  );

const nearbyCitiesFlag = document.getElementById("use-nearby-cities-flag");
document
  .getElementById("searchNearbyCities")
  .addEventListener("change", (event) => {
    nearbyCitiesFlag.value = event.currentTarget.checked;
  });
