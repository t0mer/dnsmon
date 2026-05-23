async function exportCheck(checkId, format) {
  if (!checkId) throw new Error('No check ID provided');
  const url = `/api/v1/check/${encodeURIComponent(checkId)}/export?format=${encodeURIComponent(format)}`;
  const response = await fetch(url);
  if (!response.ok) {
    let message = `Export failed: HTTP ${response.status}`;
    try { const b = await response.json(); if (b?.error?.message) message = b.error.message; } catch {}
    throw new Error(message);
  }
  const blob = await response.blob();
  const objectUrl = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = objectUrl;
  a.download = `dnsmon-check-${checkId}.${format}`;
  a.style.display = 'none';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(objectUrl), 10_000);
}

window.exportCheck = exportCheck;
