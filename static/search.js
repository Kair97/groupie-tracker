(function () {
    var input = document.getElementById("search-q");
    var list = document.getElementById("search-suggestions");
    var typeSelect = document.getElementById("search-type");
    if (!input || !list) {
        return;
    }

    var activeIndex = -1;
    var suggestions = [];
    var requestTimer = 0;

    input.addEventListener("input", function () {
        window.clearTimeout(requestTimer);
        requestTimer = window.setTimeout(fetchSuggestions, 120);
    });

    if (typeSelect) {
        typeSelect.addEventListener("change", function () {
            window.clearTimeout(requestTimer);
            requestTimer = window.setTimeout(fetchSuggestions, 80);
        });
    }

    input.addEventListener("keydown", function (event) {
        if (!suggestions.length) {
            return;
        }

        if (event.key === "ArrowDown") {
            event.preventDefault();
            setActive(Math.min(activeIndex + 1, suggestions.length - 1));
        }

        if (event.key === "ArrowUp") {
            event.preventDefault();
            setActive(Math.max(activeIndex - 1, 0));
        }

        if (event.key === "Enter" && activeIndex >= 0) {
            input.value = suggestions[activeIndex].value;
            selectSuggestionType(suggestions[activeIndex].type);
            hideSuggestions();
        }

        if (event.key === "Escape") {
            hideSuggestions();
        }
    });

    document.addEventListener("click", function (event) {
        if (!list.contains(event.target) && event.target !== input) {
            hideSuggestions();
        }
    });

    function fetchSuggestions() {
        var query = input.value.trim();
        if (!query) {
            hideSuggestions();
            return;
        }

        fetch("/suggest?q=" + encodeURIComponent(query) + "&type=" + encodeURIComponent(currentType()), {
            headers: { "Accept": "application/json" }
        })
            .then(function (response) {
                if (!response.ok) {
                    throw new Error("Suggestion request failed");
                }
                return response.json();
            })
            .then(function (items) {
                suggestions = Array.isArray(items) ? items : [];
                renderSuggestions();
            })
            .catch(function () {
                hideSuggestions();
            });
    }

    function renderSuggestions() {
        list.innerHTML = "";
        activeIndex = -1;

        if (!suggestions.length) {
            hideSuggestions();
            return;
        }

        suggestions.forEach(function (suggestion, index) {
            var button = document.createElement("button");
            button.type = "button";
            button.className = "suggestion-item";
            button.setAttribute("role", "option");
            button.setAttribute("id", "search-suggestion-" + index);
            button.innerHTML =
                "<span>" + escapeHTML(suggestion.value) + "</span>" +
                "<strong>" + escapeHTML(suggestion.type) + "</strong>";
            button.addEventListener("mousedown", function (event) {
                event.preventDefault();
                input.value = suggestion.value;
                selectSuggestionType(suggestion.type);
                hideSuggestions();
                input.form.submit();
            });
            list.appendChild(button);
        });

        list.classList.add("is-visible");
        input.setAttribute("aria-expanded", "true");
    }

    function setActive(index) {
        var items = list.querySelectorAll(".suggestion-item");
        items.forEach(function (item) {
            item.classList.remove("is-active");
        });

        activeIndex = index;
        if (items[activeIndex]) {
            items[activeIndex].classList.add("is-active");
            input.setAttribute("aria-activedescendant", items[activeIndex].id);
        }
    }

    function hideSuggestions() {
        suggestions = [];
        activeIndex = -1;
        list.innerHTML = "";
        list.classList.remove("is-visible");
        input.setAttribute("aria-expanded", "false");
        input.removeAttribute("aria-activedescendant");
    }

    function currentType() {
        return typeSelect ? typeSelect.value : "all";
    }

    function selectSuggestionType(type) {
        if (!typeSelect || currentType() !== "all") {
            return;
        }

        var value = {
            "artist/band": "artist",
            "member": "member",
            "location": "location",
            "first album date": "album",
            "creation year": "creation"
        }[type];

        if (value) {
            typeSelect.value = value;
        }
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
