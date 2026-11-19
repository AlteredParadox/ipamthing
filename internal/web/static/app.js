// ipamthing — a lightweight single-binary IPAM.
// Copyright (C) 2026 AlteredParadox
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License
// for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

(function () {
  'use strict';

  function init() {
    bindBulk();
    bindFilter();
    bindConfirms();
    bindRowEdit();
    bindSort();
  }

  // Click-to-sort for every table whose header cells carry data-sort:
  // "text" (natural, case-insensitive), "num", or "key" (exact code-unit
  // order, for the server-built IP sort keys, which a natural compare would
  // misread as numbers). A cell sorts by its data-v attribute when it has
  // one (timestamps, IP keys, raw counts) and by its text otherwise. Empty
  // values always sink to the bottom. The choice is remembered per table for
  // the browser session, so saving a row edit doesn't reshuffle the list.
  function bindSort() {
    const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });

    document.querySelectorAll('table').forEach(function (table, tableIndex) {
      const body = table.tBodies[0];
      const heads = Array.prototype.slice.call(table.querySelectorAll('thead th[data-sort]'));
      if (!body || !heads.length || body.querySelector('td.empty-row')) return;
      const storeKey = 'sort:' + location.pathname + '#' + tableIndex;

      function value(tr, col, numeric) {
        const td = tr.cells[col];
        if (!td) return null;
        const raw = (td.dataset.v === undefined ? td.textContent : td.dataset.v).trim();
        if (raw === '') return null;
        if (!numeric) return raw;
        const n = parseFloat(raw);
        return isNaN(n) ? null : n;
      }

      function apply(th, dir) {
        const col = th.cellIndex;
        const numeric = th.dataset.sort === 'num';
        const exact = th.dataset.sort === 'key';
        const rows = Array.prototype.map.call(body.rows, function (tr, i) {
          return { tr: tr, v: value(tr, col, numeric), i: i };
        });
        rows.sort(function (a, b) {
          if (a.v === null || b.v === null) {
            if (a.v === b.v) return a.i - b.i;
            return a.v === null ? 1 : -1;
          }
          let c;
          if (numeric) c = a.v - b.v;
          else if (exact) c = a.v < b.v ? -1 : (a.v > b.v ? 1 : 0);
          else c = collator.compare(a.v, b.v);
          return c === 0 ? a.i - b.i : c * dir;
        });
        rows.forEach(function (r) { body.appendChild(r.tr); });

        heads.forEach(function (h) {
          const on = h === th;
          h.classList.toggle('sorted', on);
          if (on) h.setAttribute('aria-sort', dir > 0 ? 'ascending' : 'descending');
          else h.removeAttribute('aria-sort');
          h.querySelector('.arrow').textContent = on ? (dir > 0 ? '\u2191' : '\u2193') : '';
        });
        try { sessionStorage.setItem(storeKey, col + ':' + dir); } catch (e) { /* private mode */ }
      }

      heads.forEach(function (th) {
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'th-sort';
        while (th.firstChild) btn.appendChild(th.firstChild);
        const arrow = document.createElement('span');
        arrow.className = 'arrow';
        arrow.setAttribute('aria-hidden', 'true');
        btn.appendChild(arrow);
        th.appendChild(btn);
        btn.addEventListener('click', function () {
          const first = th.dataset.first === 'desc' ? -1 : 1;
          const current = th.getAttribute('aria-sort');
          const dir = current === null ? first : (current === 'ascending' ? -1 : 1);
          apply(th, dir);
        });
      });

      let saved = null;
      try { saved = sessionStorage.getItem(storeKey); } catch (e) { /* private mode */ }
      if (saved) {
        const parts = saved.split(':');
        const th = heads.filter(function (h) { return h.cellIndex === Number(parts[0]); })[0];
        if (th) apply(th, parts[1] === '-1' ? -1 : 1);
      }
    });
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
