import { useTheme } from './contexts/ThemeContext';

export function Title() {
  const { isDarkMode } = useTheme();

  return (
      <div className={`p-4 ${isDarkMode ? 'bg-darksmoke' : 'bg-white'}`}>
          <div className="flex items-center justify-center bg-whitesmoke dark:bg-darksmoke">
              <h1 className="text-6xl md:text-7xl font-bold text-[#24c1e0] dark:text-[#1aa1bd] text-center">
                  Vacation Planner
              </h1>
          </div>
      </div>
  );
}
