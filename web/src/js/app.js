// app.js — Alpine.js components for dnsmon
// Loaded as a plain defer script (no ES module) so it runs before Alpine initializes.

// ---------------------------------------------------------------------------
// Dark mode
// ---------------------------------------------------------------------------

if (
  localStorage.theme === 'dark' ||
  (!('theme' in localStorage) && window.matchMedia('(prefers-color-scheme: dark)').matches)
) {
  document.documentElement.classList.add('dark');
}

/** Toggle dark mode and persist the preference. */
window.toggleDark = function toggleDark() {
  document.documentElement.classList.toggle('dark');
  localStorage.theme = document.documentElement.classList.contains('dark') ? 'dark' : 'light';
};

// ---------------------------------------------------------------------------
// Country flag helper (emoji flag from ISO-3166 alpha-2)
// ---------------------------------------------------------------------------
function countryFlag(code) {
  if (!code || code.length !== 2) return '';
  const codePoints = [...code.toUpperCase()].map(
    (c) => 0x1f1e6 + c.charCodeAt(0) - 65
  );
  return String.fromCodePoint(...codePoints);
}

// ---------------------------------------------------------------------------
// dnsmonApp — DNS Propagation Checker (index.html)
// ---------------------------------------------------------------------------
function dnsmonApp() {
  return {
    // Form state
    domain: '',
    type: 'A',

    // Results
    results: [],
    loading: false,
    errorMsg: '',
    summary: null,
    checkId: '',
    shareLabel: 'Share',

    // Live / WebSocket
    liveActive: false,
    _ws: null,

    // Progress
    progressPct: 0,
    progressLabel: 'Querying resolvers...',
    _totalResolvers: 0,
    _receivedCount: 0,

    // Sorting
    sortBy: 'country',
    sortDir: 'asc',

    // Pagination
    page: 1,
    pageSize: 25,

    // Map
    _map: null,
    _markerLayer: null,

    // ---------------------------------------------------------------------------
    // Lifecycle
    // ---------------------------------------------------------------------------
    init() {
      this._initMap();
      // Restore domain/type from URL query string if present
      const params = new URLSearchParams(window.location.search);
      if (params.get('domain')) this.domain = params.get('domain');
      if (params.get('type')) this.type = params.get('type');
      // If URL is a permalink like /check/:id, load it
      const match = window.location.pathname.match(/^\/check\/([a-zA-Z0-9]+)$/);
      if (match) this._loadPermalink(match[1]);
    },

    _initMap() {
      this.$nextTick(() => {
        const el = document.getElementById('map');
        if (!el) return;
        const { map, markerLayer } = window.initMap('map');
        this._map = map;
        this._markerLayer = markerLayer;
      });
    },

    // ---------------------------------------------------------------------------
    // Check (HTTP)
    // ---------------------------------------------------------------------------
    async check() {
      if (!this.domain.trim()) return;
      this._resetResults();
      this.loading = true;
      this.errorMsg = '';

      try {
        const resp = await fetch('/api/v1/check', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name: this.domain.trim(),
            type: this.type,
            save: true,
          }),
        });

        if (!resp.ok) {
          const err = await resp.json().catch(() => ({ error: { message: resp.statusText } }));
          throw new Error(err?.error?.message || `HTTP ${resp.status}`);
        }

        const data = await resp.json();
        this.results = data.results || [];
        this.summary = data.summary || null;
        this.checkId = data.id || '';
        this.progressPct = 100;

        if (this._markerLayer) {
          window.updateMarkers(this._markerLayer, this.results);
        }
      } catch (err) {
        this.errorMsg = err.message;
      } finally {
        this.loading = false;
      }
    },

    // ---------------------------------------------------------------------------
    // Live check (WebSocket)
    // ---------------------------------------------------------------------------
    toggleLive() {
      if (this.liveActive) {
        this._stopLive();
      } else {
        this._startLive();
      }
    },

    _startLive() {
      if (!this.domain.trim()) return;
      this._resetResults();
      this.liveActive = true;
      this.loading = true;
      this.errorMsg = '';

      const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${proto}//${window.location.host}/api/v1/check/stream`;
      this._ws = new WebSocket(wsUrl);

      this._ws.onopen = () => {
        this._ws.send(JSON.stringify({ name: this.domain.trim(), type: this.type }));
      };

      this._ws.onmessage = (event) => {
        let msg;
        try { msg = JSON.parse(event.data); } catch { return; }

        if (msg.type === 'result' && msg.data) {
          this.results.push(msg.data);
          this._receivedCount++;
          if (this._totalResolvers > 0) {
            this.progressPct = Math.round((this._receivedCount / this._totalResolvers) * 100);
          }
          if (this._markerLayer) {
            window.updateMarkers(this._markerLayer, this.results);
          }
        } else if (msg.type === 'total' && msg.data) {
          this._totalResolvers = msg.data.total || 0;
          this.progressLabel = `Querying ${this._totalResolvers} resolvers...`;
        } else if (msg.type === 'done' && msg.data) {
          this.summary = msg.data.summary || null;
          this.checkId = msg.data.id || '';
          this.progressPct = 100;
          this.loading = false;
          this.liveActive = false;
          this._ws = null;
        } else if (msg.type === 'error') {
          this.errorMsg = msg.data?.message || 'Stream error';
          this._stopLive();
        }
      };

      this._ws.onerror = () => {
        this.errorMsg = 'WebSocket connection failed.';
        this._stopLive();
      };

      this._ws.onclose = () => {
        this.loading = false;
        this.liveActive = false;
        this._ws = null;
      };
    },

    _stopLive() {
      if (this._ws) {
        this._ws.close();
        this._ws = null;
      }
      this.liveActive = false;
      this.loading = false;
    },

    // ---------------------------------------------------------------------------
    // Permalink
    // ---------------------------------------------------------------------------
    async _loadPermalink(id) {
      this.loading = true;
      try {
        const resp = await fetch(`/api/v1/check/${id}`);
        if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
        const data = await resp.json();
        this.domain = data.name || '';
        this.type = data.type || 'A';
        this.results = data.results || [];
        this.summary = data.summary || null;
        this.checkId = data.id || '';
        this.progressPct = 100;
        if (this._markerLayer) {
          window.updateMarkers(this._markerLayer, this.results);
        }
      } catch (err) {
        this.errorMsg = err.message;
      } finally {
        this.loading = false;
      }
    },

    share() {
      if (!this.checkId) return;
      const url = `${window.location.origin}/check/${this.checkId}`;
      navigator.clipboard.writeText(url).then(() => {
        this.shareLabel = 'Copied!';
        setTimeout(() => { this.shareLabel = 'Share'; }, 2000);
      }).catch(() => {
        window.prompt('Copy this link:', url);
      });
    },

    // ---------------------------------------------------------------------------
    // Export
    // ---------------------------------------------------------------------------
    exportResults(format) {
      if (!this.checkId) return;
      window.exportCheck(this.checkId, format).catch((err) => {
        this.errorMsg = `Export failed: ${err.message}`;
      });
    },

    // ---------------------------------------------------------------------------
    // Sorting
    // ---------------------------------------------------------------------------
    setSortBy(field) {
      if (this.sortBy === field) {
        this.sortDir = this.sortDir === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortBy = field;
        this.sortDir = 'asc';
      }
      this.page = 1;
    },

    get sortedResults() {
      const dir = this.sortDir === 'asc' ? 1 : -1;
      const sorted = [...this.results];
      switch (this.sortBy) {
        case 'country':
          return sorted.sort((a, b) =>
            dir * (a.resolver.country || '').localeCompare(b.resolver.country || ''));
        case 'location':
          return sorted.sort((a, b) => {
            const loc = (r) => [r.resolver.country || '', r.resolver.city || ''].join(' ');
            return dir * loc(a).localeCompare(loc(b));
          });
        case 'resolver':
          return sorted.sort((a, b) =>
            dir * (a.resolver.name || '').localeCompare(b.resolver.name || ''));
        case 'status':
          return sorted.sort((a, b) => dir * a.status.localeCompare(b.status));
        case 'duration':
          return sorted.sort((a, b) => dir * ((a.duration_ms || 0) - (b.duration_ms || 0)));
        case 'answer': {
          const ansVal = (r) => (r.answers && r.answers.length > 0 ? r.answers[0].value : '');
          return sorted.sort((a, b) => dir * ansVal(a).localeCompare(ansVal(b)));
        }
        default:
          return sorted;
      }
    },

    // ---------------------------------------------------------------------------
    // Pagination
    // ---------------------------------------------------------------------------
    get totalPages() {
      if (this.pageSize === 0) return 1;
      return Math.max(1, Math.ceil(this.results.length / this.pageSize));
    },

    get pagedResults() {
      if (this.pageSize === 0) return this.sortedResults;
      const start = (this.page - 1) * this.pageSize;
      return this.sortedResults.slice(start, start + this.pageSize);
    },

    get pageRangeLabel() {
      if (this.results.length === 0) return '';
      if (this.pageSize === 0) return `1–${this.results.length} of ${this.results.length}`;
      const start = (this.page - 1) * this.pageSize + 1;
      const end = Math.min(this.page * this.pageSize, this.results.length);
      return `${start}–${end} of ${this.results.length}`;
    },

    get pageNumbers() {
      const total = this.totalPages;
      if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1);
      const p = this.page;
      const pages = new Set([1, total, p]);
      if (p > 1) pages.add(p - 1);
      if (p < total) pages.add(p + 1);
      return [...pages].sort((a, b) => a - b);
    },

    setPageSize(n) {
      this.pageSize = Number(n);
      this.page = 1;
    },

    goToPage(n) {
      this.page = Math.max(1, Math.min(n, this.totalPages));
    },

    // ---------------------------------------------------------------------------
    // Helpers exposed to templates
    // ---------------------------------------------------------------------------
    countryFlag,

    // ---------------------------------------------------------------------------
    // Internal helpers
    // ---------------------------------------------------------------------------
    _resetResults() {
      this.results = [];
      this.summary = null;
      this.checkId = '';
      this.progressPct = 0;
      this.progressLabel = 'Querying resolvers...';
      this._receivedCount = 0;
      this._totalResolvers = 0;
      this.shareLabel = 'Share';
      this.page = 1;
      if (this._markerLayer) {
        this._markerLayer.clearLayers();
      }
    },
  };
}

