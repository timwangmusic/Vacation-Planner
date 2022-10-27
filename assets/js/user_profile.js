import { updateUsername } from './user.js';

const username = updateUsername();

async function deleteUserPlan() {
    const username = this.dataset.user;
    const planId = this.dataset.planid;
    const url = `/v1/users/${username}/plan/${planId}`;

    await axios.delete(
        url
    ).then(
        response => {
            console.log(response.status)
            location.reload();
        }
    ).catch(
        err => console.error(err)
    );
}

function renderCard(plan) {
    const cards = $('#cards');
    let card = $('<div>').addClass('card shadow-sm border rounded mb-2 w-75').css('max-width', '350px');
    let cardBody = $('<div>').addClass('card-body');
    cardBody.append($('<h5>').addClass('card-title').text(plan.destination));
    cardBody.append($('<h6>').addClass('card-subtitle').text(plan.travel_date));
    let placeList = $('<ul>').addClass('list-group list-group-flush');
    plan.places.forEach(
        place => {
            let p = $('<li>').addClass('list-group-item');
            p.append($('<a>').addClass('card-link').attr('href', place.url).text(place.place_name));
            placeList.append(p);
        }
    )
    cardBody.append(placeList);
    let deleteButton = $('<button>')
        .addClass('btn btn-outline-warning float-end')
        .attr('type', 'button')
        .attr('data-planId', `${plan.id}`)
        .attr('data-user', `${username}`)
        .text('delete');
    deleteButton.click(deleteUserPlan);
    cardBody.append(deleteButton);

    card.append(cardBody);
    cards.append(card);
}

async function getUserPlans() {
    const url = `/v1/users/${username}/plans`;
    await axios.get(
        url
    ).then(
        response => {
            const data = response.data;
            const plans = data['travel_plans']
            console.log('user plans: ' + plans);
            if (plans.length > 0) {
                for (let i = 0; i < plans.length; i++) {
                    renderCard(plans[i]);
                }
            }
        }
    ).catch(
        err => console.error(err)
    );
}

window.onload = getUserPlans;
