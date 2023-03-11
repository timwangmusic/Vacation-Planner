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

$('#set-new-password-form').submit(async () => {
    const url = new URL(document.URL);
    const email = url.searchParams.get('email');
    const oldPassword = document.querySelector('#old-password').value;
    const newPassword = document.querySelector('#new-password').value;
    const backendUrl = '/v1/reset-password-backend'
    const data = {
        'email': email,
        'old_password': oldPassword,
        'new_password': newPassword,
    }
    console.log(data);
    await axios.put(
        backendUrl, JSON.stringify(data), { timeout: 10000 }
    ).catch(
        err => console.error(err)
    );
    return false;
})
