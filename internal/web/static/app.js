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
    bindClip();
  }

  // A td.clip cell cuts long text off with an ellipsis. The full text is in
  // its title, but a tooltip needs a mouse, so any cell that is actually
  // truncated becomes a keyboard- and touch-operable toggle: click, Enter or
  // Space expands it to wrap in place, and again collapses it. Cells that
  // fit are left alone, so they add no tab stops.
  //
  // The cell deliberately keeps its table-cell role: the text is only cut
  // off visually, so a screen reader already reads all of it in its column,
  // and turning the cell into a button would take that away.
  //
  // Controls inside the cell (links, the inline-edit input) keep their own
  // behaviour, so a click on one of them is not a toggle.
  function isClipToggle(e, td) {
    return td.classList.contains('clipped') && !e.target.closest('a, input, button, select, textarea');
  }

  function bindClip() {
    const cells = Array.prototype.slice.call(document.querySelectorAll('td.clip'));
    if (!cells.length) return;

    function measure() {
      cells.forEach(function (td) {
        if (td.classList.contains('expanded')) return;
        const cut = td.scrollWidth > td.clientWidth;
        td.classList.toggle('clipped', cut);
        if (cut) td.tabIndex = 0;
        else td.removeAttribute('tabindex');
      });
    }

    function toggle(td) {
      if (!td.classList.toggle('expanded')) measure();
    }

    cells.forEach(function (td) {
      td.addEventListener('click', function (e) {
        if (isClipToggle(e, td)) toggle(td);
      });
      td.addEventListener('keydown', function (e) {
        if (e.target !== td || (e.key !== 'Enter' && e.key !== ' ')) return;
        e.preventDefault();
        toggle(td);
      });
    });

    // Whether a cell is cut off depends on its column's width and its own
    // content, and both move without any window resize: the web font
    // swapping in after first paint (font-display: swap), the filter hiding
    // rows, a row entering edit mode (which swaps the text for an input).
    // So re-measure when the fonts settle, whenever a clip cell changes
    // size, and after any click or typing on the page, not only on resize.
    let timer = null;
    function schedule() {
      clearTimeout(timer);
      timer = setTimeout(measure, 150);
    }
    window.addEventListener('resize', schedule);
    document.addEventListener('click', schedule);
    document.addEventListener('input', schedule);
    if (document.fonts) {
      // A font that fails to load changes nothing to measure; the resize
      // paths below still cover every later layout change.
      document.fonts.ready.then(measure).catch(function () { /* nothing to re-measure */ });
    }
    if (typeof ResizeObserver === 'function') {
      const observer = new ResizeObserver(schedule);
      cells.forEach(function (td) { observer.observe(td); });
    }
    measure();
  }

  // Click-to-sort for every table whose header cells carry data-sort:
  // "text" (natural, case-insensitive), "num", or "key" (exact code-unit
  // order, for the server-built IP sort keys, which a natural compare would
  // misread as numbers). A cell sorts by its data-v attribute when it has
  // one (timestamps, IP keys, raw counts) and by its text otherwise. Empty
  // values always sink to the bottom. The choice is remembered per table for
  // the browser session, so saving a row edit doesn't reshuffle the list.
  //
  // sortValue reads one cell's sort value: null when empty (or not a number
  // in a numeric column), so the caller can sink it.
  function sortValue(tr, col, numeric) {
    const td = tr.cells[col];
    if (!td) return null;
    const raw = (td.dataset.v === undefined ? td.textContent : td.dataset.v).trim();
    if (raw === '') return null;
    if (!numeric) return raw;
    const n = Number.parseFloat(raw);
    return Number.isNaN(n) ? null : n;
  }

  function compareExact(a, b) {
    if (a < b) return -1;
    return a > b ? 1 : 0;
  }

  function sortArrow(dir) {
    return dir > 0 ? '\u2191' : '\u2193';
  }

  // nextSortDir: an unsorted column starts in its preferred direction
  // (data-first="desc" for times), a sorted one flips.
  function nextSortDir(th) {
    const current = th.getAttribute('aria-sort');
    if (current === null) return th.dataset.first === 'desc' ? -1 : 1;
    return current === 'ascending' ? -1 : 1;
  }

  function bindSort() {
    const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });

    document.querySelectorAll('table').forEach(function (table, tableIndex) {
      const body = table.tBodies[0];
      const heads = Array.prototype.slice.call(table.querySelectorAll('thead th[data-sort]'));
      if (!body || !heads.length || body.querySelector('td.empty-row')) return;
      const storeKey = 'sort:' + location.pathname + '#' + tableIndex;

      function apply(th, dir) {
        const col = th.cellIndex;
        const numeric = th.dataset.sort === 'num';
        const exact = th.dataset.sort === 'key';
        const rows = Array.prototype.map.call(body.rows, function (tr, i) {
          return { tr: tr, v: sortValue(tr, col, numeric), i: i };
        });
        rows.sort(function (a, b) {
          if (a.v === null || b.v === null) {
            if (a.v === b.v) return a.i - b.i;
            return a.v === null ? 1 : -1;
          }
          let c;
          if (numeric) c = a.v - b.v;
          else if (exact) c = compareExact(a.v, b.v);
          else c = collator.compare(a.v, b.v);
          return c === 0 ? a.i - b.i : c * dir;
        });
        rows.forEach(function (r) { body.appendChild(r.tr); });

        heads.forEach(function (h) {
          const on = h === th;
          h.classList.toggle('sorted', on);
          if (on) h.setAttribute('aria-sort', dir > 0 ? 'ascending' : 'descending');
          else h.removeAttribute('aria-sort');
          h.querySelector('.arrow').textContent = on ? sortArrow(dir) : '';
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
          apply(th, nextSortDir(th));
        });
      });

      let saved = null;
      try { saved = sessionStorage.getItem(storeKey); } catch (e) { /* private mode */ }
      if (saved) {
        const parts = saved.split(':');
        const th = heads.find(function (h) { return h.cellIndex === Number(parts[0]); });
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
