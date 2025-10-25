import axios from 'axios';

// Create axios instance with default config
const api = axios.create({
  baseURL: '/v1',
  headers: {
    'Content-Type': 'application/json',
  },
  withCredentials: true, // Send cookies with requests
});

// Auth endpoints
export const authAPI = {
  login: (email, password) =>
    api.post('/login', { email, password }),

  signup: (email, password, username) =>
    api.post('/signup', { email, password, username }),

  forgotPassword: (email) =>
    api.get('/send-password-reset-email', { params: { email } }),

  resetPassword: (token, newPassword) =>
    api.put('/reset-password-backend', { token, new_password: newPassword }),
};

// Search and plans endpoints
export const plansAPI = {
  search: (params) =>
    api.get('/plans', { params }),

  getPlanById: (id) =>
    api.get(`/plans/${id}`),

  getOptimalPlan: (data) =>
    api.post('/optimal-plan', data),

  customize: (data) =>
    api.post('/customize', data),

  getNearbyCities: (data) =>
    api.post('/nearby-cities', data),

  getCities: (searchTerm) =>
    api.get('/cities', { params: { search: searchTerm } }),
};

// User endpoints
export const userAPI = {
  getProfile: (username) =>
    api.get(`/users/${username}/plans`),

  savePlan: (username, planData) =>
    api.post(`/users/${username}/plans`, planData),

  deletePlan: (username, planId) =>
    api.delete(`/users/${username}/plan/${planId}`),

  getFavorites: (username) =>
    api.get(`/users/${username}/favorites`),

  submitFeedback: (username, feedback) =>
    api.post(`/users/${username}/feedback`, feedback),
};

// Utility endpoints
export const utilsAPI = {
  reverseGeocode: (lat, lng) =>
    api.get('/reverse-geocoding', { params: { lat, lng } }),

  generateImage: (data) =>
    api.post('/gen_image', data),

  getPlanSummary: (data) =>
    api.post('/plan-summary', data),
};

export default api;
