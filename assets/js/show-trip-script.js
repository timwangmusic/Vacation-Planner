/**
 * Run in the script mode by trip_plan_details_template.html.
 * Initialize Swiper for trip plan details with overlapping cards effect.
 * Define a function "getTravelPlan" to get plan data in JSON format.
 */

// Initialize Swiper
const swiper = new Swiper('#tripSwiper', {
  // Swiper configuration
  effect: 'slide',
  slidesPerView: 'auto',
  centeredSlides: true,
  spaceBetween: 30,

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

  // Touch/swipe settings
  touchRatio: 1,
  grabCursor: true,
});

// Get plan JSON data
async function getTravelPlan() {
  const url = toJSONPlanURL(document.URL);
  const config = {
    headers: {
      "Content-Type": "application/json",
    },
  };
  try {
    let res = await axios.get(url, config);
    return res.data;
  } catch (err) {
    console.log(err);
  }
}

function toJSONPlanURL(urlStr) {
  const url = new URL(urlStr);
  const searchParams = new URLSearchParams(url.search);
  searchParams.append("json_only", "true");
  url.search = searchParams.toString();
  return url.toString();
}
