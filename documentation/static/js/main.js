// Mobile navigation toggle
(function() {
  var toggle = document.querySelector('.nav__toggle');
  var links = document.querySelector('.nav__links');
  var sidebar = document.querySelector('.docs-sidebar');
  var sidebarToggle = document.querySelector('.sidebar-toggle');

  if (toggle && links) {
    toggle.addEventListener('click', function() {
      links.classList.toggle('nav__links--open');
    });
    var navLinks = links.querySelectorAll('.nav__link');
    for (var i = 0; i < navLinks.length; i++) {
      navLinks[i].addEventListener('click', function() {
        links.classList.remove('nav__links--open');
      });
    }
  }

  if (sidebarToggle && sidebar) {
    sidebarToggle.addEventListener('click', function() {
      sidebar.classList.toggle('docs-sidebar--open');
    });
    // Close sidebar when clicking outside on mobile
    document.addEventListener('click', function(e) {
      if (sidebar.classList.contains('docs-sidebar--open') &&
          !sidebar.contains(e.target) && e.target !== sidebarToggle) {
        sidebar.classList.remove('docs-sidebar--open');
      }
    });
  }
})();

// Copy-to-clipboard for code blocks
(function() {
  var codeBlocks = document.querySelectorAll('pre');
  for (var i = 0; i < codeBlocks.length; i++) {
    var btn = document.createElement('button');
    btn.className = 'copy-btn';
    btn.textContent = 'Copy';
    btn.setAttribute('aria-label', 'Copy code');
    codeBlocks[i].style.position = 'relative';
    codeBlocks[i].appendChild(btn);
    btn.addEventListener('click', function() {
      var code = this.parentElement.querySelector('code');
      if (!code) return;
      navigator.clipboard.writeText(code.textContent).then(function() {
        var b = this;
        b.textContent = 'Copied!';
        setTimeout(function() { b.textContent = 'Copy'; }, 2000);
      }.bind(this));
    });
  }
})();

// Active sidebar link highlighting
(function() {
  var currentPath = window.location.pathname;
  var sidebarLinks = document.querySelectorAll('.sidebar-nav a');
  for (var i = 0; i < sidebarLinks.length; i++) {
    var href = sidebarLinks[i].getAttribute('href');
    if (href === currentPath || href === currentPath + '/') {
      sidebarLinks[i].classList.add('active');
      // Expand parent sections
      var parent = sidebarLinks[i].parentElement;
      while (parent) {
        if (parent.classList && parent.classList.contains('sidebar-nav__children')) {
          parent.classList.add('sidebar-nav__children--expanded');
          var toggle = parent.previousElementSibling;
          if (toggle && toggle.querySelector('.sidebar-nav__toggle')) {
            toggle.querySelector('.sidebar-nav__toggle').classList.add('sidebar-nav__toggle--expanded');
          }
        }
        parent = parent.parentElement;
      }
    }
  }
})();

// Sidebar section expand/collapse
(function() {
  var toggles = document.querySelectorAll('.sidebar-nav__toggle');
  for (var i = 0; i < toggles.length; i++) {
    toggles[i].addEventListener('click', function(e) {
      e.preventDefault();
      e.stopPropagation();
      this.classList.toggle('sidebar-nav__toggle--expanded');
      var children = this.closest('.sidebar-nav__item').querySelector('.sidebar-nav__children');
      if (children) {
        children.classList.toggle('sidebar-nav__children--expanded');
      }
    });
  }
})();

// Language tabs for multi-language code examples
(function() {
  var tabGroups = document.querySelectorAll('.lang-tabs');
  tabGroups.forEach(function(group) {
    var buttons = group.querySelectorAll('.lang-tab-btn');
    var panels = group.querySelectorAll('.lang-tab-panel');
    buttons.forEach(function(btn) {
      btn.addEventListener('click', function() {
        var target = this.getAttribute('data-lang');
        buttons.forEach(function(b) { b.classList.remove('active'); });
        panels.forEach(function(p) { p.classList.remove('active'); });
        this.classList.add('active');
        group.querySelector('.lang-tab-panel[data-lang="' + target + '"]').classList.add('active');
      });
    });
  });
})();
