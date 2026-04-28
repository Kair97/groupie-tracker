(function () {
    var selector = [
        ".hero-card",
        ".detail-hero",
        ".summary-stat",
        ".chart-panel",
        ".search-panel",
        ".notice-banner",
        ".artist-card",
        ".panel",
        ".empty-state",
        ".error-panel",
        ".concert-card",
        ".route-list li",
        ".stat-box"
    ].join(",");

    var cards = document.querySelectorAll(selector);
    cards.forEach(function (card) {
        card.addEventListener("pointermove", function (event) {
            var rect = card.getBoundingClientRect();
            card.style.setProperty("--spotlight-x", event.clientX - rect.left + "px");
            card.style.setProperty("--spotlight-y", event.clientY - rect.top + "px");
        });
    });
})();
