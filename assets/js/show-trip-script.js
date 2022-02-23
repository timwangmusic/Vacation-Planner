/**
 * Run in the script mode by trip_plan_details_template.html.
 * Display and hide corresponding location info in carousel's cardBodies. 
 * Define a function "getTravelPlan" to get plan data in JSON format.
 */

// Control display and hide of location to match photos in slides
const cardBodies = document.querySelectorAll("div.card div.card-body");
const myCarousel = document.querySelector('#carouselExampleIndicators');

myCarousel.addEventListener('slide.bs.carousel', function (evt) {
    cardBodies[evt.from].classList.add("d-none");
    cardBodies[evt.to].classList.remove("d-none");
})

// Get plan JSON data
async function getTravelPlan() {
    const url = toJSONPlanURL(document.URL)
    const config = {
        headers: {
            'Content-Type': 'application/json'
        },
    };
    try {
        let res = await axios.get(url, config)
        return res.data
    }
    catch (err) {
        console.log(err)
    }
}

function toJSONPlanURL(urlStr) {
    const url = new URL(urlStr)
    const searchParams = new URLSearchParams(url.search);
    searchParams.append('json_only', 'true')
    url.search = searchParams.toString()
    return url.toString();
}
