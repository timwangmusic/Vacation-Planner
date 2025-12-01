import { Place, View } from "./place.js";

import { updateUsername } from "./user.js";

const saveBtn = document.getElementById("savePlanBtn");
const profile = document.getElementById("profile");
const username = updateUsername();

// save plan button click handler
if (saveBtn) {
  saveBtn.addEventListener("click", postPlanForUser);
}

async function postPlanForUser() {
  const url = `/v1/users/${username}/plans`;
  const plan = await getTravelPlan();

  try {
    console.log("Sending post request to", url);
    await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(planToView(plan)),
    });
  } catch (e) {
    console.log("Err when saving plan:", e);
    return;
  }

  // Update button to show saved state
  const icon = this.querySelector("i");
  icon.classList.remove("bi-bookmark");
  icon.classList.add("bi-bookmark-check-fill");
  this.innerHTML = '<i class="bi bi-bookmark-check-fill me-1"></i>Saved!';
  this.classList.remove("btn-outline-primary");
  this.classList.add("btn-success");
  this.disabled = true;
}

function planToView(plan) {
  const url = new URL(document.URL);
  const travelDate = getTravelDate(url);
  const view = new View(
    plan.TravelDestination,
    travelDate, // YYYY-MM-DD
    plan.OriginalPlanID,
    new Date().toISOString(),
    []
  );
  for (const pDetail of plan.PlaceDetails) {
    const {
      ID: id,
      Name: placeName,
      FormattedAddress: address,
      TimePeriod: timePeriod = "10 - 16", // TODO: use actual time period for each place
      URL: mapURL,
    } = pDetail;
    const place = new Place(id, timePeriod, placeName, address, mapURL);
    view.places.push(place);
  }
  return view;
}

function getTravelDate(url) {
  const searchParams = new URLSearchParams(url.search);
  const date = searchParams.get("date");
  if (date) {
    return date;
  }
  return new Date().toISOString().split("T")[0]; // YYYY-MM-DD, current date
}

// Link to User Profile page
profile.addEventListener("click", () => (window.location = "/v1/profile"));
