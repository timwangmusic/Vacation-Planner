import { Place, View } from "./place.js";

import { updateUsername } from "./user.js";

const saveIcon = document.querySelector("i.bi-bookmark");
const saveTooltip = new bootstrap.Tooltip(saveIcon);
const profile = document.getElementById("profile");
const username = updateUsername();

// save bookmark tooltip
saveTooltip.disable();
saveIcon.addEventListener('click', postPlanForUser)

async function postPlanForUser() {
    const url = `/v1/users/${username}/plans`
    const plan = await getTravelPlan();

    try {
        console.log("Sending post request to", url)
        await fetch(
            url, {
            method: "POST",
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(planToView(plan))
        }
        )
    } catch (e) {
        console.log("Err when saving plan:", e)
        return
    }

    this.classList.remove("bi-bookmark")
    this.classList.add("bi-bookmark-check-fill")
    this.setAttribute("data-bs-original-title", "Saved!")
    let saveTooltip = new bootstrap.Tooltip(this)
    saveTooltip.enable();
    saveTooltip.show();
}

function planToView(plan) {
    const url = new URL(document.URL);
    const planId = getPlanId(url);
    const travelDate = getTravelDate(url);
    const view = new View(
        plan.TravelDestination,
        travelDate, // YYYY-MM-DD
        planId,
        new Date().toISOString(),
        []
    )
    for (const pDetail of plan.PlaceDetails) {
        const {
            Name: placeName,
            FormattedAddress: address,
            TimePeriod: timePeriod = '10 - 12', // TODO: use actual time period for each place
            URL: mapURL
        } = pDetail
        const place = new Place(timePeriod, placeName, address, mapURL)
        view.places.push(place)
    }
    return view
}

function getPlanId(url) {
    const results = url.pathname.split('/')
    return results[results.length - 1]
}

function getTravelDate(url) {
    const searchParams = new URLSearchParams(url.search);
    const date = searchParams.get("date");
    if (date) {
        return date
    }
    return new Date().toISOString().split('T')[0] // YYYY-MM-DD, current date
}

// Link to User Profile page
profile.addEventListener("click", () => window.location = `/v1/profile?username=`+username);