// ---------------------------------------------------------------------------
// lookupApp — Single-resolver detailed lookup (lookup.html)
// ---------------------------------------------------------------------------
function lookupApp() {
  return {
    domain: '',
    type: 'A',
    resolver: '1.1.1.1',
    protocol: 'udp',
    result: null,
    loading: false,
    errorMsg: '',

    async lookup() {
      if (!this.domain.trim()) return;
      this.loading = true;
      this.errorMsg = '';
      this.result = null;

      try {
        const resp = await fetch('/api/v1/lookup', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name: this.domain.trim(),
            type: this.type,
            resolver: this.resolver.trim() || '1.1.1.1',
            protocol: this.protocol,
          }),
        });

        if (!resp.ok) {
          const err = await resp.json().catch(() => ({ error: { message: resp.statusText } }));
          throw new Error(err?.error?.message || `HTTP ${resp.status}`);
        }

        this.result = await resp.json();
      } catch (err) {
        this.errorMsg = err.message;
      } finally {
        this.loading = false;
      }
    },
  };
}

// ---------------------------------------------------------------------------
// reverseApp — Reverse DNS lookup (reverse.html)
// ---------------------------------------------------------------------------
function reverseApp() {
  return {
    ip: '',
    results: [],
    loading: false,
    errorMsg: '',
    summary: null,
    progressPct: 0,
    progressLabel: 'Querying resolvers...',

    // Map
    _map: null,
    _markerLayer: null,

    init() {
      this.$nextTick(() => {
        const el = document.getElementById('map');
        if (!el) return;
        const { map, markerLayer } = window.initMap('map');
        this._map = map;
        this._markerLayer = markerLayer;
      });
    },

    async lookup() {
      if (!this.ip.trim()) return;
      this.results = [];
      this.summary = null;
      this.loading = true;
      this.errorMsg = '';
      this.progressPct = 0;

      if (this._markerLayer) this._markerLayer.clearLayers();

      try {
        const resp = await fetch('/api/v1/reverse', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ ip: this.ip.trim() }),
        });

        if (!resp.ok) {
          const err = await resp.json().catch(() => ({ error: { message: resp.statusText } }));
          throw new Error(err?.error?.message || `HTTP ${resp.status}`);
        }

        const data = await resp.json();
        this.results = data.results || [];
        this.summary = data.summary || null;
        this.progressPct = 100;

        if (this._markerLayer) {
          window.updateMarkers(this._markerLayer, this.results);
        }
      } catch (err) {
        this.errorMsg = err.message;
      } finally {
        this.loading = false;
      }
    },

    countryFlag,
  };
}

// ---------------------------------------------------------------------------
// Register Alpine.js components
// ---------------------------------------------------------------------------
document.addEventListener('alpine:init', () => {
  Alpine.data('dnsmonApp', dnsmonApp);
  Alpine.data('lookupApp', lookupApp);
  Alpine.data('reverseApp', reverseApp);
});
