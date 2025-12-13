# Metron Kids - Child Self-Service Web App

A child-friendly Progressive Web App (PWA) for managing screen time within parent-approved daily limits.

## Features

- 🎮 **Child-Friendly UI** - Colorful, touch-optimized interface designed for kids
- 🔐 **PIN Authentication** - Secure 4-digit PIN login for each child
- ⏱️ **Time Tracking** - Visual circular progress indicator showing remaining time
- 📱 **Device Management** - Start and stop sessions on different devices (TV, iPad, etc.)
- 💾 **PWA Support** - Installable on mobile devices, works offline
- 🔄 **Auto-Refresh** - Real-time updates every 30 seconds
- 🎨 **Responsive Design** - Optimized for phones and tablets

## Tech Stack

- **React 18** with TypeScript
- **Vite** - Fast build tool and dev server
- **Tailwind CSS** - Utility-first styling
- **React Router** - Client-side routing
- **PWA** - Progressive Web App capabilities

## Development Setup

### Prerequisites

- Node.js 18+ and npm
- Running Metron backend API (see main project README)

### Installation

1. Install dependencies:
```bash
npm install
```

2. Create environment configuration (optional):
```bash
cp .env.example .env
```

3. Start the development server:
```bash
npm run dev
```

The app will be available at `http://localhost:5173` by default.

## Running Against Production API

To run the local development version against a production API:

1. Create a `.env` file:
```bash
cp .env.example .env
```

2. Edit `.env` and set your production API URL:
```
VITE_API_BASE=https://your-production-api.com
```

3. Start the dev server:
```bash
npm run dev
```

The app will now make API requests to your production backend while running the UI locally.

## Building for Production

Build the optimized production bundle:

```bash
npm run build
```

The built files will be in the `dist/` directory, ready to deploy to any static hosting service.

Preview the production build locally:

```bash
npm run preview
```

## Project Structure

```
src/
├── api/              # API client and TypeScript types
│   ├── client.ts     # API wrapper with session management
│   └── types.ts      # TypeScript interfaces
├── components/       # Reusable UI components
│   ├── ChildSelector.tsx    # Child selection grid
│   ├── TimeDisplay.tsx      # Circular progress indicator
│   ├── ActiveSession.tsx    # Current session card
│   ├── DeviceButton.tsx     # Device selection button
│   └── DurationPicker.tsx   # Time duration selector
├── context/          # React Context for state management
│   └── AppContext.tsx       # Global app state
├── pages/            # Page components
│   ├── LoginPage.tsx        # Login with child selection + PIN
│   └── HomePage.tsx         # Main app page
├── App.tsx           # Main app component with routing
├── main.tsx          # App entry point
└── index.css         # Global styles and Tailwind setup
```

## API Endpoints Used

The app communicates with these child-facing endpoints:

**Public (no auth required):**
- `GET /child/auth/children` - List children for login screen
- `POST /child/auth/login` - Authenticate with child ID + PIN
- `POST /child/auth/logout` - Logout and clear session

**Protected (require session):**
- `GET /child/me` - Get authenticated child profile
- `GET /child/today` - Get today's usage stats
- `GET /child/devices` - List available devices
- `GET /child/sessions` - List active sessions
- `POST /child/sessions` - Start new session
- `POST /child/sessions/:id/stop` - Stop session

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VITE_API_BASE` | Backend API base URL | `http://localhost:8080` |

## Authentication Flow

1. User opens app → sees list of children
2. User selects their name
3. User enters their 4-digit PIN
4. App authenticates and receives session ID
5. Session ID stored in localStorage
6. All subsequent requests include session ID
7. Session valid for 24 hours

## Session Management

- Sessions stored in localStorage and cookie
- Auto-logout on 401 responses
- Session shared across browser tabs
- Auto-refresh data every 30 seconds while authenticated

## PWA Features

- **Installable** - Can be installed to home screen on mobile devices
- **Offline Ready** - Service worker caches static assets
- **Auto-Update** - New versions auto-update when available
- **Portrait Optimized** - Designed for vertical mobile screens

## Troubleshooting

### Can't login
- Verify backend is running: `curl http://localhost:8080/health`
- Check child exists with PIN set via parent API
- Open browser console for detailed error messages

### API requests fail
- Check `VITE_API_BASE` environment variable
- Verify CORS is enabled on backend
- Check network tab in browser dev tools

### Session expired errors
- Session IDs expire after 24 hours
- Backend restart invalidates sessions
- Clear localStorage: `localStorage.clear()` in console

## License

Part of the Metron screen time management system.
