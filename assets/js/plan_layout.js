const buttons = document.querySelectorAll(".card-button");
const expandAllPlansButton = document.getElementById("expand-all-plans-button");

for (const button of buttons) {
    button.addEventListener(
        'click',
        () => {
            if (button.getAttribute('aria-expanded') === 'true') {
                button.textContent = 'Show Plan';
            } else {
                button.textContent = 'Hide Plan';
            }
        }
    )
}

expandAllPlansButton.addEventListener(
    'click',
    () => {
        restoreCollapseButtonsTextContent(buttons);
    }
)

function restoreCollapseButtonsTextContent(collapseButtons) {
    collapseButtons.forEach(button => {
        if (button.getAttribute('aria-expanded') === 'true') {
            button.textContent = 'Show Plan';
        } else {
            button.textContent = 'Hide Plan';
        }
    })
}
