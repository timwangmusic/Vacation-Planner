$('#password-reset-submit-btn').click(() => $('#reset-email-sent-alert').removeClass('d-none'));

$('#password-reset-form').submit(async () => {
    const email = document.querySelector('#email').value;
    const url = `/v1/send-password-reset-email?email=${email}`;
    await axios.get(
        url
    ).catch(
        err => console.error(err)
    );
    return false;
})
