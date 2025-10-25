import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { useTheme } from '../contexts/ThemeContext';
import { Navbar as BSNavbar, Nav, Container, Button } from 'react-bootstrap';

const Navbar = () => {
  const { user, logout, isAuthenticated } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const navigate = useNavigate();

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  return (
    <BSNavbar bg={theme === 'dark' ? 'dark' : 'light'} variant={theme} expand="lg" className="mb-4">
      <Container>
        <BSNavbar.Brand as={Link} to="/">
          Vacation Planner
        </BSNavbar.Brand>
        <BSNavbar.Toggle aria-controls="basic-navbar-nav" />
        <BSNavbar.Collapse id="basic-navbar-nav">
          <Nav className="ms-auto align-items-center">
            <Nav.Link as={Link} to="/">Search</Nav.Link>
            <Nav.Link as={Link} to="/template">Plan Template</Nav.Link>
            <Nav.Link as={Link} to="/about">About</Nav.Link>

            {isAuthenticated ? (
              <>
                <Nav.Link as={Link} to="/profile">
                  Profile ({user?.username})
                </Nav.Link>
                <Button variant="outline-secondary" size="sm" className="ms-2" onClick={handleLogout}>
                  Logout
                </Button>
              </>
            ) : (
              <>
                <Nav.Link as={Link} to="/login">Login</Nav.Link>
                <Nav.Link as={Link} to="/signup">Sign Up</Nav.Link>
              </>
            )}

            <Button
              variant="outline-secondary"
              size="sm"
              className="ms-2"
              onClick={toggleTheme}
              title={`Switch to ${theme === 'light' ? 'dark' : 'light'} mode`}
            >
              {theme === 'light' ? '🌙' : '☀️'}
            </Button>
          </Nav>
        </BSNavbar.Collapse>
      </Container>
    </BSNavbar>
  );
};

export default Navbar;
