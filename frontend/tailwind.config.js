/** @type {import('tailwindcss').Config}*/
module.exports = {
    darkMode: 'selector', // Enables selector dark mode
    content: ["./src/**/*.{js,jsx,ts,tsx}"],
    theme: {
        extend: {
            colors: {
                whitesmoke: "#f5f5f5",
                darksmoke: "#2a2a2a", // Choose a dark equivalent
            },
        },
    },
    plugins: [],
};