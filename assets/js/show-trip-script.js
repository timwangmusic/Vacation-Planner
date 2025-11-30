/**
 * Run in the script mode by trip_plan_details_template.html.
 * Initialize Swiper for trip plan details with overlapping cards effect.
 * Define a function "getTravelPlan" to get plan data in JSON format.
 */

// Initialize Swiper with card stack effect
const swiper = new Swiper('#tripSwiper', {
  // Effect configuration for card stack
  effect: 'coverflow',
  grabCursor: true,
  centeredSlides: true,
  slidesPerView: 'auto',

  // Coverflow effect parameters
  coverflowEffect: {
    rotate: 0,
    stretch: 0,
    depth: 100,
    modifier: 1.5,
    slideShadows: false,
  },

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

  // Loop mode for better UX
  loop: false,

  // Speed of transition
  speed: 400,
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
