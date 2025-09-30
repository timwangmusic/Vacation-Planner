import { Place, View } from "./place.js";

import { capitalizeFirstChar } from "./utils.js";
import { updateUsername } from "./user.js";

const plansPerPage = 5; // Number of plans to show on each "load more" click
const username = updateUsername();
let plansData = null;
let numberOfPlans = 5;
let displayedPlans = 5;

async function getPlans(numResults = null) {
  let plansUrl = document.URL + "&json_only=true";

  // If numResults is specified, override the URL parameter
  if (numResults !== null) {
    const url = new URL(plansUrl);
    url.searchParams.set("numberResults", numResults);
    plansUrl = url.toString();
  }

  return await fetch(plansUrl, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
  })
    .then((response) => response.json())
    .catch((err) => console.log(err));
}

async function postUserFeedback(planIdx) {
  const url = `/v1/users/${username}/feedback`;

  if (!plansData) {
    plansData = await getPlans();
  }
  const plan = plansData.travel_plans[planIdx];

  await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(planToFeedback(plan)),
  })
    .then(() => $(`#refresh-${planIdx}`).css("display", "inline-block"))
    .catch((err) => console.error(err));
}

async function getImageForLocation() {
  console.log("calling image generation function...");
  const location = getLocation();
  const url = `/v1/gen_image`;
  const parts = location.split(",").map((str) => str.trim());
  if (parts.length < 2 || parts.length > 3) {
    console.log("wrong location input format", location);
    $("#loadingSpinner").hide();
    return;
  }

  let city = parts[0];
  let country = parts[parts.length - 1];
  let adminAreaLevelOne = "cities";
  if (parts.length == 3) {
    adminAreaLevelOne = parts[1];
  }

  await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      city: city,
      adminAreaLevelOne: adminAreaLevelOne,
      country: country,
    }),
  })
    .then((response) => response.json())
    .then((data) => {
      $("#generated-img").attr("src", data.photo).show();
      $("#gen-img-download-btn")
        .off("click")
        .click(() => {
          console.log("downloading location image...");
          const url = data.photo;
          const a = document.createElement("a");
          a.href = url;
          a.download = "downloaded-city-image.png";
          document.body.appendChild(a);
          a.click();
          document.body.removeChild(a);
        })
        .show();
      $("#loadingSpinner").hide();
    })
    .catch(console.error);
}

function planToFeedback(plan) {
  return {
    plan_id: "travel_plan:" + plan.id,
    plan_spec: plan.planning_spec,
  };
}

async function postPlanForUser() {
  if (!plansData) {
    plansData = await getPlans();
  }

  const url = `/v1/users/${username}/plans`;
  const fields = this.id.split("-");
  const planIndex = fields[fields.length - 1];

  // the number of plans equals the array length in the JSON result
  numberOfPlans = plansData.travel_plans.length;
  const sourcePlan = plansData.travel_plans[planIndex];
  const destination = plansData.travel_destination;

  await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(planToView(sourcePlan, planIndex, destination)),
  }).catch((err) => console.error(err));

  $(this).attr("disabled", "true");
  $(this).parent().attr("title", "saved!");
}

function planToView(plan, planIndex, destination) {
  const url = new URL(document.URL);
  const location = normalizeLocation(destination);
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

async function getPlanSummaryResponse(planIdx) {
  const url = "/v1/plan-summary";
  if (!plansData) {
    plansData = await getPlans();
  }

  return await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      plan_id: plansData.travel_plans[planIdx].id,
    }),
  })
    .then((resp) => {
      if (!resp.ok) {
        switch (resp.status) {
          case 400:
            return {
              message: "Bad request",
            };
          case 500:
            return {
              message: "Unable to generate a response, please try again later.",
            };
          default:
            return {
              message: "",
            };
        }
      }
      return resp.json();
    })
    .catch((err) => console.log(err));
}

// create button event actions
for (let planIndex = 0; planIndex < numberOfPlans; planIndex++) {
  $(`#save-${planIndex}`).click(postPlanForUser);
  $(`#gen-summary-${planIndex}`).click(async () => {
    console.log("generating plan summary...");
    $(`#gen-summary-${planIndex}`).prop("disabled", true);
    const resp = await getPlanSummaryResponse(planIndex);
    $(`#modal-body-${planIndex}`).html(summaryToHTML(resp.message));
    $(`#gen-summary-${planIndex}`)
      .prop("disabled", false)
      .text("regenerate summary");
  });
}

function summaryToHTML(message) {
  let result = ["<ul>"];
  for (const line of message.trim().split(". ")) {
    result.push("<li>" + line.trim() + "</li>");
  }
  result.push("</ul>");
  return result.join("");
}

