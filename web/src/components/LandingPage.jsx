import { useAuth } from '../contexts/AuthContext';

export function LandingPage() {
  const { login } = useAuth();

  return (
    <div className="min-h-screen flex items-center justify-center px-4">
      <div className="max-w-2xl w-full text-center space-y-8">
        <div>
          <h1 className="text-5xl font-bold mb-4">
            Walmart Order Checker
          </h1>
          <p className="text-xl text-muted">
            Track, analyze, and monitor your Walmart orders from Gmail
          </p>
        </div>

        <div className="bg-panel rounded-lg p-8 space-y-6">
          <div className="space-y-4">
            <div className="flex items-start gap-3">
              <div className="text-2xl">📊</div>
              <div className="text-left">
                <h3 className="font-semibold">Order Analytics</h3>
                <p className="text-sm text-muted">
                  View spending patterns, cancellation rates, and order history
                </p>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <div className="text-2xl">🚀</div>
              <div className="text-left">
                <h3 className="font-semibold">Fast Processing</h3>
                <p className="text-sm text-muted">
                  Scan up to a year of emails in seconds with smart caching
                </p>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <div className="text-2xl">🔒</div>
              <div className="text-left">
                <h3 className="font-semibold">Secure & Private</h3>
                <p className="text-sm text-muted">
                  Your data stays secure with encrypted OAuth authentication
                </p>
              </div>
            </div>
          </div>

          <button
            onClick={login}
            className="relative w-full bg-primary hover:bg-primary/90 text-white font-semibold py-3 px-6 rounded-lg
             transition-all duration-200 ease-out
             hover:scale-105 active:scale-95
             focus:outline-none focus:ring-2 focus:ring-primary/40
             ring-0 active:ring-4 active:ring-primary/30"
          >
            Connect with Google
          </button>

          <p className="text-xs text-muted">
            We only request read-only access to your Gmail. We never store or share your emails.
          </p>
        </div>
      </div>
    </div>
  );
}
