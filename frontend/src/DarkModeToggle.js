import { useEffect, useState } from "react";

export default function DarkModeToggle() {
    const [darkMode, setDarkMode] = useState(
        localStorage.getItem("theme") === "dark"
    );
    useEffect(() => {
        if(darkMode) {
            document.documentElement.classList.add("dark");
            localStorage.setItem("theme", "dark");
        }
        else {
            document.documentElement.classList.remove("dark");
            localStorage.setItem("theme", "light");
        }
    }, [darkMode]);
    return (
      <button
      onClick={() => setDarkMode(!darkMode)}
      className="p-2 rounded-lg bg-gray-200 dark:bg-gray-800 text-gray-900 dark:text-white"
      >
          {darkMode ? "🌙 Dark Mode" : "☀️ Light Mode"}
      </button>
    );
}