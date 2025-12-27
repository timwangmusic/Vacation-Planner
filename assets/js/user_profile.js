import { updateUsername } from "./user.js";

const username = updateUsername();
let profileSwiper = null;

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

function createFavoritesSlide(favorites) {
  const wrapper = $("#profile-swiper-wrapper");

  const slide = $("<div>").addClass("swiper-slide");
  const slideInner = $("<div>").addClass("swiper-slide-inner profile-favorites-slide");

  const cardBody = $("<div>").addClass("card-body p-4");
  cardBody.append($("<h3>").addClass("card-title text-center mb-4").text("Personal Favorites"));

  const mostSearchedDiv = $("<div>").addClass("favorites-content text-center");
  mostSearchedDiv.append($("<h5>").addClass("mb-3").text("Most Searched Place"));

  const locationText = favorites.most_frequent_search?.length > 0
    ? favorites.most_frequent_search
    : "No searches yet";
  mostSearchedDiv.append($("<p>").addClass("fs-4").text(locationText));

  cardBody.append(mostSearchedDiv);
  slideInner.append(cardBody);
  slide.append(slideInner);
  wrapper.append(slide);
}

function createPlanSlide(plan) {
  const wrapper = $("#profile-swiper-wrapper");

  const slide = $("<div>").addClass("swiper-slide");
  const slideInner = $("<div>").addClass("swiper-slide-inner profile-plan-slide");

  const cardBody = $("<div>").addClass("card-body p-4");
  cardBody.append($("<h4>").addClass("card-title").text(plan.destination));
  cardBody.append($("<h6>").addClass("card-subtitle mb-3 text-muted").text(plan.travel_date));

  const placeList = $("<ul>").addClass("list-group list-group-flush mb-3");
  plan.places.forEach((place) => {
    let p = $("<li>").addClass("list-group-item");
    p.append(
      $("<a>")
        .addClass("card-link")
        .attr("href", place.url)
        .attr("target", "_blank")
        .text(place.place_name)
    );
    placeList.append(p);
  });
  cardBody.append(placeList);

  const buttonGroup = $("<div>").addClass("d-flex justify-content-end gap-2 mt-3");

  let deleteButton = $("<button>")
    .addClass("btn btn-outline-warning")
    .attr("type", "button")
    .attr("data-planid", `${plan.id}`)
    .attr("data-user", `${username}`)
    .text("delete");
  deleteButton.click(deleteUserPlan);
  buttonGroup.append(deleteButton);

  let showDetailsButton = $("<button>")
    .addClass("btn btn-outline-info")
    .attr("type", "button")
    .attr("data-planid", `${plan.id}`)
    .text("details");
  showDetailsButton.click(showPlanDetails);
  buttonGroup.append(showDetailsButton);

  cardBody.append(buttonGroup);
  slideInner.append(cardBody);
  slide.append(slideInner);
  wrapper.append(slide);
}

function showPlanDetails() {
  const planID = this.dataset.planid;
  window.location = `users/plan/${planID}`;
}

function getUserPlans() {
  const url = `/v1/users/${username}/plans`;
  return axios
    .get(url)
    .then((response) => {
      const data = response.data;
      const plans = data["travel_plans"];
      return plans || [];
    })
    .catch((err) => {
      console.error(err);
      return [];
    });
}

function getUserFavorites() {
  const favoritesUrl = `/v1/users/${username}/favorites`;
  return axios
    .get(favoritesUrl)
    .then((response) => {
      return response.data;
    })
    .catch((err) => {
      console.error(err);
      return {};
    });
}

function initializeSwiper() {
  profileSwiper = new Swiper('#profileSwiper', {
    // Vertical direction for timeline effect
    direction: 'vertical',

    // Effect configuration for card stack
    effect: 'slide',
    grabCursor: true,
    centeredSlides: true,
    slidesPerView: 'auto',

    // Navigation arrows
    navigation: {
      nextEl: '.swiper-button-next',
      prevEl: '.swiper-button-prev',
    },

    // Pagination bullets
    pagination: {
      el: '.swiper-pagination',
      clickable: true,
    },

    // Keyboard control
    keyboard: {
      enabled: true,
    },

    // Mouse wheel control for vertical scrolling
    mousewheel: {
      enabled: true,
      forceToAxis: true,
    },

    // Loop mode
    loop: false,

    // Speed of transition
    speed: 400,

    // Space between slides
    spaceBetween: 30,
  });
}

async function renderUserProfile() {
  const [plans, favorites] = await Promise.all([getUserPlans(), getUserFavorites()]);

  // Create favorites slide first
  createFavoritesSlide(favorites);

  // Create plan slides
  if (plans.length > 0) {
    plans.forEach(plan => createPlanSlide(plan));
  }

  // Initialize Swiper after all slides are added
  initializeSwiper();
}

renderUserProfile().then(() => console.log("user profile is loaded"));
