import { updateUsername } from "./user.js";

const cardBodies = document.querySelectorAll("div.card div.card-body");
const myCarousel = document.querySelector('#carouselExampleIndicators');


myCarousel.addEventListener('slide.bs.carousel', function (evt) {
    // show new card body and hide the old one
    cardBodies[evt.from].classList.add("d-none");
    cardBodies[evt.to].classList.remove("d-none");
})

updateUsername(); 