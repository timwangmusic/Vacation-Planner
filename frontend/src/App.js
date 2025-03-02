import "bootstrap/dist/css/bootstrap.min.css";
import { Title } from "./title";
import "./App.css";
import { RequestForm } from "./requestForm";
import DarkModeToggle from "./DarkModeToggle";

function App() {
  return (
    <>
      <div className="min-h-screen bg-white dark:bg-gray-900 text-black dark:text-white p-6">
        <DarkModeToggle />
          <div className="min-h-screen bg-white dark:bg-gray-900 text-black dark:text-white p-6">
              <Title />
              <RequestForm />
          </div>
      </div>
    </>
  );
}

export default App;
