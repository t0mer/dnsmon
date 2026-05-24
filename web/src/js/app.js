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
// Country name helper (ISO-3166 alpha-2 → full name). Flags render via the
// flag-icons CSS classes (fi fi-xx) in the templates, not emoji.
// ---------------------------------------------------------------------------
const _regionNames = (typeof Intl !== 'undefined' && Intl.DisplayNames)
  ? new Intl.DisplayNames(['en'], { type: 'region' })
  : null;

function countryName(code) {
  if (!code || code.length !== 2) return code || '';
  try {
    return _regionNames ? _regionNames.of(code.toUpperCase()) : code;
  } catch {
    return code;
  }
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

    // Progress
    progressPct: 0,
    progressLabel: 'Querying resolvers...',

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
    // Check (streamed over WebSocket so the progress bar reflects real progress
    // as each resolver responds).
    // ---------------------------------------------------------------------------
    check() {
      if (!this.domain.trim()) return;
      this._resetResults();
      this.loading = true;
      this.errorMsg = '';
      this.progressLabel = 'Querying resolvers…';

      const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const ws = new WebSocket(`${proto}//${window.location.host}/api/v1/check/stream`);
      let total = 0;
      let received = 0;

      ws.onopen = () => {
        ws.send(JSON.stringify({ name: this.domain.trim(), type: this.type }));
      };

      ws.onmessage = (event) => {
        let msg;
        try { msg = JSON.parse(event.data); } catch { return; }

        if (msg.type === 'total') {
          total = msg.data?.total || 0;
          this.progressLabel = `Querying ${total} resolvers…`;
        } else if (msg.type === 'result' && msg.data) {
          this.results.push(msg.data);
          received++;
          this.progressPct = total > 0 ? Math.round((received / total) * 100) : 0;
          if (this._markerLayer) {
            window.updateMarkers(this._markerLayer, this.results);
          }
        } else if (msg.type === 'done' && msg.data) {
          this.summary = msg.data.summary || null;
          this.checkId = msg.data.id || '';
          this.progressPct = 100;
        } else if (msg.type === 'error') {
          this.errorMsg = msg.data?.message || 'Check failed.';
        }
      };

      ws.onerror = () => {
        this.errorMsg = 'Connection to the server failed.';
        this.loading = false;
      };

      ws.onclose = () => {
        this.loading = false;
      };
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
      // Build the link from the address the browser is on, so it works behind a
      // reverse proxy / on any host (not just localhost).
      const url = `${window.location.origin}/check/${this.checkId}`;
      this._copyLink(url);
    },

    _copyLink(text) {
      const copied = () => {
        this.shareLabel = 'Copied!';
        setTimeout(() => { this.shareLabel = 'Share'; }, 2000);
      };
      // The async Clipboard API only exists in a secure context (HTTPS or
      // localhost); on plain-HTTP hosts it's undefined, so guard and fall back.
      if (navigator.clipboard && window.isSecureContext) {
        navigator.clipboard.writeText(text).then(copied).catch(() => this._fallbackCopy(text, copied));
      } else {
        this._fallbackCopy(text, copied);
      }
    },

    _fallbackCopy(text, copied) {
      try {
        const ta = document.createElement('textarea');
        ta.value = text;
        ta.setAttribute('readonly', '');
        ta.style.position = 'fixed';
        ta.style.top = '-1000px';
        ta.style.opacity = '0';
        document.body.appendChild(ta);
        ta.select();
        const ok = document.execCommand('copy');
        document.body.removeChild(ta);
        if (ok) { copied(); return; }
      } catch (_) { /* fall through to prompt */ }
      window.prompt('Copy this link:', text);
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

    // Hand off the current check to the Settings → Monitors tab, prefilling the
    // FQDN, record type, and the majority answer set (as expected / baseline).
    createMonitor(monitorType) {
      if (this.results.length === 0) return;
      let expected = [];
      if (this.summary && this.summary.consensus) {
        let best = '';
        let bestN = -1;
        for (const [k, n] of Object.entries(this.summary.consensus)) {
          if (n > bestN) { bestN = n; best = k; }
        }
        if (best && best !== '<empty>') {
          expected = best.split(',').map((s) => s.trim()).filter(Boolean);
        }
      }
      sessionStorage.setItem('dnsmon.newMonitor', JSON.stringify({
        type: monitorType,
        fqdn: this.domain.trim(),
        record_type: this.type,
        expected,
      }));
      window.location.href = '/settings?tab=monitors';
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
    countryName,

    // ---------------------------------------------------------------------------
    // Internal helpers
    // ---------------------------------------------------------------------------
    _resetResults() {
      this.results = [];
      this.summary = null;
      this.checkId = '';
      this.progressPct = 0;
      this.progressLabel = 'Querying resolvers...';
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

    countryName,
  };
}

// ---------------------------------------------------------------------------
// settingsApp — Settings page (settings.html)
// ---------------------------------------------------------------------------
function settingsApp() {
  return {
    activeTab: 'notifications',
    loading: true,
    saving: false,
    statusMsg: '',
    statusErr: false,

    // Settings document
    auth: { enabled: false, username: '', password: '', password_set: false },
    notifications: [],
    disabledResolvers: [],

    // Notification channel editor (add/edit)
    showEditor: false,
    editorIndex: -1, // -1 = adding a new channel
    editor: { id: '', type: 'shoutrrr', name: '', enabled: true, config: {} },

    // API tokens
    tokens: [],
    newTokenName: '',
    createdToken: '',

    // Schedules
    schedules: [],
    scheduleForm: { name: '', cadence: '@daily', cron: '', enabled: true },
    scheduleEditId: '', // '' = adding; otherwise the id being edited

    // Resolvers
    resolvers: [],
    resolverQuery: '',

    // Monitors
    monitors: [],
    monitorForm: { type: 'propagation', name: '', fqdn: '', record_type: 'A', expected: '', scheduler_id: '', channel_id: '', enabled: true },
    monitorEditId: '',
    showMonitorEditor: false,
    historyMonitorId: '',
    historyEvents: [],

    // Session
    currentUser: '',

    async init() {
      const params = new URLSearchParams(window.location.search);
      if (params.get('tab')) this.activeTab = params.get('tab');
      await this.loadAll();

      // Prefill the monitor form from a "Create monitor" handoff (home page).
      const pending = sessionStorage.getItem('dnsmon.newMonitor');
      if (pending) {
        sessionStorage.removeItem('dnsmon.newMonitor');
        try {
          const d = JSON.parse(pending);
          this.activeTab = 'monitors';
          this.openAddMonitor();
          this.monitorForm.type = d.type || 'propagation';
          this.monitorForm.fqdn = d.fqdn || '';
          this.monitorForm.name = d.fqdn || '';
          this.monitorForm.record_type = d.record_type || 'A';
          this.monitorForm.expected = (d.expected || []).join('\n');
        } catch { /* ignore malformed handoff */ }
      }
    },

    // unauthorized() returns true (and redirects to /login) if the response is a 401.
    unauthorized(resp) {
      if (resp.status === 401) {
        window.location.href = '/login';
        return true;
      }
      return false;
    },

    async loadAll() {
      this.loading = true;
      try {
        const sess = await fetch('/api/v1/auth/session').then((r) => r.json()).catch(() => ({}));
        this.currentUser = sess.authenticated ? sess.username : '';

        const responses = await Promise.all([
          fetch('/api/v1/settings'),
          fetch('/api/v1/settings/tokens'),
          fetch('/api/v1/settings/schedules'),
          fetch('/api/v1/resolvers'),
          fetch('/api/v1/settings/monitors'),
        ]);
        if (responses.some((r) => this.unauthorized(r))) return;
        const [s, tk, sc, rs, mon] = await Promise.all(responses.map((r) => r.json()));
        this.auth = {
          enabled: !!s.auth.enabled,
          username: s.auth.username || '',
          password: '',
          password_set: !!s.auth.password_set,
        };
        this.notifications = s.notifications || [];
        this.disabledResolvers = s.disabled_resolvers || [];
        this.tokens = tk || [];
        this.schedules = sc || [];
        this.resolvers = rs || [];
        this.monitors = mon || [];
      } catch (e) {
        this.flash('Failed to load settings: ' + e.message, true);
      } finally {
        this.loading = false;
      }
    },

    flash(msg, err = false) {
      this.statusMsg = msg;
      this.statusErr = err;
      setTimeout(() => { this.statusMsg = ''; }, 4000);
    },

    async logout() {
      await fetch('/api/v1/auth/logout', { method: 'POST' }).catch(() => {});
      window.location.href = '/login';
    },

    // ---- Settings document (auth + notifications + disabled resolvers) ----
    async saveSettings() {
      this.saving = true;
      try {
        const payload = {
          auth: {
            enabled: this.auth.enabled,
            username: this.auth.username,
            password: this.auth.password || '',
          },
          notifications: this.notifications,
          disabled_resolvers: this.disabledResolvers,
        };
        const resp = await fetch('/api/v1/settings', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        });
        if (this.unauthorized(resp)) return;
        if (!resp.ok) {
          const e = await resp.json().catch(() => ({}));
          throw new Error(e?.error?.message || `HTTP ${resp.status}`);
        }
        const s = await resp.json();
        // Enabling auth starts requiring a session; send the user to log in.
        if (s.auth.enabled && !this.currentUser) {
          window.location.href = '/login';
          return;
        }
        this.auth = {
          enabled: !!s.auth.enabled,
          username: s.auth.username || '',
          password: '',
          password_set: !!s.auth.password_set,
        };
        this.notifications = s.notifications || [];
        this.disabledResolvers = s.disabled_resolvers || [];
        this.flash('Settings saved.');
      } catch (e) {
        this.flash(e.message, true);
      } finally {
        this.saving = false;
      }
    },

    // ---- Notification channels ----
    channelDefaults(type) {
      if (type === 'shoutrrr') return { url: '' };
      if (type === 'greenapi') return { instance_id: '', token: '', recipient: '', api_url: '' };
      if (type === 'gowa') return { base_url: '', username: '', password: '', recipient: '' };
      return {};
    },
    channelFields(type) {
      return Object.keys(this.channelDefaults(type));
    },
    channelTypeLabel(type) {
      return { shoutrrr: 'Shoutrrr', greenapi: 'WhatsApp (GreenAPI)', gowa: 'WhatsApp (go-whatsapp-web)' }[type] || type;
    },
    fieldLabel(field) {
      return {
        url: 'Shoutrrr URL',
        instance_id: 'Instance ID',
        token: 'API token',
        base_url: 'Base URL',
        username: 'Username',
        password: 'Password',
        recipient: 'Recipient',
        api_url: 'API URL (optional)',
      }[field] || field;
    },

    // Open the editor to add a new channel.
    openAdd() {
      this.editorIndex = -1;
      this.editor = { id: '', type: 'shoutrrr', name: '', enabled: true, config: this.channelDefaults('shoutrrr') };
      this.showEditor = true;
    },
    // Open the editor pre-filled to edit an existing channel.
    openEdit(i) {
      const ch = this.notifications[i];
      this.editorIndex = i;
      this.editor = {
        id: ch.id,
        type: ch.type,
        name: ch.name,
        enabled: ch.enabled,
        config: { ...this.channelDefaults(ch.type), ...(ch.config || {}) },
      };
      this.showEditor = true;
    },
    // Reset the config fields when the selected type changes.
    onEditorTypeChange() {
      this.editor.config = this.channelDefaults(this.editor.type);
    },
    cancelEditor() {
      this.showEditor = false;
    },
    // Upsert the edited channel and persist.
    async saveEditor() {
      if (!this.editor.name.trim()) {
        this.flash('Channel name is required.', true);
        return;
      }
      const channel = {
        id: this.editor.id,
        type: this.editor.type,
        name: this.editor.name.trim(),
        enabled: this.editor.enabled,
        config: { ...this.editor.config },
      };
      if (this.editorIndex === -1) {
        this.notifications.push(channel);
      } else {
        this.notifications[this.editorIndex] = channel;
      }
      this.showEditor = false;
      await this.saveSettings();
    },
    async removeChannel(i) {
      this.notifications.splice(i, 1);
      await this.saveSettings();
    },
    // Send a test notification using the channel currently in the editor.
    async testEditor() {
      const resp = await fetch('/api/v1/settings/notifications/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(this.editor),
      });
      const e = await resp.json().catch(() => ({}));
      this.flash(e?.error?.message || `HTTP ${resp.status}`, !resp.ok);
    },

    // ---- API tokens ----
    async createToken() {
      if (!this.newTokenName.trim()) return;
      const resp = await fetch('/api/v1/settings/tokens', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: this.newTokenName.trim() }),
      });
      if (!resp.ok) {
        const e = await resp.json().catch(() => ({}));
        this.flash(e?.error?.message || 'Failed to create token', true);
        return;
      }
      const t = await resp.json();
      this.createdToken = t.token;
      this.newTokenName = '';
      this.tokens.unshift({ id: t.id, name: t.name, prefix: t.prefix, created_at: t.created_at });
    },
    async deleteToken(id) {
      const resp = await fetch('/api/v1/settings/tokens/' + id, { method: 'DELETE' });
      if (resp.ok || resp.status === 204) {
        this.tokens = this.tokens.filter((t) => t.id !== id);
        this.flash('Token revoked.');
      } else {
        this.flash('Failed to revoke token', true);
      }
    },

    // ---- Schedulers ----
    // Resolve the cron string from the cadence selector (or the custom field).
    scheduleCron() {
      const f = this.scheduleForm;
      return f.cadence === 'custom' ? f.cron.trim() : f.cadence;
    },
    // Load an existing scheduler into the form for editing.
    editSchedule(s) {
      this.scheduleEditId = s.id;
      const macros = ['@hourly', '@daily', '@weekly', '@monthly'];
      if (macros.includes(s.cron)) {
        this.scheduleForm = { name: s.name, cadence: s.cron, cron: '', enabled: s.enabled };
      } else {
        this.scheduleForm = { name: s.name, cadence: 'custom', cron: s.cron, enabled: s.enabled };
      }
    },
    cancelScheduleEdit() {
      this.scheduleEditId = '';
      this.scheduleForm = { name: '', cadence: '@daily', cron: '', enabled: true };
    },
    // Create a new scheduler or update the one being edited.
    async saveSchedule() {
      const f = this.scheduleForm;
      const cron = this.scheduleCron();
      if (!f.name.trim() || !cron) return;

      const editing = this.scheduleEditId !== '';
      const url = editing ? '/api/v1/settings/schedules/' + this.scheduleEditId : '/api/v1/settings/schedules';
      const resp = await fetch(url, {
        method: editing ? 'PUT' : 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: f.name.trim(), cron, enabled: f.enabled }),
      });
      if (!resp.ok) {
        const e = await resp.json().catch(() => ({}));
        this.flash(e?.error?.message || 'Failed to save scheduler', true);
        return;
      }
      const sc = await resp.json();
      if (editing) {
        const i = this.schedules.findIndex((x) => x.id === sc.id);
        if (i !== -1) this.schedules[i] = sc;
        this.flash('Scheduler updated.');
      } else {
        this.schedules.unshift(sc);
        this.flash('Scheduler created.');
      }
      this.cancelScheduleEdit();
    },
    async deleteSchedule(id) {
      const resp = await fetch('/api/v1/settings/schedules/' + id, { method: 'DELETE' });
      if (resp.ok || resp.status === 204) {
        this.schedules = this.schedules.filter((s) => s.id !== id);
        this.flash('Scheduler deleted.');
      } else {
        this.flash('Failed to delete scheduler', true);
      }
    },

    // ---- Monitors ----
    monitorTypeLabel(type) {
      return { propagation: 'Propagation', change: 'Change detection' }[type] || type;
    },
    schedulerName(id) {
      const s = this.schedules.find((x) => x.id === id);
      return s ? s.name : '—';
    },
    channelName(id) {
      const c = this.notifications.find((x) => x.id === id);
      return c ? c.name : '—';
    },
    openAddMonitor() {
      this.monitorEditId = '';
      this.monitorForm = { type: 'propagation', name: '', fqdn: '', record_type: 'A', expected: '', scheduler_id: '', channel_id: '', enabled: true };
      this.showMonitorEditor = true;
    },
    openEditMonitor(m) {
      this.monitorEditId = m.id;
      this.monitorForm = {
        type: m.type,
        name: m.name || '',
        fqdn: m.fqdn,
        record_type: m.record_type,
        expected: (m.expected || []).join('\n'),
        scheduler_id: m.scheduler_id || '',
        channel_id: m.channel_id || '',
        enabled: m.enabled,
      };
      this.showMonitorEditor = true;
    },
    cancelMonitorEditor() {
      this.showMonitorEditor = false;
    },
    async saveMonitor() {
      const f = this.monitorForm;
      if (!f.fqdn.trim()) { this.flash('FQDN is required.', true); return; }
      const expected = f.expected.split(/[\n,]+/).map((v) => v.trim()).filter(Boolean);
      if (f.type === 'propagation' && expected.length === 0) {
        this.flash('Propagation monitors need at least one expected value.', true);
        return;
      }
      const editing = this.monitorEditId !== '';
      const url = editing ? '/api/v1/settings/monitors/' + this.monitorEditId : '/api/v1/settings/monitors';
      const resp = await fetch(url, {
        method: editing ? 'PUT' : 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          type: f.type,
          name: f.name.trim(),
          fqdn: f.fqdn.trim(),
          record_type: f.record_type,
          expected,
          scheduler_id: f.scheduler_id,
          channel_id: f.channel_id,
          enabled: f.enabled,
        }),
      });
      if (this.unauthorized(resp)) return;
      if (!resp.ok) {
        const e = await resp.json().catch(() => ({}));
        this.flash(e?.error?.message || 'Failed to save monitor', true);
        return;
      }
      const m = await resp.json();
      if (editing) {
        const i = this.monitors.findIndex((x) => x.id === m.id);
        if (i !== -1) this.monitors[i] = m;
        this.flash('Monitor updated.');
      } else {
        this.monitors.unshift(m);
        this.flash('Monitor created.');
      }
      this.showMonitorEditor = false;
    },
    async deleteMonitor(id) {
      const resp = await fetch('/api/v1/settings/monitors/' + id, { method: 'DELETE' });
      if (resp.ok || resp.status === 204) {
        this.monitors = this.monitors.filter((m) => m.id !== id);
        this.flash('Monitor deleted.');
      } else {
        this.flash('Failed to delete monitor', true);
      }
    },
    async refreshMonitors() {
      const resp = await fetch('/api/v1/settings/monitors');
      if (resp.ok) this.monitors = await resp.json();
    },
    // Evaluate a monitor on demand and surface the result.
    async runMonitor(id) {
      const resp = await fetch('/api/v1/settings/monitors/' + id + '/run', { method: 'POST' });
      if (this.unauthorized(resp)) return;
      if (!resp.ok) {
        const e = await resp.json().catch(() => ({}));
        this.flash(e?.error?.message || 'Run failed', true);
        return;
      }
      const ev = await resp.json();
      const isAlert = ev.status === 'not_propagated' || ev.status === 'changed' || ev.status === 'error';
      this.flash(`${ev.status}: ${ev.message}${ev.notified ? ' (notified)' : ''}`, isAlert);
      await this.refreshMonitors();
      if (this.historyMonitorId === id) await this.viewHistory(id);
    },
    async viewHistory(id) {
      const resp = await fetch('/api/v1/settings/monitors/' + id + '/history');
      if (!resp.ok) { this.flash('Failed to load history', true); return; }
      this.historyEvents = await resp.json();
      this.historyMonitorId = id;
    },
    closeHistory() {
      this.historyMonitorId = '';
      this.historyEvents = [];
    },

    // ---- Resolver enable/disable ----
    isResolverEnabled(id) {
      return !this.disabledResolvers.includes(id);
    },
    toggleResolver(id) {
      if (this.disabledResolvers.includes(id)) {
        this.disabledResolvers = this.disabledResolvers.filter((x) => x !== id);
      } else {
        this.disabledResolvers.push(id);
      }
    },
    get filteredResolvers() {
      const q = this.resolverQuery.toLowerCase();
      if (!q) return this.resolvers;
      return this.resolvers.filter((r) =>
        (r.name + ' ' + r.country + ' ' + r.city + ' ' + r.ip).toLowerCase().includes(q));
    },

    countryName,
  };
}

// ---------------------------------------------------------------------------
// loginApp — Sign-in page (login.html)
// ---------------------------------------------------------------------------
function loginApp() {
  return {
    username: '',
    password: '',
    loading: false,
    errorMsg: '',

    async login() {
      if (!this.username.trim()) return;
      this.loading = true;
      this.errorMsg = '';
      try {
        const resp = await fetch('/api/v1/auth/login', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ username: this.username.trim(), password: this.password }),
        });
        if (!resp.ok) {
          const e = await resp.json().catch(() => ({}));
          throw new Error(e?.error?.message || `HTTP ${resp.status}`);
        }
        window.location.href = '/settings';
      } catch (e) {
        this.errorMsg = e.message;
      } finally {
        this.loading = false;
      }
    },
  };
}

// ---------------------------------------------------------------------------
// Register Alpine.js components
// ---------------------------------------------------------------------------
document.addEventListener('alpine:init', () => {
  Alpine.data('dnsmonApp', dnsmonApp);
  Alpine.data('lookupApp', lookupApp);
  Alpine.data('reverseApp', reverseApp);
  Alpine.data('settingsApp', settingsApp);
  Alpine.data('loginApp', loginApp);
});
