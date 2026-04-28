(function () {
    var mapElement = document.getElementById("concert-map");
    if (!mapElement || typeof L === "undefined") {
        return;
    }

    var dataElement = document.getElementById("concert-map-data");
    var stops = [];
    try {
        stops = JSON.parse((dataElement && dataElement.textContent) || "[]");
    } catch (error) {
        stops = [];
    }

    if (!stops.length) {
        return;
    }

    var map = L.map(mapElement, {
        scrollWheelZoom: false
    });

    L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
        attribution: "&copy; OpenStreetMap contributors",
        maxZoom: 19
    }).addTo(map);

    var bounds = [];
    var route = [];

    stops.forEach(function (stop, index) {
        var position = [stop.latitude, stop.longitude];
        bounds.push(position);
        route.push(position);

        var dates = (stop.dates || []).map(function (date) {
            return "<li>" + escapeHTML(date) + "</li>";
        }).join("");

        L.marker(position)
            .addTo(map)
            .bindPopup(
                "<strong>" + (index + 1) + ". " + escapeHTML(stop.displayLocation) + "</strong>" +
                "<ul class=\"map-popup-dates\">" + dates + "</ul>"
            );
    });

    if (route.length > 1) {
        L.polyline(route, {
            color: "#737ee8",
            weight: 4,
            opacity: 0.88
        }).addTo(map);
    }

    if (bounds.length === 1) {
        map.setView(bounds[0], 7);
    } else {
        map.fitBounds(bounds, { padding: [28, 28] });
    }

    function escapeHTML(value) {
        return String(value)
            .replace(/&/g, "&amp;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;")
            .replace(/"/g, "&quot;")
            .replace(/'/g, "&#39;");
    }
})();
