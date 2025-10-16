import { useState, useEffect } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { ScanControls } from './ScanControls';
import { ScanProgress } from './ScanProgress';
import { ReportView } from './ReportView';

const REPORT_STORAGE_KEY = 'walmart_order_report';
const REPORT_TIMESTAMP_KEY = 'walmart_order_report_timestamp';
const REPORT_MAX_AGE_MS = 7 * 24 * 60 * 60 * 1000; // 7 days

export function Dashboard() {
  const { user, logout } = useAuth();
  const [scanning, setScanning] = useState(false);
  const [reportData, setReportData] = useState(null);

  // Restore report from localStorage on mount
  useEffect(() => {
    try {
      const storedReport = localStorage.getItem(REPORT_STORAGE_KEY);
      const storedTimestamp = localStorage.getItem(REPORT_TIMESTAMP_KEY);

      if (storedReport && storedTimestamp) {
        const timestamp = parseInt(storedTimestamp, 10);
        const age = Date.now() - timestamp;

        // Only restore if data is less than 7 days old
        if (age < REPORT_MAX_AGE_MS) {
          const parsedReport = JSON.parse(storedReport);
          setReportData(parsedReport);
        } else {
          // Clear stale data
          localStorage.removeItem(REPORT_STORAGE_KEY);
          localStorage.removeItem(REPORT_TIMESTAMP_KEY);
        }
      }
    } catch (error) {
      console.error('Failed to restore report from localStorage:', error);
      // Clear corrupted data
      localStorage.removeItem(REPORT_STORAGE_KEY);
      localStorage.removeItem(REPORT_TIMESTAMP_KEY);
    }
  }, []);

  // Save report to localStorage whenever it changes
  useEffect(() => {
    if (reportData) {
      try {
        localStorage.setItem(REPORT_STORAGE_KEY, JSON.stringify(reportData));
        localStorage.setItem(REPORT_TIMESTAMP_KEY, Date.now().toString());
      } catch (error) {
        console.error('Failed to save report to localStorage:', error);
        // Handle quota exceeded or other errors
        if (error.name === 'QuotaExceededError') {
          console.warn('localStorage quota exceeded, clearing old data');
          localStorage.removeItem(REPORT_STORAGE_KEY);
          localStorage.removeItem(REPORT_TIMESTAMP_KEY);
        }
      }
    }
  }, [reportData]);

  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="bg-panel border-b border-muted/10">
        <div className="max-w-7xl mx-auto px-4 py-4 flex items-center justify-between">
          <h1 className="text-2xl font-bold">Walmart Order Checker</h1>
          <div className="flex items-center gap-4">
            <span className="text-sm text-muted">{user?.email}</span>
            <button
              onClick={logout}
              className="text-sm text-muted hover:text-text transition-colors"
            >
              Logout
            </button>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 py-8 space-y-8">
        <ScanControls
          onScanStart={() => {
            setScanning(true);
            setReportData(null); // Clear previous report/error
            // Clear localStorage when starting new scan
            localStorage.removeItem(REPORT_STORAGE_KEY);
            localStorage.removeItem(REPORT_TIMESTAMP_KEY);
          }}
          onScanComplete={(data) => {
            setScanning(false);
            setReportData(data);
          }}
        />

        {scanning && <ScanProgress />}

        {/* Error Display */}
        {reportData?.error && (
          <div className="bg-danger/10 border border-danger/20 rounded-lg p-6">
            <div className="flex items-start gap-3">
              <div className="text-2xl">⚠️</div>
              <div className="flex-1">
                <h3 className="text-lg font-semibold text-danger mb-2">Scan Failed</h3>
                <p className="text-danger/90 mb-4">{reportData.error}</p>
                <button
                  onClick={() => setReportData(null)}
                  className="bg-danger hover:bg-danger/90 text-white font-medium py-2 px-4 rounded-lg transition-colors"
                >
                  Dismiss
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Report Display - only show if no error */}
        {reportData && !reportData.error && <ReportView data={reportData} />}
      </main>
    </div>
  );
}
