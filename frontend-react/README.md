# Vacation Planner - React Frontend

This is the React version of the Vacation Planner frontend, designed to work alongside the existing Go backend.

## Features

- ✅ React 18 with Vite for fast development
- ✅ React Router for client-side routing
- ✅ Bootstrap 5 + React-Bootstrap for UI components
- ✅ Dark/Light theme toggle (using existing CSS variables)
- ✅ JWT authentication with context
- ✅ Axios for API calls
- ✅ Proxy configuration for Go backend

## Tech Stack

- **React** 18
- **Vite** - Build tool
- **React Router** - Client-side routing
- **Bootstrap 5** - CSS framework
- **React Bootstrap** - Bootstrap components for React
- **Axios** - HTTP client
- **js-cookie** - Cookie management

## Prerequisites

1. **Go Backend** must be running on `http://localhost:8080`
2. Node.js 16+ installed

## Setup Instructions

### 1. Install Dependencies

```bash
cd frontend-react
npm install
```

### 2. Start the Development Server

```bash
npm run dev
```

The React app will start on `http://localhost:3000`

### 3. Start the Go Backend

In a separate terminal, from the project root:

```bash
go run main.go
```

The Go backend should be running on `http://localhost:8080`

## How It Works

### API Proxy

The React app proxies all `/v1/*` and `/stats/*` requests to the Go backend at `http://localhost:8080`. This is configured in `vite.config.js`.

### Authentication

- JWT tokens are stored in cookies (key: `JWT`)
- The `AuthContext` manages authentication state
- Login persists across page refreshes
- Token is automatically sent with API requests

### Theme Toggle

- Uses the same CSS variables from `assets/css/styles.css`
- Theme preference stored in `localStorage`
- Toggle between light and dark mode with navbar button

## Available Scripts

- `npm run dev` - Start development server (port 3000)
- `npm run build` - Build for production
- `npm run preview` - Preview production build

## Development Status

**Implemented:**
- ✅ Basic app structure with routing
- ✅ Authentication context and JWT handling
- ✅ Theme toggle (dark/light mode)
- ✅ Login page (functional)
- ✅ Navbar with navigation

**To Do:**
- ⏳ Search page with date picker and location autocomplete
- ⏳ Search results page
- ⏳ Trip details with Google Maps
- ⏳ User profile page
- ⏳ Signup page
- ⏳ Plan template builder

## Next Steps

You can start migrating pages incrementally. The existing vanilla JS app will continue to work while you build out the React version.

Test the React app by starting both:
1. Go backend: `go run main.go` (from project root)
2. React frontend: `npm run dev` (from frontend-react directory)

Visit `http://localhost:3000` to see the React app!
