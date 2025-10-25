import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { AuthProvider } from './contexts/AuthContext';
import { ThemeProvider } from './contexts/ThemeContext';
import Navbar from './components/Navbar';
import Search from './pages/Search';
import Login from './pages/Login';
import Signup from './pages/Signup';
import About from './pages/About';
import 'bootstrap/dist/css/bootstrap.min.css';
import './styles/styles.css';

function App() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <Router>
          <div className="app-container">
            <Navbar />
            <Routes>
              <Route path="/" element={<Search />} />
              <Route path="/login" element={<Login />} />
              <Route path="/signup" element={<Signup />} />
              <Route path="/about" element={<About />} />
              {/* Add more routes as we implement pages */}
            </Routes>
          </div>
        </Router>
      </AuthProvider>
    </ThemeProvider>
  );
}

export default App;
