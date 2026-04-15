(function () {
  'use strict';

  function init() {
    bindBulk();
    bindFilter();
    bindConfirms();
  }

  function bindBulk() {
    var form = document.getElementById('bulk-form');
    if (!form) return;

    var selectAll = document.getElementById('select-all');
    var countEl = document.getElementById('bulk-count');
    var actionSelect = document.getElementById('bulk-action-select');
    var valueInput = document.getElementById('bulk-value-input');
    var applyBtn = document.getElementById('bulk-apply');

    function checks() { return document.querySelectorAll('.host-check'); }
    function visibleChecks() {
      return Array.prototype.filter.call(checks(), function (c) {
        return c.closest('tr').style.display !== 'none';
      });
    }
    function updateCount() {
      if (!countEl) return;
      var n = document.querySelectorAll('.host-check:checked').length;
      countEl.textContent = n + ' selected';
    }

    if (selectAll) {
      selectAll.addEventListener('change', function (e) {
        visibleChecks().forEach(function (c) { c.checked = e.target.checked; });
        updateCount();
      });
    }
    document.addEventListener('change', function (e) {
      if (e.target.classList && e.target.classList.contains('host-check')) updateCount();
    });

    if (applyBtn) {
      applyBtn.addEventListener('click', function () {
        var action = actionSelect ? actionSelect.value : '';
        if (!action) { alert('Choose a bulk action'); return; }
        var n = document.querySelectorAll('.host-check:checked').length;
        if (n === 0) { alert('No hosts selected'); return; }
        var msg;
        if (action === 'delete') msg = 'Delete selected hosts?';
        else if (action === 'set_comment') msg = 'Set comment on selected hosts?';
        else msg = 'Set tags on selected hosts?';
        if (!confirm(msg + ' (' + n + ' host' + (n === 1 ? '' : 's') + ')')) return;
        var value = valueInput ? valueInput.value : '';
        document.getElementById('bulk-action').value = action;
        if (action === 'set_comment') document.getElementById('bulk-comments').value = value;
        if (action === 'set_tags') document.getElementById('bulk-tags').value = value;
        form.submit();
      });
    }
  }

  function bindFilter() {
    var input = document.getElementById('host-filter');
    var countEl = document.getElementById('filter-count');
    if (!input) return;
    input.addEventListener('input', function () {
      var q = input.value.toLowerCase().trim();
      var shown = 0, total = 0;
      document.querySelectorAll('tr.host-row').forEach(function (tr) {
        total++;
        if (!q) { tr.style.display = ''; shown++; return; }
        var s = (tr.getAttribute('data-search') || '').toLowerCase();
        var match = s.indexOf(q) >= 0;
        tr.style.display = match ? '' : 'none';
        if (match) shown++;
      });
      if (countEl) countEl.textContent = q ? (shown + ' of ' + total) : '';
    });
  }

  // Any form with data-confirm="message" will prompt before submitting.
  function bindConfirms() {
    document.querySelectorAll('form[data-confirm]').forEach(function (f) {
      f.addEventListener('submit', function (e) {
        if (!confirm(f.getAttribute('data-confirm'))) e.preventDefault();
      });
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
