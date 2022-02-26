// methods for the search page
import { locateMe, setDateToday } from "./utils.js";

import { logOut } from "./user.js";

setDateToday();

(function ($) {
    $("#location").autocomplete(
        {
            source: function (request, response) {
                $.ajax(
                    {
                        url: "/v1/cities",
                        dataType: "json",
                        data: { term: request.term },
                        success: function (data) {
                            response($.map(data.results, function (city) {
                                if (city.region) {
                                    return [city.city, city.region, city.country].join(", ")
                                }
                                return [city.city, city.country].join(", ")
                            }))
                        }
                    }
                )
            },
            minLength: 2,
        }
    )
})(jQuery);

document.getElementById("logout-confirm-btn").addEventListener(
    "click", logOut
)

document.querySelector('#autofill').addEventListener('click', locateMe);

const locationSearchInput = document.getElementById('location');
const spinner = document.getElementById("searchSpinner");
locationSearchInput.addEventListener(
    "keyup", (evt) => {
        if (evt.key === "Enter") {
            console.log("Pressed Enter in location input!")
            spinner.classList.remove("visually-hidden");
        }
    }
)
// hide spinner when switching pages
const hideSpinner = function () {
    spinner.classList.add("visually-hidden");
}
document.addEventListener("visibilitychange", hideSpinner);

document.getElementById("profile").addEventListener("click", () => window.location = `/v1/users/${username}/profile`);
