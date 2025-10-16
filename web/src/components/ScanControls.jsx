import { useState } from 'react';

export function ScanControls({ onScanStart, onScanComplete }) {
  const [days, setDays] = useState(365);
  const [clearCache, setClearCache] = useState(false);
  const [loading, setLoading] = useState(false);

  const handleScan = async () => {
    setLoading(true);
    onScanStart();

    try {
      const response = await fetch('/api/scan', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ days, clear_cache: clearCache }),
      });

      if (!response.ok) {
        throw new Error('Scan failed');
      }

      const pollStatus = setInterval(async () => {
        const statusResponse = await fetch('/api/scan/status', {
          credentials: 'include',
        });
        const status = await statusResponse.json();

        if (!status.in_progress) {
          clearInterval(pollStatus);
          setLoading(false);

          // Check if scan completed with error
          if (status.error) {
            onScanComplete({ error: status.error });
            return;
          }

          // Fetch report on successful completion
          const reportResponse = await fetch('/api/report', {
            credentials: 'include',
          });

          if (!reportResponse.ok) {
            onScanComplete({ error: 'Failed to load report' });
            return;
          }

          const reportData = await reportResponse.json();
          onScanComplete(reportData);
        }
      }, 1000);
    } catch (error) {
      console.error('Scan error:', error);
      setLoading(false);
      onScanComplete(null);
    }
  };

  return (
    <div className="bg-panel rounded-lg p-6">
      <h2 className="text-xl font-semibold mb-4">Start New Scan</h2>

      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium mb-2">
            Days to Scan
          </label>
          <select
            value={days}
            onChange={(e) => setDays(Number(e.target.value))}
            className="w-full bg-bg border border-muted/20 rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-primary"
            disabled={loading}
          >
            <option value={7}>Last 7 days</option>
            <option value={30}>Last 30 days</option>
            <option value={90}>Last 90 days</option>
            <option value={180}>Last 6 months</option>
            <option value={365}>Last year</option>
          </select>
        </div>

        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            id="clearCache"
            checked={clearCache}
            onChange={(e) => setClearCache(e.target.checked)}
            className="w-4 h-4 rounded border-muted/20 text-primary focus:ring-primary"
            disabled={loading}
          />
          <label htmlFor="clearCache" className="text-sm text-muted">
            Clear cache before scanning (slower but gets latest data)
          </label>
        </div>

        <button
          onClick={handleScan}
          disabled={loading}
          className="w-full bg-primary hover:bg-primary/90 disabled:bg-muted/20 disabled:cursor-not-allowed text-white font-semibold py-3 px-6 rounded-lg transition-colors"
        >
          {loading ? 'Scanning...' : 'Start Scan'}
        </button>
      </div>
    </div>
  );
}
