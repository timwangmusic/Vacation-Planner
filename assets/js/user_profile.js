import { updateUsername } from "./user.js";

const username = updateUsername();
const initNumPlansShown = 3;
let numPlansShown = initNumPlansShown;

async function deleteUserPlan() {
  const username = this.dataset.user;
  const planId = this.dataset.planid;
  const url = `/v1/users/${username}/plan/${planId}`;

  await axios
    .delete(url)
    .then((response) => {
      console.log(response.status);
      location.reload();
    })
    .catch((err) => console.error(err));
}

function renderCard(plan, idx) {
  const cards = $("#cards");
  let card = $("<div>").addClass("card rounded mb-2").css("max-width", "350px");
  if (idx >= numPlansShown) {
    card.css("display", "none");
  }
  let cardBody = $("<div>").addClass("card-body");
  cardBody.append($("<h5>").addClass("card-title").text(plan.destination));
  cardBody.append($("<h6>").addClass("card-subtitle").text(plan.travel_date));
  let placeList = $("<ul>").addClass("list-group list-group-flush");
  plan.places.forEach((place) => {
    let p = $("<li>").addClass("list-group-item");
    p.append(
      $("<a>")
        .addClass("card-link")
        .attr("href", place.url)
        .text(place.place_name)
    );
    placeList.append(p);
  });
  cardBody.append(placeList);
  let deleteButton = $("<button>")
    .addClass("btn btn-outline-warning m-1 float-end")
    .attr("type", "button")
    .attr("data-planid", `${plan.id}`)
    .attr("data-user", `${username}`)
    .text("delete");
  deleteButton.click(deleteUserPlan);
  cardBody.append(deleteButton);

  let showDetailsButton = $("<button>")
    .addClass("btn btn-outline-info float-end m-1")
    .attr("type", "button")
    .attr("data-planid", `${plan.original_plan_id}`)
    .text("details");
  showDetailsButton.click(showPlanDetails);
  cardBody.append(showDetailsButton);

  card.append(cardBody);
  cards.append(card);
}

function renderFavorites(favorites) {
  document.getElementById("most-searched-place").style.maxWidth = "350px";
  const mostSearchedPlace = document.querySelector(
    "#most-searched-place .card-body .card-text"
  );
  let result = "";
  let count = 0;
  for (const location in favorites) {
    if (favorites[location].count > count) {
      result = favorites[location].location;
      count = favorites[location].count;
    }
  }
  mostSearchedPlace.innerText = result;
}

function showPlanDetails() {
  const planID = this.dataset.planid;
  window.location = `plans/${planID}`;
}

function getUserPlans() {
  const url = `/v1/users/${username}/plans`;
  return axios
    .get(url)
    .then((response) => {
      const data = response.data;
      const plans = data["travel_plans"];
      if (plans.length > 0) {
        for (let i = 0; i < plans.length; i++) {
          renderCard(plans[i], i);
        }
        if (plans.length > initNumPlansShown) {
          $("#load-more-plans-btn").css("display", "");
        }
      }
    })
    .catch((err) => console.error(err));
}

function getUserFavorites() {
  const favoritesUrl = `/v1/users/${username}/favorites`;
  return axios
    .get(favoritesUrl)
    .then((response) => {
      const data = response.data;
      const favorites = data["searchHistory"];
      if (Object.keys(favorites).length > 0) {
        renderFavorites(favorites);
      }
    })
    .catch((err) => console.error(err));
}

async function renderUserProfile() {
  await Promise.all([getUserPlans(), getUserFavorites()]);
}

$("#load-more-plans-btn").on("click", () => {
  numPlansShown = numPlansShown + 3;
  $("#cards")
    .children()
    .each((idx, card) => {
      if (idx >= numPlansShown) {
        card.style.display = "none";
      } else {
        card.style.display = "";
      }
    });
  // hide the load more button when all plans are shown
  if (numPlansShown >= $("#cards").children().length) {
    $("#load-more-plans-btn").css("display", "none");
  }
});

renderUserProfile().then(() => console.log("user profile is loaded"));
