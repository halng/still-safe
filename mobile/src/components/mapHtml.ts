import type { LocationPoint, Place } from "../types";

export function mapHtml(input: { location?: LocationPoint; trail: LocationPoint[]; places: Place[] }): string {
  const data = JSON.stringify(input).replace(/</g, "\\u003c");
  return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css" />
<script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"></script>
<style>
  html, body, #map { height: 100%; margin: 0; background: #fff; }
  .leaflet-control-attribution { font-size: 10px; }
  .leaflet-tooltip { border: 0; border-radius: 0; box-shadow: none; color: #171717; }
</style>
</head>
<body>
<div id="map"></div>
<script>
  const data = ${data};
  const map = L.map('map', { zoomControl: false, attributionControl: true });
  L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
    maxZoom: 19,
    attribution: '&copy; OpenStreetMap'
  }).addTo(map);
  const bounds = [];
  (data.trail || []).forEach((point) => bounds.push([point.latitude, point.longitude]));
  if (data.trail && data.trail.length > 1) {
    L.polyline(data.trail.map((point) => [point.latitude, point.longitude]), { color: '#171717', weight: 2 }).addTo(map);
  }
  (data.places || []).forEach((place) => {
    const at = [place.latitude, place.longitude];
    bounds.push(at);
    L.circleMarker(at, { radius: 5, color: '#171717', weight: 2, fillColor: '#ffffff', fillOpacity: 1 })
      .addTo(map)
      .bindTooltip(place.label, { permanent: true, direction: 'right', opacity: 1 });
  });
  if (data.location) {
    const at = [data.location.latitude, data.location.longitude];
    bounds.push(at);
    L.circle(at, {
      radius: Math.max(data.location.accuracyM || 0, 8),
      color: '#171717', weight: 1, fillColor: '#171717', fillOpacity: 0.06
    }).addTo(map);
    L.circleMarker(at, { radius: 7, color: '#171717', fillColor: '#171717', fillOpacity: 1 }).addTo(map);
  }
  if (!bounds.length) map.setView([20, 0], 1);
  else if (bounds.length === 1) map.setView(bounds[0], 15);
  else map.fitBounds(bounds, { padding: [28, 28] });
</script>
</body>
</html>`;
}
