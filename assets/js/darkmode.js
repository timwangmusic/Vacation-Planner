function applyDarkModePreference() {
    if (localStorage.getItem('darkMode') === 'enabled') {
        document.body.classList.add('dark-mode');
        document.getElementById('darkModeToggle').checked = true;
    }
}

document.addEventListener('DOMContentLoaded', function() {
    applyDarkModePreference();

    document.getElementById('darkModeToggle').addEventListener('change', function() {
        document.body.classList.toggle('dark-mode', this.checked);
        if (this.checked) {
            localStorage.setItem('darkMode', 'enabled');
        } else {
            localStorage.setItem('darkMode', 'disabled');
        }
    });
});