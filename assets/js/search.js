// methods for the search page
import { locateMe, locationAutocomplete, preciseLocation, setDateToday } from "./utils.js";
import { logOut, updateUsername } from "./user.js";

const username = updateUsername();
function randomPriceRange() {
    const item = document.getElementById('priceToSelect');
    const valueSel = item.options[item.selectedIndex].text;
    if (valueSel === "Surprise") {
        const index = Math.floor(Math.random() * 5);
        item.value = [0, 1, 2, 3, 4][index];
    }
}

setDateToday();

// auto-completes location input
locationAutocomplete(jQuery);

document.getElementById("logout-confirm-btn").addEventListener(
    "click", logOut
)

const FORM = document.getElementById("main-form");
const STORAGE_ITEM = "location";
const LOCATION_INPUT = document.getElementById("location");
$(document).ready(() => {
    const val = sessionStorage.getItem(STORAGE_ITEM);
    if (val) {
        console.log(`Set the Location based on PageLoad...` + val);
        LOCATION_INPUT.value = val;
    }
});

FORM.addEventListener('submit', () => {
    if (LOCATION_INPUT.value) {
        sessionStorage.setItem(STORAGE_ITEM, LOCATION_INPUT.value);
        console.log(`The location is ${LOCATION_INPUT.value}`);
    }
})

document.querySelector('#autofill').addEventListener('click', locateMe);

document.querySelector('#use-precise-location').addEventListener('click', preciseLocation);

document.querySelector('#priceToSelect').addEventListener('change', randomPriceRange)

const locationSearchInput = document.getElementById('location');
const spinner = document.getElementById("searchSpinner");
const searchBtn = document.getElementById("searchBtn");

// Show the wasearch spinner in two cases
// 1. Enter key is pressed in the location search box
// 2. Search button is clicked
locationSearchInput.addEventListener(
    "keyup", (evt) => {
        if (evt.key === "Enter") {
            console.log("Pressed Enter in location input!")
            spinner.classList.remove("visually-hidden");
        }
    }
);
searchBtn.addEventListener("click", () => {
    spinner.classList.remove("visually-hidden");
});

// hide spinner when switching pages
const hideSpinner = function () {
    spinner.classList.add("visually-hidden");
}
document.addEventListener("visibilitychange", hideSpinner);

document.getElementById("profile").addEventListener("click", () => window.location = `/v1/profile?username=` + username);
