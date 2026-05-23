/**
 * map.js — Leaflet world map helpers for gdns
 * Provides initMap() and updateMarkers() for the propagation and reverse pages.
 */

/**
 * initMap initializes a Leaflet map in the given element and returns
 * { map, markerLayer } for later updates.
 *
 * @param {string} elementId - The DOM element ID to attach the map to.
 * @returns {{ map: L.Map, markerLayer: L.LayerGroup }}
 */
function initMap(elementId) {
  if (typeof L === 'undefined') {
    console.warn('Leaflet is not loaded; map will not initialize.');
    return { map: null, markerLayer: null };
  }

  const map = L.map(elementId, {
    center: [20, 0],
    zoom: 2,
    minZoom: 1,
    maxZoom: 8,
    zoomControl: true,
    scrollWheelZoom: false,
    attributionControl: true,
  });

  const isDark = () => document.documentElement.classList.contains('dark');

  const lightTiles = L.tileLayer(
    'https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png',
    {
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/attributions">CARTO</a>',
      subdomains: 'abcd',
      maxZoom: 19,
    }
  );

  const darkTiles = L.tileLayer(
    'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',
    {
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/attributions">CARTO</a>',
      subdomains: 'abcd',
      maxZoom: 19,
    }
  );

  (isDark() ? darkTiles : lightTiles).addTo(map);

  // Watch for dark mode toggle and swap tile layers
  const observer = new MutationObserver(() => {
    if (isDark()) {
      if (map.hasLayer(lightTiles)) { map.removeLayer(lightTiles); darkTiles.addTo(map); }
    } else {
      if (map.hasLayer(darkTiles)) { map.removeLayer(darkTiles); lightTiles.addTo(map); }
    }
  });
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });

  // Layer group for resolver markers (cleared & rebuilt on each check)
  const markerLayer = L.layerGroup().addTo(map);

  return { map, markerLayer };
}

/**
 * updateMarkers updates the marker layer on the map based on an array of
 * ResolverResult objects. Existing markers in the layer are preserved so
 * live streaming can incrementally add new markers.
 *
 * Marker colors:
 *   - green  : status === 'ok' and answer matches the consensus (most common answer)
 *   - orange : status === 'ok' but answer differs from consensus
 *   - red    : timeout, servfail, refused, or error
 *   - yellow : nxdomain
 *   - gray   : unknown
 *
 * @param {L.LayerGroup} markerLayer
 * @param {Array<object>} results - Array of ResolverResult objects
 */
function updateMarkers(markerLayer, results) {
  if (!markerLayer || !results) return;

  markerLayer.clearLayers();

  // Determine consensus answer (the most common non-empty answer set string)
  const consensusAnswer = _findConsensus(results);

  for (const result of results) {
    const { resolver, status, answers, duration_ms } = result;
    if (!resolver || resolver.lat == null || resolver.lng == null) continue;

    const answerStr = _answersToString(answers);
    const isConsensus = answerStr !== '' && answerStr === consensusAnswer;
    const color = statusColor(status, isConsensus);

    const marker = L.circleMarker([resolver.lat, resolver.lng], {
      radius: 7,
      fillColor: color,
      color: _darken(color),
      weight: 2,
      opacity: 1,
      fillOpacity: 0.88,
    });

    // Build popup content
    const popupContent = _buildPopup(resolver, status, answers, duration_ms);
    marker.bindPopup(popupContent, { maxWidth: 260, className: 'gdns-popup' });

    // Tooltip (shown on hover)
    marker.bindTooltip(
      `<strong>${_escapeHtml(resolver.name)}</strong><br/>${_escapeHtml(resolver.ip)}`,
      { direction: 'top', offset: [0, -8] }
    );

    marker.addTo(markerLayer);
  }
}

/**
 * statusColor returns a hex color string for a given resolver status.
 *
 * @param {string} status - "ok" | "nxdomain" | "timeout" | "servfail" | "refused" | "error"
 * @param {boolean} isConsensus - Whether this result matches the majority answer
 * @returns {string} hex color
 */
function statusColor(status, isConsensus) {
  switch (status) {
    case 'ok':
      return isConsensus ? '#22c55e' : '#f97316'; // green or orange
    case 'nxdomain':
      return '#eab308'; // yellow
    case 'timeout':
    case 'servfail':
    case 'refused':
    case 'error':
      return '#ef4444'; // red
    default:
      return '#9ca3af'; // gray
  }
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

/**
 * _findConsensus returns the most common answer string across all ok results.
 * Returns '' if there are no answers or no clear answer.
 */
function _findConsensus(results) {
  const counts = {};
  for (const r of results) {
    if (r.status === 'ok') {
      const key = _answersToString(r.answers);
      if (key) counts[key] = (counts[key] || 0) + 1;
    }
  }
  let best = '';
  let bestCount = 0;
  for (const [key, count] of Object.entries(counts)) {
    if (count > bestCount) {
      best = key;
      bestCount = count;
    }
  }
  return best;
}

/**
 * _answersToString converts an array of Answer objects to a canonical string.
 */
function _answersToString(answers) {
  if (!answers || answers.length === 0) return '';
  return answers
    .map((a) => a.value || '')
    .filter(Boolean)
    .sort()
    .join(' | ');
}

/**
 * _darken returns a slightly darkened version of a hex color for the marker border.
 */
function _darken(hex) {
  try {
    const n = parseInt(hex.slice(1), 16);
    const r = Math.max(0, ((n >> 16) & 0xff) - 40);
    const g = Math.max(0, ((n >> 8) & 0xff) - 40);
    const b = Math.max(0, (n & 0xff) - 40);
    return `rgb(${r},${g},${b})`;
  } catch {
    return hex;
  }
}

/**
 * _buildPopup creates HTML content for a marker popup.
 */
function _buildPopup(resolver, status, answers, durationMs) {
  const location = [resolver.city, resolver.country].filter(Boolean).join(', ');
  const answerHtml =
    answers && answers.length > 0
      ? answers.map((a) => `<span class="font-mono text-xs">${_escapeHtml(a.value)}</span>`).join('<br/>')
      : `<span class="text-gray-500">${_escapeHtml(status)}</span>`;

  const statusClass =
    status === 'ok' ? 'text-green-600' : status === 'nxdomain' ? 'text-yellow-600' : 'text-red-600';

  return `
    <div style="min-width:180px;font-family:sans-serif;font-size:13px;line-height:1.4;">
      <div style="font-weight:600;margin-bottom:4px;">${_escapeHtml(resolver.name)}</div>
      <div style="color:#6b7280;font-size:11px;margin-bottom:6px;">${_escapeHtml(resolver.ip)} &middot; ${_escapeHtml(location)}</div>
      <div style="margin-bottom:4px;">${answerHtml}</div>
      <div style="font-size:11px;color:#9ca3af;">
        <span class="${statusClass}">${_escapeHtml(status.toUpperCase())}</span>
        ${durationMs != null ? ` &middot; ${durationMs}ms` : ''}
      </div>
    </div>
  `;
}

/** _escapeHtml prevents XSS in popup/tooltip content. */
function _escapeHtml(str) {
  if (!str) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

window.initMap = initMap;
window.updateMarkers = updateMarkers;
window.statusColor = statusColor;