$(".reload-btn").each(function (_, element) {
  $(element).click(() => location.reload());
});

function handleUserLike() {
  const fields = this.id.split("-");
  const planIdx = fields[fields.length - 1];
  $(`#dislike-${planIdx}`).attr("disabled", "true");
}

async function handleUserDislike() {
  const fields = this.id.split("-");
  const planIdx = fields[fields.length - 1];
  $(`#like-${planIdx}`).attr("disabled", "true");

  await postUserFeedback(planIdx);
}

for (let planIdx = 0; planIdx < numberOfPlans; planIdx++) {
  $(`#like-${planIdx}`).click(handleUserLike);
  $(`#dislike-${planIdx}`).click(handleUserDislike);
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

// Function to attach event listeners to a specific plan
function attachPlanEventListeners(planIndex) {
  $(`#save-${planIndex}`).off("click").click(postPlanForUser);
  $(`#gen-summary-${planIndex}`).off("click").click(async () => {
    console.log("generating plan summary...");
    $(`#gen-summary-${planIndex}`).prop("disabled", true);
    const resp = await getPlanSummaryResponse(planIndex);
    $(`#modal-body-${planIndex}`).html(summaryToHTML(resp.message));
    $(`#gen-summary-${planIndex}`)
      .prop("disabled", false)
      .text("regenerate summary");
  });

  $(`#like-${planIndex}`).off("click").click(handleUserLike);
  $(`#dislike-${planIndex}`).off("click").click(handleUserDislike);
  $(`#refresh-${planIndex}`).off("click").click(() => location.reload());
}

// Function to render a single plan as HTML
function renderPlanHTML(plan, planIndex, detailsURL) {
  const placeRows = plan.places.map(place => `
    <tr>
      <td class="col-3" style="color: darkcyan">
        <div class="d-flex flex-row">
          <span class="material-icons">${place.place_icon_css_class}</span>
          <span class="mx-2" id="interval-${planIndex}">${place.start_time} - ${place.end_time}</span>
        </div>
      </td>
      <td class="col-4 col-md-3">
        <a href="${place.url}">${place.place_name}</a>
      </td>
      <td class="d-none d-md-block" style="color: #0d6efd">
        ${place.address}
      </td>
    </tr>
  `).join('');

  return `
    <div class="accordion-item" id="plan-accordion-${planIndex}">
      <h2 class="accordion-header border">
        <button
          class="accordion-button"
          type="button"
          data-bs-toggle="collapse"
          data-bs-target="#plan-${planIndex}"
          aria-expanded="true"
          aria-controls="plan-${planIndex}"
          style="color: #24c1e0"
        >
          One-Day Travel Plan
        </button>
      </h2>
      <div
        id="plan-${planIndex}"
        class="accordion-collapse collapse show"
        data-bs-parent="#accordionSetParent"
      >
        <div class="accordion-body">
          <div class="btn-group" role="group">
            <button id="like-${planIndex}" class="btn btn-sm btn-outline-primary">
              <i class="fa fa-thumbs-o-up"></i>
            </button>

            <button
              id="dislike-${planIndex}"
              class="btn btn-sm btn-outline-primary"
            >
              <i class="fa fa-thumbs-o-down"></i>
            </button>

            <button
              id="refresh-${planIndex}"
              class="reload-btn btn btn-sm btn-outline-success"
            >
              <i class="fa fa-refresh fa-spin"></i>
            </button>
          </div>

          <span
            class="d-inline-block float-end"
            tabindex="0"
            data-bs-toggle="tooltip"
            data-bs-placement="left"
            title="Save to profile"
          >
            <button
              id="save-${planIndex}"
              type="button"
              class="btn btn-sm btn-outline-primary m-1"
              ${plan.saved ? 'disabled' : ''}
            >
              save
            </button>
          </span>
          <a
            class="btn btn-sm btn-outline-primary m-1 float-end"
            href="${detailsURL}"
            >show</a
          >

          <span class="d-inline-block float-end">
            <button
              type="button"
              class="btn btn-sm btn-outline-primary m-1"
              data-bs-toggle="modal"
              data-bs-target="#modal-${planIndex}"
            >
              summary
            </button>
          </span>

          <!-- Modal -->
          <div
            class="modal fade"
            id="modal-${planIndex}"
            tabindex="-1"
            aria-hidden="true"
          >
            <div class="modal-dialog">
              <div class="modal-content">
                <div class="modal-header">
                  <h1
                    class="modal-title fs-5"
                    style="background-color: white"
                  >
                    Travel Plan Summary
                  </h1>
                  <button
                    type="button"
                    class="btn-close"
                    data-bs-dismiss="modal"
                    aria-label="Close"
                  ></button>
                </div>
                <div class="modal-body" id="modal-body-${planIndex}">...</div>
                <div class="modal-footer">
                  <button
                    type="button"
                    class="btn btn-primary"
                    id="gen-summary-${planIndex}"
                  >
                    Generate Summary
                  </button>
                  <button
                    type="button"
                    class="btn btn-secondary"
                    data-bs-dismiss="modal"
                  >
                    Close
                  </button>
                </div>
              </div>
            </div>
          </div>

          <table
            id="plan-table-${planIndex}"
            class="table table-bordered table-striped table-hover"
            style="table-layout: fixed"
          >
            <thead>
              <tr>
                <th class="col-3">Time</th>
                <th class="col-4 col-md-3">Place Name</th>
                <th class="d-none d-md-block">Address</th>
              </tr>
            </thead>
            <tbody>
              ${placeRows}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  `;
}

// Function to handle loading more plans
async function loadMorePlans() {
  const loadMoreBtn = $("#load-more-btn");
  const loadMoreContainer = loadMoreBtn.parent();
  loadMoreBtn.prop("disabled", true).text("Loading...");

  // Check if we have more plans in the already fetched data
  if (displayedPlans < plansData.travel_plans.length) {
    // Show plans from existing data
    const endIndex = Math.min(displayedPlans + plansPerPage, plansData.travel_plans.length);

    for (let i = displayedPlans; i < endIndex; i++) {
      const plan = plansData.travel_plans[i];
      const detailsURL = plansData.trip_details_url[i];
      const planHTML = renderPlanHTML(plan, i, detailsURL);
      // Insert before the load more button container
      loadMoreContainer.before(planHTML);
      attachPlanEventListeners(i);
    }

    displayedPlans = endIndex;
  } else {
    // Need to fetch more plans from the API
    const url = new URL(window.location.href);
    const currentResults = parseInt(url.searchParams.get("numberResults") || "5");
    const newNumberResults = currentResults + plansPerPage;

    url.searchParams.set("numberResults", newNumberResults);
    url.searchParams.set("json_only", "true");

    try {
      const response = await fetch(url.toString());
      const newData = await response.json();

      if (newData.travel_plans && newData.travel_plans.length > plansData.travel_plans.length) {
        // Render only the new plans
        for (let i = plansData.travel_plans.length; i < newData.travel_plans.length; i++) {
          const plan = newData.travel_plans[i];
          const detailsURL = newData.trip_details_url[i];
          const planHTML = renderPlanHTML(plan, i, detailsURL);
          // Insert before the load more button container
          loadMoreContainer.before(planHTML);
          attachPlanEventListeners(i);
        }

        plansData = newData;
        displayedPlans = newData.travel_plans.length;
      }
    } catch (err) {
      console.error("Failed to fetch more plans:", err);
    }
  }

  // Update button state
  if (displayedPlans >= plansData.travel_plans.length) {
    // Hide button when all plans are shown
    loadMoreBtn.hide();
  } else {
    // Re-enable button for next click
    loadMoreBtn.text("Load More...").prop("disabled", false);
  }
}

$(document).ready(async function () {
  {
    // Count how many plans are actually in the DOM (server-side rendered)
    displayedPlans = $(".accordion-item").length;

    // Get the current numberResults from URL, or use the displayedPlans count
    const url = new URL(window.location.href);
    const urlNumberResults = parseInt(url.searchParams.get("numberResults") || displayedPlans.toString());

    // Fetch plans with a higher number to pre-load more results
    // Use max of (URL param + 10) or 15 to ensure we have extra plans
    const fetchCount = Math.max(urlNumberResults + 10, 15);
    plansData = await getPlans(fetchCount);
    numberOfPlans = plansData.travel_plans.length;

    for (let idx = 0; idx < plansData.travel_plans.length; idx++) {
      let btn = document.getElementById("save-" + idx);
      if (btn != null && plansData.travel_plans[idx].saved) {
        btn.disabled = true;
      }
    }

    // Attach event listeners to initially displayed plans
    for (let idx = 0; idx < displayedPlans; idx++) {
      attachPlanEventListeners(idx);
    }

    // Setup load more button
    $("#load-more-btn").click(loadMorePlans);

    // Hide load more button if all plans are already displayed
    if (displayedPlans >= numberOfPlans) {
      $("#load-more-btn").hide();
    }

    await getImageForLocation();
  }
});

function getLocation() {
  const url = new URL(document.URL);
  const searchParams = new URLSearchParams(url.search);
  return searchParams.get("location");
}
