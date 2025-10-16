import { useWebSocket } from '../hooks/useWebSocket';

export function ScanProgress() {
  const { data: progress } = useWebSocket('/api/ws/scan');

  if (!progress || !progress.in_progress) {
    return null;
  }

  const percentage = progress.total_messages > 0
    ? Math.round((progress.processed / progress.total_messages) * 100)
    : 0;

  const elapsed = progress.start_time
    ? Math.round((Date.now() - new Date(progress.start_time).getTime()) / 1000)
    : 0;

  // Calculate estimated time remaining
  const estimatedTotal = progress.processed > 0
    ? (elapsed / progress.processed) * progress.total_messages
    : 0;
  const remaining = Math.max(0, Math.round(estimatedTotal - elapsed));

  // Format time as MM:SS
  const formatTime = (seconds) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

  return (
    <div className="bg-panel rounded-lg p-6 shadow-lg">
      <div className="space-y-4">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="relative">
              <div className="w-10 h-10 rounded-full bg-primary/20 flex items-center justify-center">
                <div className="w-6 h-6 border-3 border-primary border-t-transparent rounded-full animate-spin" />
              </div>
            </div>
            <div>
              <h3 className="text-lg font-semibold">Scanning Emails</h3>
              <p className="text-xs text-muted">{progress.days_scanned} days</p>
            </div>
          </div>
          <div className="text-right">
            <div className="text-2xl font-bold text-primary">{percentage}%</div>
            <div className="text-xs text-muted">Complete</div>
          </div>
        </div>

        {/* Progress Bar */}
        <div className="space-y-2">
          <div className="w-full bg-bg rounded-full h-4 overflow-hidden shadow-inner">
            <div
              className="h-full transition-all duration-500 ease-out rounded-full relative overflow-hidden"
              style={{
                width: `${percentage}%`,
                background: 'linear-gradient(90deg, #3b82f6 0%, #60a5fa 50%, #93c5fd 100%)',
                boxShadow: percentage > 0 ? '0 0 10px rgba(59, 130, 246, 0.5)' : 'none'
              }}
            >
              {/* Animated shine effect */}
              {percentage > 0 && percentage < 100 && (
                <div
                  className="absolute inset-0 opacity-30"
                  style={{
                    background: 'linear-gradient(90deg, transparent 0%, rgba(255,255,255,0.8) 50%, transparent 100%)',
                    animation: 'shimmer 2s infinite',
                  }}
                />
              )}
            </div>
          </div>

          {/* Progress Stats */}
          <div className="flex justify-between text-sm">
            <span className="text-muted">
              {progress.processed.toLocaleString()} / {progress.total_messages.toLocaleString()} messages
            </span>
            <span className="text-muted">
              {progress.processed > 0 && remaining > 0 ? (
                <span>~{formatTime(remaining)} remaining</span>
              ) : (
                <span>Calculating...</span>
              )}
            </span>
          </div>
        </div>

        {/* Time Info */}
        <div className="flex justify-between items-center pt-2 border-t border-border/50">
          <div className="text-sm">
            <span className="text-muted">Elapsed: </span>
            <span className="font-medium">{formatTime(elapsed)}</span>
          </div>
          <div className="text-xs text-muted">
            {progress.current_email}
          </div>
        </div>

        {/* Error Display */}
        {progress.error && (
          <div className="bg-danger/10 border border-danger/20 rounded-lg p-3 text-danger text-sm">
            <div className="font-semibold mb-1">Error</div>
            {progress.error}
          </div>
        )}
      </div>

      {/* Add shimmer animation */}
      <style jsx>{`
        @keyframes shimmer {
          0% { transform: translateX(-100%); }
          100% { transform: translateX(100%); }
        }
      `}</style>
    </div>
  );
}
