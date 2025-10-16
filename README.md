# Walmart Order Checker

<p align="center">
  <img width="560" src="./.github/assets/template-preview.png">
</p>

A modern web application and CLI tool for tracking and analyzing Walmart orders from your Gmail account. Scans email confirmations to provide comprehensive order history, analytics, and tracking information.

## Features

- 🔐 **Secure OAuth 2.0** - Google account authentication with encrypted token storage
- 📊 **Real-time Scanning** - Live progress updates via WebSocket
- 💾 **Smart Caching** - Fast subsequent scans with SQLite cache (24-hour TTL)
- 📈 **Order Analytics** - View spending patterns, cancellations, and order history
- 🚀 **Rate Limit Handling** - Automatic detection and graceful handling of Gmail API limits
- 🎨 **Modern UI** - Clean dark theme with Tailwind CSS
- 💿 **Persistent Reports** - Reports saved in browser localStorage (7-day expiry)

## Quick Start

### Prerequisites

- Go 1.21 or higher
- Node.js 18+ and npm
- Google Cloud Project with Gmail API enabled

### 1. Google Cloud Setup

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select existing one
3. Enable the Gmail API:
   - Navigate to "APIs & Services" > "Library"
   - Search for "Gmail API" and enable it
4. Create OAuth 2.0 credentials:
   - Go to "APIs & Services" > "Credentials"
   - Click "Create Credentials" > "OAuth 2.0 Client ID"
   - Application type: **Web application**
   - Add authorized redirect URI: `http://localhost:3000/api/auth/callback`
   - Download the credentials JSON

### 2. Environment Setup

```bash
# Copy environment template
cp .env.example .env

# Generate secure keys
go run ./cmd/tools/generate-keys.go

# Edit .env and add your OAuth credentials
nano .env
```

Your `.env` should contain:
```env
GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-client-secret
REDIRECT_URL=http://localhost:3000/api/auth/callback
SESSION_KEY=<generated-key>
ENCRYPTION_KEY=<generated-key>
```

### 3. Build & Run

```bash
# Build backend
go build -o bin/web-server ./cmd/web

# Build frontend
cd web
npm install
npm run build
cd ..

# Start the server
./bin/web-server
```

Open `http://localhost:3000` in your browser.

## Development Mode

Run with hot-reload for active development:

**Terminal 1 - Backend:**
```bash
go run ./cmd/web/main.go
```

**Terminal 2 - Frontend:**
```bash
cd web
npm run dev
```

Frontend dev server runs at `http://localhost:5173` with API proxy to backend.

## Usage

1. **Login**: Click "Connect with Google" and authorize Gmail access
2. **Configure Scan**: Select date range (7 days to 1 year)
3. **Optional**: Check "Clear cache" for fresh data (slower but ensures latest information)
4. **Start Scan**: Real-time progress updates via WebSocket
5. **View Results**:
   - Total spending and order statistics
   - Live orders with tracking information
   - Cancellation history
   - Detailed order tables with product images
6. **Persistence**: Reports automatically saved to browser localStorage
   - Survives page refreshes and browser restarts
   - Auto-expires after 7 days

## Architecture

### Backend (Go)
```
cmd/
├── web/          # Web server with OAuth and WebSocket
├── cli/          # Legacy CLI tool
└── tools/        # Utilities (key generation)

internal/
├── api/          # HTTP handlers, WebSocket, rate limiting
├── auth/         # OAuth manager
├── security/     # Key management and permissions
└── storage/      # Encrypted token storage

pkg/
├── gmail/        # Gmail API client, email processing, caching
├── report/       # Report generation and data structures
└── util/         # Shared utilities
```

**Key Technologies:**
- **Chi Router**: Fast HTTP router with middleware
- **OAuth 2.0**: Secure authentication with AES-GCM encrypted tokens
- **WebSocket**: Real-time scan progress (100ms update interval)
- **SQLite**: Persistent cache with WAL mode for concurrency
- **Goroutines**: Parallel processing (24 workers)

### Frontend (React)
```
web/src/
├── components/   # React components (Dashboard, ScanControls, ReportView)
├── contexts/     # Auth context for state management
└── hooks/        # Custom hooks (WebSocket)
```

**Key Technologies:**
- **Vite**: Fast build tool and dev server
- **Tailwind CSS**: Utility-first styling
- **localStorage**: Client-side report persistence
- **Context API**: Global auth state

## API Endpoints

### Authentication
- `GET /api/auth/login` - Initiate OAuth flow
- `GET /api/auth/callback` - OAuth callback handler
- `POST /api/auth/logout` - Clear session
- `GET /api/auth/status` - Check authentication status

### Scanning
- `POST /api/scan` - Start email scan (body: `{days: int, clear_cache: bool}`)
- `GET /api/scan/status` - Poll scan progress
- `GET /api/report` - Fetch completed report
- `WS /api/ws/scan` - WebSocket for real-time updates

### Cache Management
- `GET /api/cache/stats` - Cache statistics
- `DELETE /api/cache/clear` - Clear message cache

## Security

- ✅ **OAuth tokens** encrypted with AES-256-GCM
- ✅ **HttpOnly session cookies** prevent XSS attacks
- ✅ **CSRF protection** with state parameter
- ✅ **Read-only Gmail access** (limited scope)
- ✅ **No email storage** (only parsed order metadata)
- ✅ **Rate limit detection** prevents API abuse
- ✅ **Secure key generation** via crypto/rand

See [SECURITY.md](SECURITY.md) for detailed security information.

## Troubleshooting

### "Unauthorized" or "Invalid credentials" errors
- Verify `.env` has correct `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET`
- Check OAuth redirect URL matches Google Console: `http://localhost:3000/api/auth/callback`
- Ensure Gmail API is enabled in Google Cloud Console

### Scan fails with "Rate limit exceeded"
- Gmail API has quota limits (15,000 queries/minute/user)
- Wait 2-5 minutes and try again
- The app automatically detects and aborts scans when rate limited

### WebSocket connection failed
- Ensure backend server is running on port 3000
- Check browser console for detailed error messages
- Verify firewall/proxy settings allow WebSocket connections

### Cache not working / Slow scans
- Check `.cache/` directory exists and is writable
- Cache uses SQLite with 24-hour TTL
- Disable cache by checking "Clear cache" option during scan

### Frontend not loading
- Ensure you've built the frontend: `cd web && npm run build`
- Check `web/dist/` directory exists with built assets
- Try clearing browser cache and hard refresh (Ctrl+Shift+R)

## CLI Tool (Legacy)

The original command-line tool is still available for advanced users:

```bash
# Build CLI
go build -o bin/cli ./cmd/cli

# Run with credentials.json in current directory
./bin/cli --days 365

# Multi-account support
mkdir account1@gmail.com
# Place credentials.json in account folder
./bin/cli
```

The CLI tool generates HTML and CSV reports in the `out/` directory.

## Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Support

For issues or questions:
- Open a GitHub issue with error messages and logs
- Include browser console logs for frontend issues
- Include server logs for backend issues
- Provide steps to reproduce the problem

## Acknowledgments

- Gmail API by Google
- Built with Go, React, Vite, and Tailwind CSS
- Email parsing with goquery
- Progress bars by schollz/progressbar
