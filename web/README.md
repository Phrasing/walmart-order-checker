# Walmart Order Checker - Frontend

React + Vite frontend for the Walmart Order Checker web application.

## Tech Stack

- **React 18** - UI library
- **Vite** - Fast build tool and dev server
- **Tailwind CSS** - Utility-first styling with custom dark theme
- **WebSocket** - Real-time scan progress updates

## Development

```bash
# Install dependencies
npm install

# Start dev server (with hot reload)
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview

# Lint code
npm run lint
```

## Project Structure

```
src/
├── components/
│   ├── Dashboard.jsx       # Main authenticated view
│   ├── LandingPage.jsx     # Unauthenticated landing page
│   ├── ScanControls.jsx    # Scan configuration form
│   ├── ScanProgress.jsx    # Real-time progress display
│   └── ReportView.jsx      # Order analytics and tables
├── contexts/
│   └── AuthContext.jsx     # Global authentication state
├── hooks/
│   └── useWebSocket.js     # WebSocket connection hook
├── App.jsx                 # Root component and routing
├── main.jsx                # React entry point
└── index.css               # Tailwind imports and global styles
```

## Key Features

### Authentication
- OAuth 2.0 flow with Google
- Session management via httpOnly cookies
- Protected routes based on auth state

### Real-time Updates
- WebSocket connection for scan progress
- 100ms update interval for smooth animations
- Automatic reconnection handling
- Ping/pong keepalive mechanism

### Report Persistence
- Automatic save to localStorage on scan completion
- 7-day expiration for cached reports
- Survives page refreshes and browser restarts
- Graceful handling of quota exceeded errors

### Responsive Design
- Mobile-friendly layout
- Dark theme optimized for readability
- Gradient progress bars with animations
- Accessible UI components

## Environment Variables

The frontend proxies API requests to the backend in development mode. Configure proxy settings in `vite.config.js`:

```javascript
server: {
  proxy: {
    '/api': 'http://localhost:3000'
  }
}
```

## Building for Production

The production build is served by the Go backend from the `dist/` directory:

```bash
npm run build
```

The backend serves the frontend at `http://localhost:3000/`.

## Troubleshooting

### WebSocket connection fails
- Ensure backend is running on port 3000
- Check browser console for detailed errors
- Verify CORS/WebSocket origin configuration in backend

### Styles not updating
- Clear browser cache
- Restart Vite dev server
- Check Tailwind config is valid

### localStorage errors
- Check browser storage quota
- Clear old localStorage data
- Ensure browser allows localStorage

## Related Documentation

See main [README.md](../README.md) in project root for complete setup instructions.
