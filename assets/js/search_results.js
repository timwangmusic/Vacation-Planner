import { Place, View } from "./place.js";

import { capitalizeFirstChar } from "./utils.js";
import { updateUsername } from "./user.js";

let numberOfPlans = 5;
const username = updateUsername();

async function getPlans() {
  const plansUrl = document.URL + "&json_only=true";

  return await fetch(plansUrl, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
  })
    .then((response) => response.json())
    .catch((err) => console.log(err));
}

async function postPlanForUser() {
  const url = `/v1/users/${username}/plans`;
  const fields = this.id.split("-");
  const planIndex = fields[fields.length - 1];

  const data = await getPlans();
  // the number of plans equals the array length in the JSON result
  numberOfPlans = data.length;
  const sourcePlan = data[planIndex];

  await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(planToView(sourcePlan, planIndex)),
  }).catch((err) => console.error(err));

  $(this).attr("disabled", "true");
  $(this).parent().attr("title", "saved!");

  $(`#edit-${planIndex}`).attr("disabled", "true");
  $(`#plan-table-${planIndex} tBody`).attr("contenteditable", "false");
}

function planToView(plan, planIndex) {
  const url = new URL(document.URL);
  const location = normalizeLocation(url.searchParams.get("location"));
  console.log("The format fixed location is: ", location);
  const view = new View(
    location,
    url.searchParams.get("date"),
    plan.id,
    new Date().toISOString(),
    []
  );

  $(`#plan-table-${planIndex} tbody tr`).map((idx, row) => {
    const $row = $(row);

    view.places.push(
      new Place(
        plan.places[idx].id,
        $row.find(`:nth-child(1) #interval-${planIndex}`).text().trim(),
        $row.find(":nth-child(2)").find("a").text(),
        $row.find(":nth-child(3)").text(),
        $row.find(":nth-child(2)").find("a").attr("href")
      )
    );
  });

  return view;
}

// The autocompleted locations need to be fixed. City and country names need to be capitalized and admin level 2 names need to be changed to all upper cases.
function normalizeLocation(location) {
  const results = location.split(", ").map((s) =>
    s
      .split(" ")
      .map((word) => capitalizeFirstChar(word))
      .join(" ")
  );

  // city, admin level 2, country format
  if (results.length === 3) {
    return [results[0], results[1].toUpperCase(), results[2]].join(", ");
  }
  // city, country format
  return [results[0], results[1]].join(", ");
}

// create button event actions
for (let planIndex = 0; planIndex < numberOfPlans; planIndex++) {
  $(`#save-${planIndex}`).click(postPlanForUser);
  $(`#edit-${planIndex}`).click(() => {
    $(`#plan-table-${planIndex} tBody`).attr("contenteditable", "true");
    $(this).innerText = "done";
  });
}

document
  .getElementById("profile")
  .addEventListener("click", () => (window.location = "/v1/profile"));

const rollUpButton = document.getElementById("scroll-to-top");
rollUpButton.addEventListener("click", () => {
  window.scrollTo({
    top: 0,
    behavior: "smooth",
  });
});

window.onload = async function () {
  const data = await getPlans();
  numberOfPlans = data.length;
  for (let planIndex = 0; planIndex < numberOfPlans; planIndex++) {
    var buttonId = "save-" + planIndex;
    let buttonHandle = document.getElementById(buttonId);
    if (buttonHandle != null && data[planIndex].saved) {
      buttonHandle.disabled = true;
    }
  }
};
