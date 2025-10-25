import { createContext, useContext, useState, useEffect } from 'react';
import Cookies from 'js-cookie';

const AuthContext = createContext(null);

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};

export const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);

  // Decode JWT token to get user info
  const decodeToken = (token) => {
    try {
      const base64Url = token.split('.')[1];
      const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
      const jsonPayload = decodeURIComponent(
        atob(base64)
          .split('')
          .map((c) => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
          .join('')
      );
      return JSON.parse(jsonPayload);
    } catch (error) {
      console.error('Error decoding token:', error);
      return null;
    }
  };

  // Check for existing JWT token on mount
  useEffect(() => {
    const token = Cookies.get('JWT');
    if (token) {
      const decoded = decodeToken(token);
      if (decoded) {
        setUser({
          username: decoded.username,
          email: decoded.email,
          token: token
        });
      }
    }
    setLoading(false);
  }, []);

  const login = (token) => {
    Cookies.set('JWT', token, { expires: 7 }); // 7 days
    const decoded = decodeToken(token);
    if (decoded) {
      setUser({
        username: decoded.username,
        email: decoded.email,
        token: token
      });
    }
  };

  const logout = () => {
    Cookies.remove('JWT');
    setUser(null);
  };

  const value = {
    user,
    login,
    logout,
    loading,
    isAuthenticated: !!user
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};
