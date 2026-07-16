import { useState, useRef, useEffect, useCallback } from 'react';
import { DownloadIcon, CopyIcon } from './Icons';
import { toast } from './toastBus';

interface LogViewerProps {
  lines: string[];
  // Name used for the downloaded file, e.g. "my-app.log".
  filename: string;
  // Shown when there are no lines at all.
  emptyText?: string;
  // When the buffer is capped, tells the reader what they're looking at.
  capNotice?: string;
  compact?: boolean;
}

/*
LogViewer renders a terminal pane with the tooling a log actually
needs: filter, copy, download, and follow. Follow auto-scrolls to new
output but steps aside the moment the reader scrolls up; scrolling back
to the bottom re-engages it.
*/
export default function LogViewer({ lines, filename, emptyText = 'No output yet', capNotice, compact = false }: LogViewerProps) {
  const [filter, setFilter] = useState('');
  const [follow, setFollow] = useState(true);
  const paneRef = useRef<HTMLDivElement>(null);

  const visible = filter
    ? lines.filter(l => l.toLowerCase().includes(filter.toLowerCase()))
    : lines;

  useEffect(() => {
    if (follow && paneRef.current) {
      paneRef.current.scrollTop = paneRef.current.scrollHeight;
    }
  }, [visible.length, follow]);

  const onScroll = useCallback(() => {
    const el = paneRef.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 16;
    setFollow(atBottom);
  }, []);

  function download() {
    const blob = new Blob([lines.join('\n') + '\n'], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
  }

  async function copyAll() {
    try {
      await navigator.clipboard.writeText(visible.join('\n'));
      toast('Copied to clipboard');
    } catch {
      toast('Could not copy to clipboard', 'error');
    }
  }

  return (
    <div className="logviewer">
      <div className="logviewer-toolbar">
        <input
          className="logviewer-filter"
          type="search"
          placeholder="Filter lines"
          value={filter}
          onChange={e => setFilter(e.target.value)}
          aria-label="Filter log lines"
        />
        <span className="logviewer-count">
          {filter ? `${visible.length} of ${lines.length}` : `${lines.length} lines`}
        </span>
        <div className="logviewer-actions">
          {!follow && (
            <button
              className="btn btn-sm btn-secondary"
              onClick={() => {
                setFollow(true);
                if (paneRef.current) paneRef.current.scrollTop = paneRef.current.scrollHeight;
              }}
            >
              Follow
            </button>
          )}
          <button className="btn btn-sm btn-ghost btn-icon" onClick={copyAll} title="Copy" aria-label="Copy log">
            <CopyIcon />
          </button>
          <button className="btn btn-sm btn-ghost btn-icon" onClick={download} title="Download" aria-label="Download log">
            <DownloadIcon />
          </button>
        </div>
      </div>
      <div
        ref={paneRef}
        className={`log-output ${compact ? 'log-output-compact' : ''}`}
        onScroll={onScroll}
        tabIndex={0}
        aria-label="Log output"
      >
        {visible.length > 0 ? visible.join('\n') : (filter ? 'No lines match the filter' : emptyText)}
      </div>
      {capNotice && lines.length > 0 && <div className="logviewer-notice">{capNotice}</div>}
    </div>
  );
}
