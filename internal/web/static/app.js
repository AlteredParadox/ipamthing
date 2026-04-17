(function () {
  'use strict';

  function init() {
    bindBulk();
    bindFilter();
    bindConfirms();
    bindRowEdit();
  }

  function bindRowEdit() {
    document.querySelectorAll('button.edit-btn').forEach(function (btn) {
      btn.addEventListener('click', function () {
        const row = btn.closest('tr');
        if (!row) return;
        row.classList.add('editing');
        const input = row.querySelector('input[name="hostname_override"]');
        if (input) input.focus();
      });
    });
  }

  function bindBulk() {
    const form = document.getElementById('bulk-form');
    if (!form) return;

    const selectAll = document.getElementById('select-all');
    const countEl = document.getElementById('bulk-count');
    const actionSelect = document.getElementById('bulk-action-select');
    const valueInput = document.getElementById('bulk-value-input');
    const applyBtn = document.getElementById('bulk-apply');

    function checks() { return document.querySelectorAll('.host-check'); }
    function visibleChecks() {
      return Array.prototype.filter.call(checks(), function (c) {
        return c.closest('tr').style.display !== 'none';
      });
    }
    function updateCount() {
      if (!countEl) return;
      const n = document.querySelectorAll('.host-check:checked').length;
      countEl.textContent = n + ' selected';
    }

    if (selectAll) {
      selectAll.addEventListener('change', function (e) {
        visibleChecks().forEach(function (c) { c.checked = e.target.checked; });
        updateCount();
      });
    }
    document.addEventListener('change', function (e) {
      if (e.target.classList?.contains('host-check')) updateCount();
    });

    if (applyBtn) {
      applyBtn.addEventListener('click', function () {
        const action = actionSelect ? actionSelect.value : '';
        if (!action) { alert('Choose a bulk action'); return; }
        const n = document.querySelectorAll('.host-check:checked').length;
        if (n === 0) { alert('No hosts selected'); return; }
        let msg;
        if (action === 'delete') msg = 'Delete selected hosts?';
        else if (action === 'set_comment') msg = 'Set comment on selected hosts?';
        else msg = 'Set tags on selected hosts?';
        if (!confirm(msg + ' (' + n + ' host' + (n === 1 ? '' : 's') + ')')) return;
        const value = valueInput ? valueInput.value : '';
        document.getElementById('bulk-action').value = action;
        if (action === 'set_comment') document.getElementById('bulk-comments').value = value;
        if (action === 'set_tags') document.getElementById('bulk-tags').value = value;
        form.submit();
      });
    }
  }

  function bindFilter() {
    const input = document.getElementById('host-filter');
    const countEl = document.getElementById('filter-count');
    if (!input) return;
    input.addEventListener('input', function () {
      const q = input.value.toLowerCase().trim();
      let shown = 0, total = 0;
      document.querySelectorAll('tr.host-row').forEach(function (tr) {
        total++;
        if (!q) { tr.style.display = ''; shown++; return; }
        const match = (tr.dataset.search || '').toLowerCase().includes(q);
        tr.style.display = match ? '' : 'none';
        if (match) shown++;
      });
      if (countEl) countEl.textContent = q ? (shown + ' of ' + total) : '';
    });
  }

  function bindConfirms() {
    document.querySelectorAll('form[data-confirm]').forEach(function (f) {
      f.addEventListener('submit', function (e) {
        if (!confirm(f.dataset.confirm)) e.preventDefault();
      });
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
