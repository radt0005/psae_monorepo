// Mobile navigation toggle
(function() {
  var toggle = document.querySelector('.nav__toggle');
  var links = document.querySelector('.nav__links');

  if (toggle && links) {
    toggle.addEventListener('click', function() {
      links.classList.toggle('nav__links--open');
    });

    // Close mobile nav when clicking a link
    var navLinks = links.querySelectorAll('.nav__link');
    for (var i = 0; i < navLinks.length; i++) {
      navLinks[i].addEventListener('click', function() {
        links.classList.remove('nav__links--open');
      });
    }
  }
})();

// Language tab switcher — builds tabs from highlighted code blocks
(function() {
  var panelsContainer = document.querySelector('.code-tabs__panels');
  var buttonsContainer = document.querySelector('.code-tabs__buttons');
  if (!panelsContainer || !buttonsContainer) return;

  var langNames = {
    python: 'Python', r: 'R', typescript: 'TypeScript',
    go: 'Go', rust: 'Rust', yaml: 'YAML'
  };

  // Find all highlighted code blocks and wrap them in panel divs
  var codeBlocks = panelsContainer.querySelectorAll('pre');
  var panels = [];
  for (var i = 0; i < codeBlocks.length; i++) {
    var codeEl = codeBlocks[i].querySelector('code[data-lang]');
    if (!codeEl) continue;
    var lang = codeEl.getAttribute('data-lang');
    var panel = document.createElement('div');
    panel.className = 'code-panel' + (i === 0 ? ' active' : '');
    panel.id = 'panel-' + lang;
    codeBlocks[i].parentNode.insertBefore(panel, codeBlocks[i]);
    panel.appendChild(codeBlocks[i]);
    panels.push({ lang: lang, panel: panel });
  }

  // Create tab buttons
  for (var i = 0; i < panels.length; i++) {
    var btn = document.createElement('button');
    btn.className = 'code-tab' + (i === 0 ? ' active' : '');
    btn.setAttribute('data-tab', panels[i].lang);
    btn.textContent = langNames[panels[i].lang] || panels[i].lang;
    buttonsContainer.appendChild(btn);
  }

  // Tab click handler
  var tabs = buttonsContainer.querySelectorAll('.code-tab');
  for (var i = 0; i < tabs.length; i++) {
    tabs[i].addEventListener('click', function() {
      var target = this.getAttribute('data-tab');
      var allTabs = buttonsContainer.querySelectorAll('.code-tab');
      var allPanels = panelsContainer.querySelectorAll('.code-panel');
      for (var j = 0; j < allTabs.length; j++) allTabs[j].classList.remove('active');
      for (var j = 0; j < allPanels.length; j++) allPanels[j].classList.remove('active');
      this.classList.add('active');
      var panel = document.getElementById('panel-' + target);
      if (panel) panel.classList.add('active');
    });
  }
})();

// Scroll animations using IntersectionObserver
(function() {
  if (!('IntersectionObserver' in window)) return;

  var sections = document.querySelectorAll('.section, .card, .feature-card, .use-case-card, .step-card');

  var observer = new IntersectionObserver(function(entries) {
    for (var i = 0; i < entries.length; i++) {
      if (entries[i].isIntersecting) {
        entries[i].target.classList.add('animate-in');
        observer.unobserve(entries[i].target);
      }
    }
  }, {
    threshold: 0.1
  });

  for (var i = 0; i < sections.length; i++) {
    sections[i].classList.add('animate-ready');
    observer.observe(sections[i]);
  }
})();
