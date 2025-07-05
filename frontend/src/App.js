import "bootstrap/dist/css/bootstrap.min.css";
import { ThemeProvider } from './contexts/ThemeContext';
import { Title } from "./title";
import "./App.css";
import { RequestForm } from "./requestForm";
import DarkModeToggle from "./DarkModeToggle";

function App() {
    return (
        <ThemeProvider>
            <div className="min-vh-100 p-3">
                <DarkModeToggle />
                <Title />
                <RequestForm />
            </div>
        </ThemeProvider>
    );
}

export default App;