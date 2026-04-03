(function() {
  var searchIndex = null;
  var searchInput = document.getElementById('search-input');
  var searchResults = document.getElementById('search-results');
  var searchOverlay = document.getElementById('search-overlay');
  var headerSearchInput = document.getElementById('header-search-input');
  var debounceTimer = null;

  function loadIndex(callback) {
    if (searchIndex) return callback();
    var script = document.createElement('script');
    script.src = '/elasticlunr.min.js';
    script.onload = function() {
      fetch('/search_index.en.json')
        .then(function(r) { return r.json(); })
        .then(function(data) {
          searchIndex = elasticlunr.Index.load(data);
          callback();
        })
        .catch(function() {
          // Search index not available
        });
    };
    // If elasticlunr is already loaded (embedded), skip script load
    if (typeof elasticlunr !== 'undefined') {
      fetch('/search_index.en.json')
        .then(function(r) { return r.json(); })
        .then(function(data) {
          searchIndex = elasticlunr.Index.load(data);
          callback();
        })
        .catch(function() {});
      return;
    }
    document.head.appendChild(script);
  }

  function doSearch(query) {
    if (!searchResults) return;
    if (!query || query.length < 2) {
      searchResults.innerHTML = '';
      searchResults.classList.remove('search-results--visible');
      return;
    }
    loadIndex(function() {
      if (!searchIndex) return;
      var results = searchIndex.search(query, { expand: true });
      if (results.length === 0) {
        searchResults.innerHTML = '<div class="search-result search-result--empty">No results found.</div>';
      } else {
        var html = '';
        var max = Math.min(results.length, 10);
        for (var i = 0; i < max; i++) {
          var item = results[i].doc;
          var title = item.title || 'Untitled';
          var body = item.body || '';
          var snippet = body.substring(0, 150).replace(/</g, '&lt;') + (body.length > 150 ? '...' : '');
          html += '<a class="search-result" href="' + item.id + '">';
          html += '<div class="search-result__title">' + title + '</div>';
          html += '<div class="search-result__snippet">' + snippet + '</div>';
          html += '</a>';
        }
        searchResults.innerHTML = html;
      }
      searchResults.classList.add('search-results--visible');
    });
  }

  function onInput(e) {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(function() {
      doSearch(e.target.value.trim());
    }, 300);
  }

  if (searchInput) {
    searchInput.addEventListener('input', onInput);
    searchInput.addEventListener('focus', function() {
      if (this.value.trim().length >= 2) doSearch(this.value.trim());
    });
  }

  if (headerSearchInput) {
    headerSearchInput.addEventListener('input', onInput);
    headerSearchInput.addEventListener('focus', function() {
      if (this.value.trim().length >= 2) doSearch(this.value.trim());
    });
  }

  // Close search results on outside click
  document.addEventListener('click', function(e) {
    if (searchResults && !searchResults.contains(e.target) &&
        e.target !== searchInput && e.target !== headerSearchInput) {
      searchResults.classList.remove('search-results--visible');
    }
  });

  // Keyboard shortcuts
  document.addEventListener('keydown', function(e) {
    // Cmd/Ctrl+K to focus search
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault();
      var input = headerSearchInput || searchInput;
      if (input) input.focus();
    }
    // Escape to close
    if (e.key === 'Escape' && searchResults) {
      searchResults.classList.remove('search-results--visible');
    }
  });
})();
