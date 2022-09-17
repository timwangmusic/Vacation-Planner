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

document.querySelectorAll("button[id^=delete-plan]").forEach(
    btn => btn.addEventListener("click", deleteUserPlan)
);
