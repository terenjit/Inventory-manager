/* Textile POS — client logic */
(function () {
  'use strict';

  var $ = function (s, r) { return (r || document).querySelector(s); };
  var $$ = function (s, r) { return Array.prototype.slice.call((r || document).querySelectorAll(s)); };

  function rp(n) { return Math.round(Number(n) || 0).toLocaleString('id-ID'); }
  function money(n) { return 'Rp ' + rp(n); }
  function meter(n) {
    var v = Math.round((Number(n) || 0) * 100) / 100;
    return String(v).replace('.', ',');
  }
  function normalizeUnit(value) { return String(value || '').toLowerCase() === 'pcs' ? 'pcs' : 'meter'; }
  function unitName(value) { return normalizeUnit(value) === 'pcs' ? 'pcs' : 'meter'; }
  function qtyText(value, unit) { return normalizeUnit(unit) === 'pcs' ? rp(value) + ' pcs' : meter(value) + ' m'; }
  function qtyStep(unit) { return normalizeUnit(unit) === 'pcs' ? '1' : '0.1'; }
  function qtyMinimum(unit) { return normalizeUnit(unit) === 'pcs' ? 1 : 0.1; }
  function normalizeQty(value, unit) { return normalizeUnit(unit) === 'pcs' ? Math.round(value) : Math.round(value * 10) / 10; }
  function qtySummary(items) {
    var totals = { meter: 0, pcs: 0 };
    items.forEach(function (item) { totals[normalizeUnit(item.unit)] += Number(item.qty) || 0; });
    return [totals.meter ? qtyText(totals.meter, 'meter') : '', totals.pcs ? qtyText(totals.pcs, 'pcs') : ''].filter(Boolean).join(' + ') || '0';
  }
  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  function num(v) { var n = parseFloat(v); return isNaN(n) ? 0 : n; }
  function parseMoney(v) { var digits = String(v == null ? '' : v).replace(/\D/g, ''); return digits ? Number(digits) : 0; }
  function groupedMoney(v) { var digits = String(v == null ? '' : v).replace(/\D/g, ''); return digits ? Number(digits).toLocaleString('id-ID') : ''; }
  function clearOldBrowserData() {
    try {
      localStorage.removeItem('textilepos.salespeople.v1');
      localStorage.removeItem('textilepos.supplier-bills.v1');
    } catch (err) { console.warn('Data lama browser tidak dapat dibersihkan.', err); }
  }

  function printThermal(markup) {
    var root = $('#thermalPrint');
    if (!root) {
      root = document.createElement('div');
      root.id = 'thermalPrint';
      document.body.appendChild(root);
    }
    root.innerHTML = markup;
    document.body.classList.add('thermal-printing');
    window.print();
    setTimeout(function () {
      document.body.classList.remove('thermal-printing');
      root.innerHTML = '';
    }, 700);
  }

  function thermalHeader(title, subtitle) {
    return '<header class="thermal-header"><b>SUMBAWA TEXTILE</b><span>SWT</span><strong>' + esc(title) + '</strong><small>' + esc(subtitle) + '</small></header>';
  }

  function thermalPreview(title, markup) {
    openModal('<div class="thermal-preview-head"><div><small class="eyebrow">PRATINJAU CETAK · 80 MM</small><h2>' + esc(title) + '</h2></div>' +
      '<button class="dialog-x" data-close type="button" aria-label="Tutup">&times;</button></div>' +
      '<div class="thermal-preview-paper">' + markup + '</div>' +
      '<div class="form-actions thermal-preview-actions"><button class="btn btn-out" data-close type="button">Kembali</button>' +
      '<button class="btn btn-success" id="confirmThermalPrint" type="button">Cetak</button></div>', 'dialog-thermal-preview');
    $('#confirmThermalPrint').onclick = function () { printThermal(markup); };
  }

  function a4Preview(title, markup, thermalMarkup) {
    openModal('<div class="thermal-preview-head"><div><small class="eyebrow">PRATINJAU LAPORAN · A4</small><h2>' + esc(title) + '</h2></div>' +
      '<button class="dialog-x" data-close type="button" aria-label="Tutup">&times;</button></div>' +
      '<div class="a4-preview-paper">' + markup + '</div>' +
      '<div class="form-actions thermal-preview-actions"><button class="btn btn-out" data-close type="button">Tutup</button>' +
      '<button class="btn btn-success" id="confirmA4Print" type="button">Pratinjau Thermal 80 mm</button></div>', 'dialog-a4-preview');
    $('#confirmA4Print').onclick = function () { thermalPreview(title + ' · 80 mm', thermalMarkup); };
  }

  function hit(e, sel) {
    var el = e.target;
    if (!el || !el.closest) return null;
    return el.closest(sel);
  }

  /* ------------------------------------------------------------------ toast */
  function toast(msg, kind) {
    var box = $('#toasts');
    if (!box) return;
    var el = document.createElement('div');
    el.className = 'toast ' + (kind || '');
    el.textContent = msg;
    box.appendChild(el);
    setTimeout(function () {
      el.style.transition = 'opacity .25s';
      el.style.opacity = '0';
      setTimeout(function () { el.remove(); }, 260);
    }, 3000);
  }

  /* ------------------------------------------------------------------ modal */
  function openModal(html, cls) {
    var m = $('#modal'), d = $('#dialog');
    if (!m || !d) return null;
    d.className = 'dialog' + (cls ? ' ' + cls : '');
    d.innerHTML = html;
    $$('[data-money-input]', d).forEach(function (input) { input.value = groupedMoney(input.value); });
    m.hidden = false;
    return d;
  }

  function closeModal() {
    var m = $('#modal');
    if (m) m.hidden = true;
    document.body.classList.remove('print-modal');
  }

  function confirmBox(opt) {
    return new Promise(function (resolve) {
      openModal(
        '<button class="dialog-x" data-close type="button">&times;</button>' +
        '<small class="eyebrow">KONFIRMASI</small>' +
        '<h2>' + esc(opt.title) + '</h2>' +
        '<p style="color:var(--ink-2);font-size:13.5px;margin:0 0 16px">' + (opt.body || '') + '</p>' +
        '<div class="form-actions" style="margin-top:0;padding-top:0;border-top:0">' +
        '<button class="btn btn-out" data-no type="button">Batal</button>' +
        '<button class="btn ' + (opt.danger === false ? 'btn-success' : 'btn-danger') + '" data-yes type="button">' +
        esc(opt.ok || 'Hapus') + '</button></div>'
      );
      $('#dialog').querySelector('[data-yes]').onclick = function () { closeModal(); resolve(true); };
      $('#dialog').querySelector('[data-no]').onclick = function () { closeModal(); resolve(false); };
    });
  }

  function api(url, method, body) {
    return fetch(url, {
      method: method || 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: body ? JSON.stringify(body) : undefined
    }).then(function (r) {
      return r.json().catch(function () { return {}; }).then(function (j) {
        if (!r.ok) throw new Error(j.error || 'Permintaan gagal.');
        return j;
      });
    });
  }

  /* --------------------------------------------------------------- chrome */
  function chrome() {
    var body = document.body;

    var toggle = $('#navToggle'), scrim = $('#scrim');
    if (toggle) toggle.onclick = function () {
      body.classList.toggle('nav-open');
      if (scrim) scrim.hidden = !body.classList.contains('nav-open');
    };
    if (scrim) scrim.onclick = function () {
      body.classList.remove('nav-open');
      scrim.hidden = true;
    };

    var userBtn = $('#userBtn'), userMenu = $('#userMenu');
    if (userBtn && userMenu) userBtn.onclick = function (e) {
      e.stopPropagation();
      userMenu.hidden = !userMenu.hidden;
    };
    document.addEventListener('click', function (e) {
      if (userMenu && !userMenu.hidden && !userMenu.contains(e.target)) userMenu.hidden = true;
    });

    document.addEventListener('click', function (e) {
      if (hit(e, '[data-close]')) closeModal();
      if (hit(e, '[data-soon]')) toast('Fitur ini sedang disiapkan.', 'warn');
    });

    $$('.grp-head').forEach(function (head) {
      head.addEventListener('click', function () {
        var grp = head.parentNode;
        var open = grp.classList.toggle('open');
        head.setAttribute('aria-expanded', open ? 'true' : 'false');
      });
    });

    $$('.side-nested-toggle').forEach(function (toggle) {
      toggle.addEventListener('click', function () {
        var open = toggle.getAttribute('aria-expanded') !== 'true';
        toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
        var submenu = toggle.parentNode.querySelector('.side-submenu');
        if (submenu) submenu.classList.toggle('is-open', open);
      });
    });

    var modal = $('#modal');
    if (modal) modal.addEventListener('click', function (e) {
      if (e.target === modal) closeModal();
    });

    var bell = $('#bell');
    if (bell) bell.onclick = function () {
      var n = $$('#productGrid .product em.low').length;
      toast(n ? n + ' produk berstok menipis' : 'Semua stok aman', n ? 'warn' : 'ok');
    };

    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') {
        closeModal();
        if (body.classList.contains('nav-open')) {
          body.classList.remove('nav-open');
          if (scrim) scrim.hidden = true;
        }
      }
      if (e.key === '/' && !/^(INPUT|SELECT|TEXTAREA)$/.test(document.activeElement.tagName)) {
        var s = $('[id$="search"], #fq');
        if (s) { e.preventDefault(); s.focus(); s.select(); }
      }
    });
  }

  /* ---------------------------------------------------------------- kasir */
  function kasirPage() {
    var entry = $('#kasirEntry');
    if (!entry) return;
    var m = $('#modal');
    if (m) m.hidden = true;
    var isAdmin = document.body.dataset.role === 'admin';
    var prods = $$('#k-products .k-prod-item').map(function (el) {
      return { id: Number(el.dataset.id) || 0, sku: el.dataset.sku, name: el.dataset.name, price: Number(el.dataset.price), stock: Number(el.dataset.stock), unit: normalizeUnit(el.dataset.unit) };
    });
    var cart = [], discMode = 'percent', payMethod = 'Cash';

    if (isAdmin) {
      api('/stores', 'GET').then(function (data) {
        var sel = $('#k-store');
        if (sel && Array.isArray(data)) {
          data.forEach(function (s) { var o = document.createElement('option'); o.value = s.ID; o.textContent = s.Name; sel.appendChild(o); });
        }
        updateChange();
      }).catch(function () {});
    }
    api('/salespeople', 'GET').then(function (data) {
      var sel = $('#k-sales');
      if (sel && Array.isArray(data)) {
        data.filter(function (s) { return s.is_active; }).forEach(function (s) {
          var o = document.createElement('option'); o.value = s.id; o.textContent = s.name; sel.appendChild(o);
        });
      }
    }).catch(function () {});

    function findProd(val) {
      var v = String(val || '').trim().toLowerCase();
      if (!v) return null;
      var exact = prods.filter(function (p) { return v === p.sku.toLowerCase() || v === p.name.toLowerCase() || v === (p.sku + ' — ' + p.name).toLowerCase(); })[0];
      if (exact) return exact;
      return prods.filter(function (p) { return p.sku.toLowerCase().indexOf(v) !== -1 || p.name.toLowerCase().indexOf(v) !== -1; })[0];
    }
    function calcT() {
      var sub = cart.reduce(function (a, i) { return a + i.qty * i.price; }, 0);
      var pct = Math.max(0, Math.min(100, num($('#k-disc-pct').value)));
      var rpv = parseMoney($('#k-disc-rp').value);
      var disc = discMode === 'amount' ? Math.min(rpv, sub) : Math.round(sub * pct / 100);
      return { sub: sub, disc: disc, total: Math.max(0, sub - disc) };
    }
    function updateChange() {
      var noStore = isAdmin && (!$('#k-store') || !$('#k-store').value);
      $('#k-save').disabled = false;
    }
    function renderSummary() {
      var t = calcT();
      $('#k-total').textContent = money(t.total);
      $('#k-sub').textContent = money(t.sub);
      $('#k-disc-val').textContent = '− ' + money(t.disc);
      $('#k-sum-total').textContent = money(t.total);
      updateChange();
    }
    function renderCart() {
      try {
      var countM = 0, countPcs = 0;
      $('#k-rows').innerHTML = cart.map(function (i, ix) {
        if (i.unit === 'meter') countM += i.qty; else countPcs += i.qty;
        return '<tr><td>' + (ix + 1) + '</td><td class="mono strong">' + esc(i.sku) + '</td><td>' + esc(i.name) + '</td>' +
          '<td class="right">' + qtyText(i.qty, i.unit) + '</td><td>' + unitName(i.unit) + '</td>' +
          '<td class="right mono">' + money(i.price) + '</td><td class="right mono strong">' + money(i.qty * i.price) + '</td>' +
          '<td><button type="button" data-del="' + ix + '" style="background:none;border:none;cursor:pointer;color:var(--danger)">✕</button></td></tr>';
      }).join('');
      $('#k-empty').hidden = cart.length > 0;
      var parts = [];
      if (countM > 0) parts.push(countM.toFixed(1) + ' m');
      if (countPcs > 0) parts.push(countPcs + ' pcs');
      $('#k-count').textContent = cart.length + ' item' + (parts.length ? ' · ' + parts.join(', ') : '');
      $('#k-clear').disabled = cart.length === 0;
      renderSummary();
      } catch (err) { console.error(err); toast('RenderCart error: ' + (err && err.message ? err.message : err), 'err'); }
    }

    $('#k-prod').addEventListener('input', function () {
      var p = findProd(this.value);
      if (p) { $('#k-qty').step = qtyStep(p.unit); $('#k-qty').min = String(qtyMinimum(p.unit)); $('#k-qty-label').textContent = 'Jumlah (' + unitName(p.unit) + ')'; $('#k-hint').textContent = p.name + ' · ' + money(p.price) + '/' + unitName(p.unit) + ' · ' + qtyText(p.stock, p.unit) + ' tersedia'; }
      else { $('#k-qty').step = '0.1'; $('#k-qty').min = '0.1'; $('#k-qty-label').textContent = 'Jumlah'; $('#k-hint').textContent = 'Pilih item untuk melihat harga dan stok yang tersedia.'; }
    });
    $('#k-add').onclick = function () {
      try {
      var p = findProd($('#k-prod').value);
      if (!p) { toast('Pilih item dari daftar.', 'warn'); return; }
      var qty = normalizeQty(num($('#k-qty').value), p.unit);
      if (qty < qtyMinimum(p.unit)) { toast('Jumlah minimal ' + qtyMinimum(p.unit) + ' ' + unitName(p.unit) + '.', 'warn'); return; }
      var inCart = cart.filter(function (i) { return i.id === p.id; })[0];
      if ((inCart ? inCart.qty : 0) + qty > p.stock) { toast('Stok ' + p.name + ' hanya ' + qtyText(p.stock, p.unit) + '.', 'warn'); return; }
      if (inCart) inCart.qty = normalizeQty(inCart.qty + qty, p.unit);
      else cart.push({ id: p.id, sku: p.sku, name: p.name, price: p.price, qty: qty, stock: p.stock, unit: p.unit });
      $('#k-prod').value = ''; $('#k-qty').value = '1.0'; $('#k-qty-label').textContent = 'Jumlah'; $('#k-hint').textContent = 'Pilih item untuk melihat harga dan stok yang tersedia.';
      renderCart();
      } catch (err) { console.error(err); toast('Add error: ' + (err && err.message ? err.message : err), 'err'); }
    };
    $('#k-rows').addEventListener('click', function (e) { var b = e.target.closest('[data-del]'); if (b) { cart.splice(Number(b.dataset.del), 1); renderCart(); } });
    $('#k-clear').onclick = function () { cart = []; renderCart(); };
    if (isAdmin && $('#k-store')) $('#k-store').addEventListener('change', updateChange);
    updateChange();
    $('#k-disc-pct').addEventListener('input', function () {
      discMode = 'percent';
      var sub = cart.reduce(function (a, i) { return a + i.qty * i.price; }, 0);
      var pct = Math.max(0, Math.min(100, num($('#k-disc-pct').value)));
      if (sub > 0) {
        var disc = Math.round(sub * pct / 100);
        var rpEl = $('#k-disc-rp');
        if (rpEl) { rpEl.value = groupedMoney(disc); }
      } else {
        var rpEl2 = $('#k-disc-rp');
        if (rpEl2) rpEl2.value = '0';
      }
      renderSummary();
    });
    $('#k-disc-rp').addEventListener('input', function () {
      discMode = 'amount';
      var sub = cart.reduce(function (a, i) { return a + i.qty * i.price; }, 0);
      var rpv = parseMoney($('#k-disc-rp').value);
      if (sub > 0) {
        var pctCalc = rpv <= 0 ? 0 : Math.min(100, Math.round((rpv / sub) * 1000) / 10);
        var pctEl = $('#k-disc-pct');
        if (pctEl) pctEl.value = pctCalc;
      } else {
        var pctEl2 = $('#k-disc-pct');
        if (pctEl2) pctEl2.value = 0;
      }
      renderSummary();
    });
    $('#k-save').onclick = function () {
      var t = calcT();
      var paid = parseMoney($('#k-paid').value) || t.total;
      var isDP = payMethod === 'DP';
      var customer = ($('#k-cust').value || '').trim();
      var salesId = $('#k-sales') && $('#k-sales').value ? Number($('#k-sales').value) : null;
      var storeId = isAdmin && $('#k-store') && $('#k-store').value ? Number($('#k-store').value) : null;
      var discPct = discMode === 'percent' ? num($('#k-disc-pct').value) : 0;
      var discRp = discMode === 'amount' ? t.disc : 0;
      var cartSnapshot = cart.map(function (i) { return { product_id: i.id, qty: i.qty }; });

      var payHtml = '<div style="margin-top:12px"><div class="methods" id="pm"><button class="is-active" data-pay="Cash" type="button">Cash</button><button data-pay="Transfer" type="button">Transfer</button><button data-pay="DP" type="button">DP</button></div><label class="field" style="margin-top:10px"><span>Nominal Bayar</span><input id="pm-paid" type="text" inputmode="numeric" data-money-input value="' + groupedMoney(t.total) + '" autocomplete="off"></label><div class="change" id="pm-change"><span id="pm-change-lbl">Kembalian</span><b class="mono" id="pm-change-val">Rp 0</b></div></div>';
      try {
      var d = openModal(
        '<button class="dialog-x" data-close type="button">&times;</button>' +
        '<small class="eyebrow">KONFIRMASI</small>' +
        '<h2>Konfirmasi Transaksi</h2>' +
        '<p style="color:var(--ink-2);font-size:13.5px;margin:0 0 16px">' + cart.length + ' item · Total <b>' + money(t.total) + '</b>' + (t.disc > 0 ? ' (diskon ' + money(t.disc) + ')' : '') + '</p>' +
        payHtml +
        '<div class="form-actions" style="margin-top:16px;padding-top:12px;border-top:1px solid var(--line-2)">' +
        '<button class="btn btn-out" data-cancel type="button">Batal</button>' +
        '<button class="btn btn-success" data-confirm type="button">Lanjut ke Pembayaran</button></div>'
      );
      if (!d) return;
      // wire payment method buttons
      var pmBtns = d.querySelectorAll('#pm button');
      pmBtns.forEach(function (b) {
        b.addEventListener('click', function () {
          pmBtns.forEach(function (x) { x.classList.remove('is-active'); });
          b.classList.add('is-active');
          var inp = d.querySelector('#pm-paid');
          if (!inp) return;
          if (b.dataset.pay === 'DP') inp.value = '';
          else inp.value = groupedMoney(t.total);
          upd();
        });
      });
      var pmInput = d.querySelector('#pm-paid');
      if (pmInput) pmInput.addEventListener('input', upd);
      function upd() {
        var p = parseMoney(d.querySelector('#pm-paid').value);
        var isDp = (d.querySelector('#pm button.is-active') || {}).dataset && d.querySelector('#pm button.is-active').dataset.pay === 'DP';
        var mc = d.querySelector('#pm-change');
        var mcl = d.querySelector('#pm-change-lbl');
        var mcv = d.querySelector('#pm-change-val');
        if (isDp) {
          if (mc) mc.classList.toggle('short', p === null || p === undefined || isNaN(p) || p <= 0 || p > t.total);
          if (mcl) mcl.textContent = 'Sisa setelah DP';
          if (mcv) mcv.textContent = money(Math.max(0, t.total - (p || 0)));
        } else {
          var pp = p === null || p === undefined || isNaN(p) ? t.total : p;
          if (mc) mc.classList.toggle('short', pp < t.total);
          if (mcl) mcl.textContent = pp >= t.total ? 'Kembalian' : 'Kurang bayar';
          if (mcv) mcv.textContent = money(Math.max(0, pp - t.total));
        }
      }
      upd();
      d.querySelector('[data-cancel]').onclick = function () { closeModal(); };
      d.querySelector('[data-confirm]').onclick = function () {
        var pm = 'Cash';
        var btns = d.querySelectorAll('#pm button');
        btns.forEach(function (b) { if (b.classList.contains('is-active')) pm = b.dataset.pay; });
        var paidM = pmInput ? parseMoney(pmInput.value) : t.total;
        if (pm === 'DP') {
          if (paidM === null || paidM === undefined || isNaN(paidM) || paidM <= 0 || paidM > t.total) { toast('Nominal DP tidak valid.', 'warn'); return; }
        } else {
          if (paidM === null || paidM === undefined || isNaN(paidM)) paidM = t.total;
          if (paidM < t.total) { toast('Nominal bayar kurang.', 'warn'); return; }
        }
        var noStore = isAdmin && (!$('#k-store') || !$('#k-store').value);
        if (noStore) { toast('Pilih toko terlebih dahulu.', 'warn'); return; }
        closeModal();
        // proceed
        $('#k-save').disabled = true;
        var salesName = ($('#k-sales') && $('#k-sales').selectedIndex > 0) ? $('#k-sales').options[$('#k-sales').selectedIndex].textContent : '—';
        var storeName = (isAdmin && $('#k-store') && $('#k-store').selectedIndex > 0) ? $('#k-store').options[$('#k-store').selectedIndex].textContent : (document.body.dataset.dept || '');
        var cashierName = document.body.dataset.cashier || 'Kasir';
        var cartForReceipt = cart.slice();
        var noteVal = $('#k-note') ? $('#k-note').value.trim() : '';
        api('/api/sales', 'POST', {
          customer: customer, sales_id: salesId, store_id: storeId,
          payment: pm, paid: paidM,
          discountPercent: discPct, discountAmount: discRp,
          note: noteVal, items: cartSnapshot
        }).then(function (res) {
          var change = Number(res.change) || 0;
          var isPending = !!res.pending;
          var statusHtml = isPending
            ? '<div class="struk-status struk-pending">Pending · Sisa <b>' + money(Number(res.balance)) + '</b></div>'
            : '<div class="struk-status struk-paid">Lunas · Kembalian <b>' + money(change) + '</b></div>';
          var msgText = isPending ? 'Bill disimpan sebagai pending pembayaran.' : 'Pembayaran diterima dan bill ditutup.';
          function buildReceipt() {
            var now = new Date();
            var itemsHtml = cartForReceipt.map(function (i) {
              return '<div class="thermal-item">' +
                '<b>' + esc(i.name) + '</b>' +
                '<div class="thermal-pair"><span>' + qtyText(i.qty, i.unit) + ' × ' + money(i.price) + '</span><b>' + money(i.qty * i.price) + '</b></div>' +
                '</div>';
            }).join('');
            return '<section class="thermal-sheet">' +
              thermalHeader('RESI PENJUALAN', now.toLocaleString('id-ID')) +
              '<div class="thermal-receipt-meta">' +
              '<div><span>No. transaksi</span><b>' + esc(res.no) + '</b></div>' +
              '<div><span>Pelanggan</span><b>' + esc(customer || 'Pelanggan umum') + '</b></div>' +
              '<div><span>Sales</span><b>' + esc(salesName) + '</b></div>' +
              '<div><span>Kasir</span><b>' + esc(cashierName) + '</b></div>' +
              '</div><hr>' +
              '<div class="thermal-items-list">' + itemsHtml + '</div><hr>' +
              (Number(res.discountAmount) > 0 ? '<div class="thermal-pair"><span>Subtotal</span><span>' + money(Number(res.subtotal)) + '</span></div><div class="thermal-pair"><span>Diskon</span><span>− ' + money(Number(res.discountAmount)) + '</span></div>' : '') +
              '<div class="thermal-pair" style="font-weight:700"><span>TOTAL</span><b>' + money(Number(res.total)) + '</b></div>' +
              '<div class="thermal-pair"><span>Metode</span><span>' + esc(pm) + '</span></div>' +
              '<div class="thermal-pair"><span>Dibayar</span><span>' + money(paidM) + '</span></div>' +
              (isPending ? '<div class="thermal-pair"><b>Pending · Sisa</b><b>' + money(Number(res.balance)) + '</b></div>' : '<div class="thermal-pair"><span>Kembalian</span><span>' + money(change) + '</span></div>') +
              '<footer class="thermal-footer">Terima kasih telah berbelanja.<br>Barang yang sudah dibeli tidak dapat dikembalikan.</footer></section>';
          }
          openModal(
            '<button class="dialog-x" data-close type="button">&times;</button>' +
            '<small class="eyebrow">STRUK</small><h2>' + esc(res.no) + '</h2>' +
            '<p class="struk-total">' + money(Number(res.total)) + '</p>' +
            statusHtml +
            '<p class="muted" style="margin:10px 0 16px">' + esc(msgText) + '</p>' +
            '<div class="form-actions">' +
            '<button class="btn btn-out" id="strukTutup" type="button">Tutup</button>' +
            '<button class="btn btn-success" id="strukResi" type="button">Pratinjau Resi</button>' +
            '<button class="btn btn-out" id="strukPenjualan" type="button">Lihat Riwayat Penjualan</button>' +
            '</div>'
          );
          $('#strukTutup').onclick = function () { closeModal(); location.reload(); };
          $('#strukPenjualan').onclick = function () { location.href = '/penjualan'; };
          $('#strukResi').onclick = function () { thermalPreview('Resi penjualan', buildReceipt()); };
        }).catch(function (err) {
          toast(err.message, 'err');
          $('#k-save').disabled = false;
        });
      };
      } catch (err) {
        console.error(err);
        toast('KSave error: ' + (err && err.message ? err.message : err), 'err');
      }
      return;
      if (pm === 'DP') {
          if (paidM === null || paidM === undefined || isNaN(paidM) || paidM <= 0 || paidM > t.total) { toast('Nominal DP tidak valid.', 'warn'); return; }
        } else {
          if (paidM === null || paidM === undefined || isNaN(paidM)) paidM = t.total;
          if (paidM < t.total) { toast('Nominal bayar kurang.', 'warn'); return; }
        }
        var noStore = isAdmin && (!$('#k-store') || !$('#k-store').value);
        if (noStore) { toast('Pilih toko terlebih dahulu.', 'warn'); return; }
        $('#k-save').disabled = true;
        var salesName = ($('#k-sales') && $('#k-sales').selectedIndex > 0) ? $('#k-sales').options[$('#k-sales').selectedIndex].textContent : '—';
        var storeName = (isAdmin && $('#k-store') && $('#k-store').selectedIndex > 0) ? $('#k-store').options[$('#k-store').selectedIndex].textContent : (document.body.dataset.dept || '');
        var cashierName = document.body.dataset.cashier || 'Kasir';
        var cartForReceipt = cart.slice();
        var noteVal = $('#k-note') ? $('#k-note').value.trim() : '';
        api('/api/sales', 'POST', {
          customer: customer, sales_id: salesId, store_id: storeId,
          payment: pm, paid: paidM,
          discountPercent: discPct, discountAmount: discRp,
          note: noteVal, items: cartSnapshot
        }).then(function (res) {
          var change = Number(res.change) || 0;
          var isPending = !!res.pending;
          var statusHtml = isPending
            ? '<div class="struk-status struk-pending">Pending · Sisa <b>' + money(Number(res.balance)) + '</b></div>'
            : '<div class="struk-status struk-paid">Lunas · Kembalian <b>' + money(change) + '</b></div>';
          var msgText = isPending ? 'Bill disimpan sebagai pending pembayaran.' : 'Pembayaran diterima dan bill ditutup.';
          function buildReceipt() {
            var now = new Date();
            var itemsHtml = cartForReceipt.map(function (i) {
              return '<div class="thermal-item">' +
                '<b>' + esc(i.name) + '</b>' +
                '<div class="thermal-pair"><span>' + qtyText(i.qty, i.unit) + ' × ' + money(i.price) + '</span><b>' + money(i.qty * i.price) + '</b></div>' +
                '</div>';
            }).join('');
            return '<section class="thermal-sheet">' +
              thermalHeader('RESI PENJUALAN', now.toLocaleString('id-ID')) +
              '<div class="thermal-receipt-meta">' +
              '<div><span>No. transaksi</span><b>' + esc(res.no) + '</b></div>' +
              '<div><span>Pelanggan</span><b>' + esc(customer || 'Pelanggan umum') + '</b></div>' +
              '<div><span>Sales</span><b>' + esc(salesName) + '</b></div>' +
              '<div><span>Kasir</span><b>' + esc(cashierName) + '</b></div>' +
              '</div><hr>' +
              '<div class="thermal-items-list">' + itemsHtml + '</div><hr>' +
              (Number(res.discountAmount) > 0 ? '<div class="thermal-pair"><span>Subtotal</span><span>' + money(Number(res.subtotal)) + '</span></div><div class="thermal-pair"><span>Diskon</span><span>− ' + money(Number(res.discountAmount)) + '</span></div>' : '') +
              '<div class="thermal-pair" style="font-weight:700"><span>TOTAL</span><b>' + money(Number(res.total)) + '</b></div>' +
              '<div class="thermal-pair"><span>Metode</span><span>' + esc(pm) + '</span></div>' +
              '<div class="thermal-pair"><span>Dibayar</span><span>' + money(paidM) + '</span></div>' +
              (isPending ? '<div class="thermal-pair"><b>Pending · Sisa</b><b>' + money(Number(res.balance)) + '</b></div>' : '<div class="thermal-pair"><span>Kembalian</span><span>' + money(change) + '</span></div>') +
              '<footer class="thermal-footer">Terima kasih telah berbelanja.<br>Barang yang sudah dibeli tidak dapat dikembalikan.</footer></section>';
          }
          openModal(
            '<button class="dialog-x" data-close type="button">&times;</button>' +
            '<small class="eyebrow">STRUK</small><h2>' + esc(res.no) + '</h2>' +
            '<p class="struk-total">' + money(Number(res.total)) + '</p>' +
            statusHtml +
            '<p class="muted" style="margin:10px 0 16px">' + esc(msgText) + '</p>' +
            '<div class="form-actions">' +
            '<button class="btn btn-out" id="strukTutup" type="button">Tutup</button>' +
            '<button class="btn btn-success" id="strukResi" type="button">Pratinjau Resi</button>' +
            '<button class="btn btn-out" id="strukPenjualan" type="button">Lihat Riwayat Penjualan</button>' +
            '</div>'
          );
          $('#strukTutup').onclick = function () { closeModal(); location.reload(); };
          $('#strukPenjualan').onclick = function () { location.href = '/penjualan'; };
          $('#strukResi').onclick = function () { thermalPreview('Resi penjualan', buildReceipt()); };
        }).catch(function (err) {
          toast(err.message, 'err');
          $('#k-save').disabled = false;
        });
    };
    updateChange();
    renderCart();
    if ($('#k-save')) $('#k-save').disabled = false;
  }

  /* -------------------------------------------------------------- penjualan */
  function penjualanPage() {
    var table = $('#saleTable');
    if (!table) return;
    var tbody = $('#saleRows');

    function todayISO() {
      var d = new Date();
      return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0');
    }
    var today = todayISO();
    if ($('#fdari')) $('#fdari').value = today;
    if ($('#fsampai')) $('#fsampai').value = today;

    var state = { q: '', dept: '', dari: today, sampai: today, urut: 'date', dir: -1, page: 1, per: 10 };
    var allRows = [];
    var selectedNo = null;

    api('/stores', 'GET').then(function (data) {
      var sel = $('#fdept');
      if (sel && Array.isArray(data)) {
        data.forEach(function (s) {
          var o = document.createElement('option');
          o.value = s.Name; o.textContent = s.Name;
          sel.appendChild(o);
        });
      }
    }).catch(function () {});

    function dtStr(ts) {
      var d = new Date(ts);
      return d.toLocaleDateString('id-ID') + ' ' +
        String(d.getHours()).padStart(2, '0') + ':' +
        String(d.getMinutes()).padStart(2, '0') + ':' +
        String(d.getSeconds()).padStart(2, '0');
    }

    function buildRow(d) {
      var statusChip = d.status !== 'paid'
        ? '<span class="chip chip-low">Pending pembayaran</span>'
        : '<span class="chip chip-ok">Lunas · Bill ditutup</span>';
      var count = d.items.length;
      var qs = qtySummary(d.items.map(function (it) { return { qty: Number(it.qty), unit: it.unit }; }));
      var itemsHtml = d.items.map(function (it) {
        return '<tr><td class="mono">' + esc(it.product_code) + '</td><td>' + esc(it.product_name) + '</td>' +
          '<td class="right">' + qtyText(Number(it.qty), it.unit) + '</td>' +
          '<td class="right">' + money(Number(it.unit_price)) + '</td>' +
          '<td class="right">' + money(Number(it.line_total)) + '</td></tr>';
      }).join('');
      var selClass = d.no === selectedNo ? ' is-sel' : '';
      return '<tr class="grow' + selClass + '" data-no="' + esc(d.no) + '">' +
        '<td class="c-pick"><span class="tick"></span></td>' +
        '<td class="mono strong">' + esc(d.no) + '</td>' +
        '<td class="mono">' + esc(dtStr(d.ts)) + '</td>' +
        '<td>' + esc(d.dept) + '</td>' +
        '<td class="strong">' + esc(d.name) + '</td>' +
        '<td>' + (d.sales ? esc(d.sales) : '<span class="muted">UMUM</span>') + '</td>' +
        '<td class="right mono strong">' + money(d.total) + '</td>' +
        '<td class="right mono">' + money(d.paid) + '</td>' +
        '<td class="right mono">' + money(d.balance) + '</td>' +
        '<td>' + statusChip + '</td>' +
        '<td><details class="sale-detail"><summary>' + count + ' item \xb7 ' + esc(qs) + '</summary>' +
          '<div class="sale-detail-panel"><table><thead><tr><th>Kode</th><th>Nama barang</th>' +
          '<th class="right">Jumlah</th><th class="right">Harga/satuan</th><th class="right">Subtotal</th></tr></thead>' +
          '<tbody>' + (itemsHtml || '<tr><td colspan="5" class="muted">Tidak ada detail.</td></tr>') + '</tbody>' +
          '</table></div></details></td>' +
        '<td class="ellip">' + (d.note ? esc(d.note) : '<span class="muted">—</span>') + '</td>' +
        '<td>' + esc(d.cashier) + '</td>' +
        '</tr>';
    }

    function filtered() {
      return allRows.filter(function (d) {
        if (state.q && d.hay.indexOf(state.q.toLowerCase()) === -1) return false;
        if (state.dept && d.dept !== state.dept) return false;
        return true;
      });
    }

    function render(resetPage) {
      if (resetPage !== false) state.page = 1;
      var list = filtered();
      var keyMap = { date: 'ts', no: 'no', name: 'name', sales: 'sales', total: 'total' };
      var key = keyMap[state.urut] || 'ts';
      list.sort(function (a, b) {
        var x = a[key], y = b[key];
        if (x === y) return 0;
        return (typeof x === 'number' ? (x - y) : String(x).localeCompare(String(y))) * state.dir;
      });
      var pages = Math.max(1, Math.ceil(list.length / state.per));
      if (state.page > pages) state.page = pages;
      var shown = list.slice((state.page - 1) * state.per, state.page * state.per);
      tbody.innerHTML = shown.map(buildRow).join('');
      var sum = list.reduce(function (s, d) { return s + d.total; }, 0);
      $('#nodata').hidden = list.length !== 0;
      $('#countInfo').textContent = 'Menampilkan ' + shown.length + ' dari ' + list.length + ' transaksi';
      $('#sumTotal').textContent = money(sum);
      $('#curPage').textContent = state.page;
      $('#totPage').textContent = pages;
      $$('.totPage').forEach(function (el) { el.textContent = pages; });
      $$('.curPage').forEach(function (el) { el.textContent = state.page; });
      $('#fdir span').textContent = state.dir === -1 ? 'Desc' : 'Asc';
      updateButtons();
      return list;
    }

    function getSelected() {
      if (!selectedNo) return null;
      for (var i = 0; i < allRows.length; i++) { if (allRows[i].no === selectedNo) return allRows[i]; }
      return null;
    }

    function updateButtons() {
      var d = getSelected();
      $('#btnEdit').disabled = !d;
      var delBtn = $('#btnDelete');
      if (delBtn) delBtn.disabled = !d;
      $('#btnContinue').disabled = !d || d.status === 'paid';
      $$('tr.grow', tbody).forEach(function (tr) {
        tr.classList.toggle('is-sel', tr.dataset.no === selectedNo);
      });
    }

    function select(no) {
      selectedNo = no;
      updateButtons();
    }

    tbody.addEventListener('click', function (e) {
      var r = e.target.closest('tr.grow');
      if (r) select(r.dataset.no === selectedNo ? null : r.dataset.no);
    });

    function fetchAndRender() {
      var params = 'limit=500';
      if (state.dari) params += '&date_from=' + encodeURIComponent(state.dari);
      if (state.sampai) params += '&date_to=' + encodeURIComponent(state.sampai);
      if (state.q) params += '&q=' + encodeURIComponent(state.q);
      api('/api/sales?' + params, 'GET').then(function (result) {
        var data = (Array.isArray(result) ? result : (result.data || [])).filter(function (o) { return o.status !== 'void'; });
        allRows = data.map(function (o) {
          var bal = Math.max(0, Number(o.total) - Number(o.paid_amount));
          return {
            no: o.order_no,
            ts: new Date(o.created_at).getTime(),
            dept: o.store_name || '',
            name: o.customer_name || '',
            sales: o.sales_name || '',
            cashier: o.cashier_name || '',
            note: o.note || '',
            subtotal: Number(o.subtotal),
            discountPercent: Number(o.discount_pct),
            discountAmount: Number(o.discount_amount),
            total: Number(o.total),
            paid: Number(o.paid_amount),
            balance: bal,
            status: o.status,
            items: o.items || [],
            storeId: o.store_id,
            hay: [o.order_no, o.customer_name || '', o.cashier_name || '', o.sales_name || '', o.note || ''].join(' ').toLowerCase()
          };
        });
        render();
      }).catch(function (err) { toast(err.message || 'Gagal memuat data.', 'err'); });
    }

    function applyFilters() {
      state.q = ($('#fq').value || '').trim();
      state.dept = $('#fdept').value;
      state.dari = $('#fdari').value;
      state.sampai = $('#fsampai').value;
      state.urut = $('#furut').value;
      select(null);
      fetchAndRender();
    }

    $('#fq').addEventListener('keydown', function (e) { if (e.key === 'Enter') applyFilters(); });
    $('#furut').addEventListener('change', function () { state.urut = this.value; render(false); });
    $('#fsearch').onclick = applyFilters;
    $('#fdir').onclick = function () { state.dir *= -1; render(false); };
    $('#freset').onclick = function () {
      $('#fq').value = ''; $('#fdept').value = ''; $('#fdari').value = today; $('#fsampai').value = today; $('#furut').value = 'date';
      state.q = ''; state.dept = ''; state.dari = today; state.sampai = today; state.urut = 'date'; state.dir = -1;
      select(null);
      fetchAndRender();
    };
    $('#perPage').onchange = function () { state.per = num(this.value); render(); };
    $$('button[data-page]').forEach(function (b) {
      b.onclick = function () {
        var pages = num($('#totPage').textContent);
        state.page = b.dataset.page === 'next' ? Math.min(pages, state.page + 1) : Math.max(1, state.page - 1);
        render(false);
        window.scrollTo({ top: 0, behavior: 'smooth' });
      };
    });

    $('#btnEdit').onclick = function () {
      var d = getSelected();
      if (!d) return;
      api('/salespeople', 'GET').then(function (spList) {
        var activeSp = Array.isArray(spList) ? spList.filter(function (s) { return s.is_active; }) : [];
        var salesOpts = '<option value="">— Tidak ada sales —</option>' +
          activeSp.map(function (s) {
            var sel = d.sales && s.name.toLowerCase() === d.sales.toLowerCase() ? ' selected' : '';
            return '<option value="' + s.id + '"' + sel + '>' + esc(s.name) + '</option>';
          }).join('');
        var html =
          '<button class="dialog-x" data-close type="button">&times;</button>' +
          '<small class="eyebrow">EDIT TRANSAKSI</small>' +
          '<h2>' + esc(d.no) + '</h2>' +
          '<div class="field"><span>Total</span><input value="' + money(d.total) + '" disabled></div>' +
          '<label class="field"><span>Nama Pelanggan</span><input id="eName" value="' + esc(d.name) + '"></label>' +
          '<label class="field"><span>Sales</span><select id="eSales">' + salesOpts + '</select></label>' +
          '<label class="field"><span>Keterangan</span><input id="eNote" value="' + esc(d.note || '') + '"></label>' +
          '<div class="form-actions"><button class="btn btn-out" data-close type="button">Batal</button>' +
          '<button class="btn btn-success" id="eSave" type="button">Simpan</button></div>';
        openModal(html);
        $('#eSave').onclick = function () {
          var salesIdVal = $('#eSales').value ? Number($('#eSales').value) : null;
          api('/api/sales/' + encodeURIComponent(d.no), 'PATCH', {
            customer: $('#eName').value.trim(),
            sales_id: salesIdVal,
            note: $('#eNote').value.trim()
          }).then(function () {
            closeModal();
            toast('Transaksi diperbarui.', 'ok');
            fetchAndRender();
          }).catch(function (err) { toast(err.message, 'err'); });
        };
      }).catch(function () { toast('Gagal memuat daftar sales.', 'err'); });
    };

    $('#btnContinue').onclick = function () {
      var d = getSelected();
      if (!d || d.status === 'paid') return;
      var no = d.no;
      api('/api/sales/' + encodeURIComponent(no), 'GET').then(function (sale) {
        var storeId = sale.store_id;
        return api('/api/items?limit=500&store_id=' + storeId, 'GET').then(function (itemsResult) {
          var rawProds = Array.isArray(itemsResult) ? itemsResult : (itemsResult.data || []);
          var products = rawProds.filter(function (p) { return p.is_active !== false; }).map(function (p) {
            var storeStock = 0;
            (p.stock || []).forEach(function (s) { if (String(s.store_id) === String(storeId)) storeStock = Number(s.qty); });
            return { id: p.id, sku: p.code, name: p.name, price: Number(p.sell_price), stock: storeStock, unit: normalizeUnit(p.unit) };
          });
          return { sale: sale, products: products };
        });
      }).then(function (ctx) {
        var sale = ctx.sale, products = ctx.products;
        var added = [];
        var productOptions = products.map(function (p) { return '<option value="' + esc(p.sku + ' — ' + p.name) + '"></option>'; }).join('');
        var existing = (sale.items || []).map(function (i) {
          return '<tr><td>' + esc(i.product_code) + '</td><td>' + esc(i.product_name) + '</td><td class="right">' + qtyText(i.qty, i.unit) + '</td><td class="right">' + money(Number(i.qty) * Number(i.unit_price)) + '</td></tr>';
        }).join('');
        var paymentHistory = (sale.payments || []).map(function (p) {
          return '<tr><td>' + esc(p.method) + '</td><td>' + esc(new Date(p.created_at).toLocaleString('id-ID')) + '</td><td class="right">' + money(p.amount) + '</td></tr>';
        }).join('');
        var salePaid = Number(sale.paid_amount);
        function addedRows() {
          $('#continueRows').innerHTML = added.map(function (i, ix) {
            return '<tr><td>' + esc(i.sku) + '</td><td>' + esc(i.name) + '</td><td class="right">' + qtyText(i.qty, i.unit) + '</td><td class="right">' + money(i.qty * i.price) + '</td>' +
              '<td class="right"><button type="button" class="sale-remove" data-remove-added="' + ix + '">Hapus</button></td></tr>';
          }).join('');
          var addedTotal = added.reduce(function (sum, i) { return sum + i.qty * i.price; }, 0);
          var rawSubtotal = (Number(sale.subtotal) || Number(sale.total)) + addedTotal;
          var discountAmount = Math.round(rawSubtotal * (Number(sale.discount_pct) || 0) / 100);
          currentDue = Math.max(0, rawSubtotal - discountAmount) - salePaid;
          $('#continueDue').textContent = money(currentDue);
          $('#continueAddTotal').textContent = money(addedTotal);
          $('#continuePayAmount').max = currentDue;
          validateContinue();
        }
        function findBillProduct(value) {
          var v = String(value || '').trim().toLowerCase();
          if (!v) return null;
          var exact = products.filter(function (p) {
            return v === p.sku.toLowerCase() || v === p.name.toLowerCase() ||
              v === (p.sku + ' — ' + p.name).toLowerCase() || v === (p.sku + ' - ' + p.name).toLowerCase();
          })[0];
          if (exact) return exact;
          return products.filter(function (p) { return p.sku.toLowerCase().indexOf(v) !== -1 || p.name.toLowerCase().indexOf(v) !== -1; })[0];
        }
        function validateContinue() {
          var amount = num($('#continuePayAmount').value);
          var hasItems = added.length > 0;
          var validPay = amount >= 0 && amount <= currentDue && (amount === 0 || !!$('#continueMethod').value);
          $('#continueSave').disabled = (!hasItems && amount <= 0) || !validPay;
        }
        var currentDue = Number(sale.total) - salePaid;
        var html =
          '<button class="dialog-x" data-close type="button">&times;</button>' +
          '<small class="eyebrow">BILL PENDING</small><h2>Lanjutkan ' + esc(no) + '</h2>' +
          '<p class="muted" style="margin:0 0 10px">' + esc(sale.customer_name || 'Pelanggan') + ' \xb7 Sudah dibayar ' + money(salePaid) + '</p>' +
          '<h3 style="font-size:13px;margin:12px 0 6px">Riwayat pembayaran</h3><div class="tablewrap"><table class="grid"><thead><tr><th>Metode</th><th>Waktu</th><th class="right">Nominal</th></tr></thead><tbody>' + (paymentHistory || '<tr><td colspan="3" class="muted">Belum ada pembayaran.</td></tr>') + '</tbody></table></div>' +
          '<div class="tablewrap"><table class="grid"><thead><tr><th>Kode</th><th>Item sebelumnya</th><th class="right">Jumlah</th><th class="right">Total</th></tr></thead><tbody>' + existing + '</tbody></table></div>' +
          '<label class="field" style="margin-top:14px"><span>Tambah barang (opsional)</span><input id="continueProduct" list="continueProductOptions" placeholder="Cari kode atau nama item"><datalist id="continueProductOptions">' + productOptions + '</datalist></label>' +
          '<div class="form2"><label class="field"><span id="continueQtyLabel">Jumlah</span><input id="continueQty" type="number" min="0.1" step="0.1" value="1.0"></label><button class="btn btn-out" id="continueAdd" type="button" style="align-self:end">Tambah barang</button></div>' +
          '<div class="tablewrap" style="margin-top:8px"><table class="grid"><thead><tr><th>Kode</th><th>Barang tambahan</th><th class="right">Jumlah</th><th class="right">Total</th><th></th></tr></thead><tbody id="continueRows"></tbody></table></div>' +
          '<div class="continue-balance"><span>Total barang tambahan</span><b id="continueAddTotal">Rp 0</b></div>' +
          '<div class="continue-balance"><span>Sisa setelah barang tambahan</span><b id="continueDue">' + money(currentDue) + '</b></div>' +
          '<div class="form2"><label class="field"><span>Pembayaran tambahan (boleh kosong)</span><input id="continuePayAmount" type="text" inputmode="numeric" data-money-input autocomplete="off" placeholder="0" value="0"></label>' +
          '<label class="field"><span>Metode pembayaran</span><select id="continueMethod"><option value="">Tidak ada pembayaran</option><option>DP</option><option>Cash</option><option>Transfer</option></select></label></div>' +
          '<label class="field" style="margin-top:10px"><span>Catatan tambahan</span><input id="continueNote" placeholder="Opsional"></label>' +
          '<div class="form-actions"><button class="btn btn-out" data-close type="button">Batal</button><button class="btn btn-success" id="continueSave" type="button">Simpan bill</button></div>';
        openModal(html, 'dialog-wide');
        $('#continueAdd').onclick = function () {
          var p = findBillProduct($('#continueProduct').value);
          var qty = p ? normalizeQty(num($('#continueQty').value), p.unit) : 0;
          if (!p) { toast('Pilih item dari daftar yang tersedia.', 'warn'); return; }
          if (qty < qtyMinimum(p.unit)) { toast(p.unit === 'pcs' ? 'Jumlah minimal 1 pcs.' : 'Jumlah minimal 0,1 meter.', 'warn'); return; }
          var inAdded = added.filter(function (i) { return i.sku === p.sku; })[0];
          if ((inAdded ? inAdded.qty : 0) + qty > p.stock) { toast('Stok ' + p.name + ' hanya tersisa ' + qtyText(p.stock, p.unit) + '.', 'warn'); return; }
          if (inAdded) inAdded.qty += qty; else added.push({ id: p.id, sku: p.sku, name: p.name, price: p.price, qty: qty, stock: p.stock, unit: p.unit });
          $('#continueProduct').value = '';
          $('#continueQty').value = '1.0';
          addedRows();
        };
        $('#continueProduct').addEventListener('input', function () {
          var p = findBillProduct(this.value);
          if (!p) return;
          $('#continueQty').step = qtyStep(p.unit);
          $('#continueQty').min = String(qtyMinimum(p.unit));
          $('#continueQtyLabel').textContent = 'Jumlah (' + unitName(p.unit) + ')';
        });
        $('#continueRows').addEventListener('click', function (e) {
          var b = e.target.closest('[data-remove-added]');
          if (b) { added.splice(Number(b.dataset.removeAdded), 1); addedRows(); }
        });
        $('#continuePayAmount').addEventListener('input', validateContinue);
        $('#continueMethod').addEventListener('change', validateContinue);
        $('#continueSave').onclick = function () {
          var amount = parseMoney($('#continuePayAmount').value);
          api('/api/sales/' + encodeURIComponent(no) + '/continue', 'POST', {
            customer: sale.customer_name || '',
            sales: sale.sales_name || '',
            note: $('#continueNote').value.trim(),
            method: $('#continueMethod').value,
            amount: amount,
            items: added.map(function (i) { return { product_id: i.id, qty: i.qty }; })
          }).then(function (updated) {
            closeModal();
            toast(updated.pending ? 'Bill disimpan dan masih pending.' : 'Pembayaran lunas, bill ditutup.', 'ok');
            fetchAndRender();
          }).catch(function (err) { toast(err.message, 'err'); });
        };
        addedRows();
      }).catch(function (err) { toast(err.message, 'err'); });
    };

    var delBtn = $('#btnDelete');
    if (delBtn) {
      delBtn.onclick = function () {
        var d = getSelected();
        if (!d) return;
        var no = d.no;
        confirmBox({
          title: 'Hapus transaksi?',
          body: 'Transaksi <b>' + esc(no) + '</b> akan dibatalkan (void). Stok semua barang akan dikembalikan ke inventori.',
          ok: 'Ya, Hapus & Kembalikan Stok'
        }).then(function (ok) {
          if (!ok) return;
          api('/api/sales/' + encodeURIComponent(no), 'DELETE').then(function () {
            toast('Transaksi dibatalkan dan stok dikembalikan.', 'ok');
            select(null);
            fetchAndRender();
          }).catch(function (err) { toast(err.message, 'err'); });
        });
      };
    }

    $('#btnReport').onclick = function () {
      var list = filtered();
      var from = state.dari, to = state.sampai;
      var payments = [];
      var included = {};
      list.forEach(function (d) {
        payments.push({ sale: d, date: new Date(d.ts), amount: d.paid, method: 'Pembayaran' });
        included[d.no] = d;
      });
      payments.sort(function (a, b) { return a.date - b.date; });
      var rowsHtml = payments.map(function (p) {
        var d = p.sale;
        return '<article class="thermal-entry">' +
          '<div class="thermal-entry-top"><b>' + esc(d.no) + '</b><span>' + esc(p.date.toLocaleString('id-ID', { dateStyle: 'short', timeStyle: 'short' })) + '</span></div>' +
          '<div>' + esc(d.name || 'Pelanggan umum') + '</div>' +
          '<div class="thermal-pair"><span>' + esc(p.method) + ' diterima</span><b>' + money(p.amount) + '</b></div>' +
          '<div class="thermal-pair"><span>' + (d.status !== 'paid' ? 'Pending \xb7 sisa' : 'Status bill') + '</span><b>' + (d.status !== 'paid' ? money(d.balance) : 'Lunas') + '</b></div></article>';
      }).join('');
      var totalCollected = payments.reduce(function (a, p) { return a + p.amount; }, 0);
      var totalBills = Object.keys(included).reduce(function (a, no) { return a + included[no].total; }, 0);
      var pendingBalance = Object.keys(included).reduce(function (a, no) {
        var sale = included[no];
        return a + (sale.status !== 'paid' ? sale.balance : 0);
      }, 0);
      var period = from && to ? (from === to ? from : from + ' sampai ' + to) : (from ? 'Sejak ' + from : (to ? 'Sampai ' + to : 'Semua tanggal'));
      var reportMarkup = '<section class="thermal-sheet">' + thermalHeader('LAPORAN KAS MASUK', period) +
        '<div class="thermal-summary">' +
        '<div class="thermal-pair"><span>Jumlah pembayaran</span><b>' + payments.length + '</b></div>' +
        '<div class="thermal-pair"><span>Total nilai bill</span><b>' + money(totalBills) + '</b></div>' +
        '<div class="thermal-pair"><span>Uang diterima</span><b>' + money(totalCollected) + '</b></div>' +
        '<div class="thermal-pair"><span>Sisa pending DP</span><b>' + money(pendingBalance) + '</b></div></div>' +
        '<div class="thermal-entries">' + (rowsHtml || '<p class="thermal-empty">Tidak ada pembayaran dalam periode ini.</p>') + '</div>' +
        '<footer class="thermal-footer">Dicetak ' + esc(new Date().toLocaleString('id-ID')) + '<br>Rekap kas masuk</footer></section>';
      openModal(
        '<button class="dialog-x" data-close type="button">&times;</button>' +
        '<small class="eyebrow">LAPORAN HARIAN</small>' +
        '<h2>Kas masuk dan bill pending</h2>' + reportMarkup +
        '<div class="form-actions"><button class="btn btn-out" data-close type="button">Tutup</button>' +
        '<button class="btn btn-success" id="doPrint" type="button">Cetak</button></div>'
      );
      $('#doPrint').onclick = function () { printThermal(reportMarkup); };
    };

    var printHistory = $('#btnPreviewReport') || $('#printSalesHistory');
    if (printHistory) printHistory.onclick = function () {
      var list = filtered();
      var grandTotal = 0, grandPaid = 0, grandPending = 0, grandDiscount = 0;
      var entries = list.map(function (d) {
        grandTotal += d.total;
        grandPaid += d.paid;
        grandDiscount += d.discountAmount;
        if (d.status !== 'paid') grandPending += d.balance;
        var date = new Date(d.ts);
        var saleQuantities = d.items.map(function (it) { return { qty: Number(it.qty), unit: it.unit }; });
        var itemMarkup = d.items.map(function (it, ix) {
          return '<tr><td>' + (ix + 1) + '</td><td class="mono">' + esc(it.product_code) + '</td>' +
            '<td>' + esc(it.product_name) + '</td>' +
            '<td class="right">' + qtyText(Number(it.qty), it.unit) + '</td>' +
            '<td>' + (it.unit === 'pcs' ? 'PCS' : 'MTR') + '</td>' +
            '<td class="right">' + money(Number(it.unit_price)) + '</td>' +
            '<td class="right">0,00</td>' +
            '<td class="right">' + money(Number(it.line_total)) + '</td></tr>';
        }).join('');
        var storeLabel = d.dept || (document.body.dataset.dept) || 'Sumbawa Textile';
        return '<article class="report-sale">' +
          '<div class="report-sale-meta"><b>' + esc(d.no) + '</b><span>' + esc(date.toLocaleDateString('id-ID')) + '</span>' +
          '<span>' + esc(storeLabel) + '</span><span>' + esc(d.sales || '—') + '</span>' +
          '<strong>' + esc(d.name || 'UMUM/CASH') + '</strong><span>—</span></div>' +
          '<table class="report-items"><thead><tr><th>No</th><th>Kd. Item</th><th>Nama Item</th>' +
          '<th class="right">Jml</th><th>Satuan</th><th class="right">Harga</th><th class="right">Pot%</th><th class="right">Total</th></tr></thead>' +
          '<tbody>' + (itemMarkup || '<tr><td colspan="8" class="muted">Detail barang tidak tersedia.</td></tr>') + '</tbody></table>' +
          '<div class="report-sale-summary"><span>Jumlah: ' + qtySummary(saleQuantities) + '</span>' +
          '<span>Subtotal: ' + money(d.subtotal || d.total) + '</span>' +
          '<span>Pot (' + (d.discountPercent || 0) + '%): ' + money(d.discountAmount || 0) + '</span>' +
          '<span>Pajak: Rp 0</span><span>Biaya: Rp 0</span>' +
          '<strong>Total Akhir: ' + money(d.total) + '</strong></div>' +
          '<div class="report-settlement">Dibayar: ' + money(d.paid) + ' \xb7 ' + (d.status !== 'paid' ? 'Pending pembayaran: ' + money(d.balance) : 'Lunas') + '</div></article>';
      }).join('');
      function reportDate(iso) {
        if (!iso) return '—';
        var parts = iso.split('-');
        return parts.length === 3 ? parts[2] + '/' + parts[1] + '/' + parts[0] : iso;
      }
      var period = state.dari || state.sampai ? reportDate(state.dari || state.sampai) + ' 00:00:00 — ' + reportDate(state.sampai || state.dari) + ' 23:59:59' : 'Semua tanggal';
      var reportShopName = document.body.dataset.dept || 'Sumbawa Textile';
      var reportMarkup = '<section class="report-a4"><header class="report-a4-head"><div class="report-brand"><span class="report-logo">S</span><div><h1>Laporan Penjualan Detail</h1><b>' + esc(reportShopName) + '</b></div></div>' +
        '<dl><dt>Periode</dt><dd>' + esc(period) + '</dd><dt>No Transaksi</dt><dd>' + esc(state.q || '—') + '</dd><dt>Pelanggan</dt><dd>—</dd><dt>Supplier</dt><dd>—</dd></dl></header>' +
        '<div class="report-column-head"><b>No Transaksi</b><b>Tanggal</b><b>Dept</b><b>Nama Sales</b><b>Nama Pelanggan</b><b>Alamat</b></div>' +
        '<div class="report-sales-list">' + (entries || '<p class="report-empty">Tidak ada transaksi pada filter ini.</p>') + '</div>' +
        '<footer class="report-a4-footer"><span>' + list.length + ' transaksi</span><strong>Total penjualan: ' + money(grandTotal) + '</strong><small>Dicetak ' + esc(new Date().toLocaleString('id-ID')) + '</small></footer></section>';
      var thermalMarkup = '<section class="thermal-sheet">' + thermalHeader('LAPORAN PENJUALAN DETAIL', period) +
        '<div class="thermal-receipt-meta"><div><span>Jumlah transaksi</span><b>' + list.length + '</b></div><div><span>Total penjualan</span><b>' + money(grandTotal) + '</b></div><div><span>Total diskon</span><b>' + money(grandDiscount) + '</b></div>' +
        '<div><span>Total dibayar</span><b>' + money(grandPaid) + '</b></div><div><span>Total pending</span><b>' + money(grandPending) + '</b></div></div>' +
        '<hr><div class="thermal-items-list">' + (entries ? list.map(function (d) {
          var date = new Date(d.ts);
          var thermalItems = d.items.map(function (it) {
            return '<div class="thermal-item"><b>' + esc(it.product_name) + '</b>' +
              '<div class="thermal-pair"><span>' + qtyText(Number(it.qty), it.unit) + ' \xd7 ' + money(Number(it.unit_price)) + '</span><b>' + money(Number(it.line_total)) + '</b></div></div>';
          }).join('');
          return '<article class="thermal-entry"><div class="thermal-entry-top"><b>' + esc(d.no) + '</b><span>' + esc(date.toLocaleString('id-ID', { dateStyle: 'short', timeStyle: 'short' })) + '</span></div>' +
            '<div>Pelanggan: ' + esc(d.name || 'UMUM/CASH') + '</div><div>Sales: ' + esc(d.sales || '—') + '</div>' +
            (thermalItems || '<small>Detail barang tidak tersedia.</small>') +
            (d.discountAmount > 0 ? '<div class="thermal-pair"><span>Subtotal</span><span>' + money(d.subtotal) + '</span></div><div class="thermal-pair"><span>Diskon (' + esc(String(d.discountPercent)) + '%)</span><span>− ' + money(d.discountAmount) + '</span></div>' : '') +
            '<div class="thermal-pair"><b>Total</b><b>' + money(d.total) + '</b></div>' +
            '<div class="thermal-pair"><span>Dibayar</span><span>' + money(d.paid) + '</span></div>' +
            (d.status !== 'paid' ? '<div class="thermal-pair"><b>Pending</b><b>' + money(d.balance) + '</b></div>' : '<div class="thermal-pair"><b>Status</b><b>LUNAS</b></div>') +
            '</article>';
        }).join('') : '<p>Tidak ada transaksi pada filter ini.</p>') + '</div><small>Dicetak ' + esc(new Date().toLocaleString('id-ID')) + '</small></section>';
      a4Preview('Laporan Penjualan Detail', reportMarkup, thermalMarkup);
    };

    fetchAndRender();
  }

  /* --------------------------------------------------------- void orders */
  function voidOrdersPage() {
    var table = $('#voidTable');
    if (!table) return;
    var tbody = $('#voidRows');

    function todayISO() {
      var d = new Date();
      return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0');
    }
    var today = todayISO();
    if ($('#vdari')) $('#vdari').value = today;
    if ($('#vsampai')) $('#vsampai').value = today;

    var state = { q: '', dept: '', dari: today, sampai: today, page: 1, per: 10 };
    var allRows = [];

    api('/stores', 'GET').then(function (data) {
      var sel = $('#vdept');
      if (sel && Array.isArray(data)) {
        data.forEach(function (s) {
          var o = document.createElement('option');
          o.value = s.Name; o.textContent = s.Name;
          sel.appendChild(o);
        });
      }
    }).catch(function () {});

    function dtStr(ts) {
      var d = new Date(ts);
      return d.toLocaleDateString('id-ID') + ' ' +
        String(d.getHours()).padStart(2, '0') + ':' +
        String(d.getMinutes()).padStart(2, '0') + ':' +
        String(d.getSeconds()).padStart(2, '0');
    }

    function buildRow(d) {
      var count = d.items.length;
      var qs = qtySummary(d.items.map(function (it) { return { qty: Number(it.qty), unit: it.unit }; }));
      var itemsHtml = d.items.map(function (it) {
        return '<tr><td class="mono">' + esc(it.product_code) + '</td><td>' + esc(it.product_name) + '</td>' +
          '<td class="right">' + qtyText(Number(it.qty), it.unit) + '</td>' +
          '<td class="right">' + money(Number(it.unit_price)) + '</td>' +
          '<td class="right">' + money(Number(it.line_total)) + '</td></tr>';
      }).join('');
      return '<tr>' +
        '<td class="mono strong">' + esc(d.no) + '</td>' +
        '<td class="mono">' + esc(dtStr(d.ts)) + '</td>' +
        '<td>' + esc(d.dept) + '</td>' +
        '<td class="strong">' + esc(d.name) + '</td>' +
        '<td>' + (d.sales ? esc(d.sales) : '<span class="muted">UMUM</span>') + '</td>' +
        '<td class="right mono strong">' + money(d.total) + '</td>' +
        '<td class="ellip"><span class="chip chip-warn">' + esc(d.voidReason || 'Dibatalkan') + '</span></td>' +
        '<td><details class="sale-detail"><summary>' + count + ' item \xb7 ' + esc(qs) + '</summary>' +
          '<div class="sale-detail-panel"><table><thead><tr><th>Kode</th><th>Nama barang</th>' +
          '<th class="right">Jumlah</th><th class="right">Harga/satuan</th><th class="right">Subtotal</th></tr></thead>' +
          '<tbody>' + (itemsHtml || '<tr><td colspan="5" class="muted">Tidak ada detail.</td></tr>') + '</tbody>' +
          '</table></div></details></td>' +
        '<td class="ellip">' + (d.note ? esc(d.note) : '<span class="muted">&mdash;</span>') + '</td>' +
        '<td>' + esc(d.cashier) + '</td>' +
        '</tr>';
    }

    function filtered() {
      return allRows.filter(function (d) {
        if (state.q && d.hay.indexOf(state.q.toLowerCase()) === -1) return false;
        if (state.dept && d.dept !== state.dept) return false;
        return true;
      });
    }

    function render(resetPage) {
      if (resetPage !== false) state.page = 1;
      var list = filtered();
      var pages = Math.max(1, Math.ceil(list.length / state.per));
      if (state.page > pages) state.page = pages;
      var shown = list.slice((state.page - 1) * state.per, state.page * state.per);
      tbody.innerHTML = shown.map(buildRow).join('');
      var sum = list.reduce(function (s, d) { return s + d.total; }, 0);
      $('#vnodata').hidden = list.length !== 0;
      $('#vCountInfo').textContent = 'Menampilkan ' + shown.length + ' dari ' + list.length + ' transaksi';
      $('#vSumTotal').textContent = money(sum);
      $('#vCurPage').textContent = state.page;
      $('#vTotPage').textContent = pages;
    }

    function fetchAndRender() {
      var params = 'status=void&limit=500';
      if (state.dari) params += '&date_from=' + encodeURIComponent(state.dari);
      if (state.sampai) params += '&date_to=' + encodeURIComponent(state.sampai);
      if (state.q) params += '&q=' + encodeURIComponent(state.q);
      api('/api/sales?' + params, 'GET').then(function (result) {
        var data = Array.isArray(result) ? result : (result.data || []);
        allRows = data.map(function (o) {
          return {
            no: o.order_no,
            ts: new Date(o.created_at).getTime(),
            dept: o.store_name || '',
            name: o.customer_name || '',
            sales: o.sales_name || '',
            cashier: o.cashier_name || '',
            note: o.note || '',
            total: Number(o.total),
            items: o.items || [],
            voidReason: o.void_reason || '',
            hay: [o.order_no, o.customer_name || '', o.cashier_name || '', o.sales_name || '', o.void_reason || ''].join(' ').toLowerCase()
          };
        });
        render();
      }).catch(function (err) { toast(err.message || 'Gagal memuat data.', 'err'); });
    }

    $('#vq').addEventListener('keydown', function (e) { if (e.key === 'Enter') fetchAndRender(); });
    $('#vsearch').onclick = function () {
      state.q = ($('#vq').value || '').trim();
      state.dept = $('#vdept').value;
      state.dari = $('#vdari').value;
      state.sampai = $('#vsampai').value;
      fetchAndRender();
    };
    $('#vreset').onclick = function () {
      $('#vq').value = ''; $('#vdept').value = ''; $('#vdari').value = today; $('#vsampai').value = today;
      state.q = ''; state.dept = ''; state.dari = today; state.sampai = today;
      fetchAndRender();
    };
    $('#vPerPage').onchange = function () { state.per = num(this.value); render(); };
    $$('button[data-vpage]').forEach(function (b) {
      b.onclick = function () {
        var pages = num($('#vTotPage').textContent);
        state.page = b.dataset.vpage === 'next' ? Math.min(pages, state.page + 1) : Math.max(1, state.page - 1);
        render(false);
        window.scrollTo({ top: 0, behavior: 'smooth' });
      };
    });

    fetchAndRender();
  }

  /* ------------------------------------------------------------ transaksi */
  function transaksiPage() {
    var tbody = $('#trxRows');
    if (!tbody) return;
    var rows = $$('tr.grow', tbody);
    var state = { cashier: '', pay: '', per: 10, page: 1 };

    rows.forEach(function (r) {
      r._d = {
        no: r.dataset.no,
        ts: Number(r.dataset.date),
        cashier: r.dataset.cashier,
        pay: r.dataset.pay,
        total: Number(r.dataset.total),
        status: r.dataset.status || 'paid'
      };
    });

    function apply() {
      state.page = 1;
      var list = rows.filter(function (r) {
        return (!state.cashier || r._d.cashier === state.cashier) && (!state.pay || r._d.pay === state.pay) &&
          (!$('#tstatus').value || r._d.status === $('#tstatus').value);
      });
      list.sort(function (a, b) { return b._d.ts - a._d.ts; });
      var pages = Math.max(1, Math.ceil(list.length / state.per));
      var shown = list.slice(0, state.per);
      rows.forEach(function (r) { r.hidden = shown.indexOf(r) === -1; });
      $('#trxNoData').hidden = list.length !== 0;
      $('#trxCountInfo').textContent = list.length + ' transaksi';
      $('#trxSum').textContent = money(list.reduce(function (a, r) { return a + r._d.total; }, 0));
      return list;
    }

    $('#tcashier').onchange = function () { state.cashier = this.value; apply(); };
    $('#taccount').onchange = function () { state.pay = this.value; apply(); };
    $('#tstatus').onchange = function () { apply(); };
    $('#tperPage').onchange = function () { state.per = num(this.value); apply(); };

    var list = apply();

    var byPay = {}, byHour = {};
    list.forEach(function (r) {
      var d = r._d;
      byPay[d.pay] = (byPay[d.pay] || 0) + d.total;
      var h = new Date(d.ts).getHours();
      byHour[h] = (byHour[h] || 0) + 1;
    });

    var rank = $('#payRank');
    Object.keys(byPay).sort(function (a, b) { return byPay[b] - byPay[a]; }).forEach(function (k, i) {
      var li = document.createElement('li');
      li.innerHTML = '<span class="rk">' + (i + 1) + '</span><div class="rk-b"><b>' + esc(k) + '</b></div>' +
        '<div class="rk-v"><b class="mono">' + money(byPay[k]) + '</b></div>';
      rank.appendChild(li);
    });

    var hours = $('#hours');
    var max = Math.max.apply(null, Object.keys(byHour).map(function (h) { return byHour[h]; }).concat([1]));
    for (var h2 = 8; h2 <= 21; h2++) {
      var v = byHour[h2] || 0;
      var d2 = document.createElement('div');
      d2.className = 'hour';
      d2.title = 'Jam ' + h2 + ': ' + v + ' transaksi';
      d2.innerHTML = '<em>' + (v || '') + '</em><i style="height:' + Math.round((v / max) * 100) + '%"></i><b>' + h2 + '</b>';
      hours.appendChild(d2);
    }
  }

  /* ---------------------------------------------------------------- items */
  function itemsPage() {
    var tbody = $('#itemRows');
    if (!tbody) return;
    var rows = $$('tr.grow', tbody);
    var selected = null;
    var stores = [];

    // Populate store filter dropdown.
    api('/stores', 'GET').then(function (data) {
      var sel = $('#istore');
      if (sel && Array.isArray(data)) {
        data.forEach(function (s) {
          var o = document.createElement('option');
          o.value = String(s.ID); o.textContent = s.Name;
          sel.appendChild(o);
        });
      }
    }).catch(function () {});

    function filter() {
      var q = $('#isearch').value.trim().toLowerCase();
      var cat = $('#icat').value;
      var st = $('#istock').value;
      var store = $('#istore').value;
      var n = 0;
      rows.forEach(function (r) {
        var ok = (!q || r.dataset.hay.toLowerCase().indexOf(q) > -1) &&
          (!cat || r.dataset.cat === cat) &&
          (!st || (st === 'low' ? r.dataset.low === 'true' : r.dataset.low !== 'true')) &&
          (!store || String(r.dataset.storeId || '') === store);
        r.hidden = !ok;
        if (ok) n++;
      });
      $('#itemNoData').hidden = n !== 0;
      $('#itemCount').textContent = n + ' dari ' + rows.length + ' item';
    }

    ['#isearch', '#icat', '#istock', '#istore'].forEach(function (id) {
      $(id).addEventListener('input', filter);
    });

    tbody.addEventListener('click', function (e) {
      var r = e.target.closest('tr.grow');
      if (!r) return;
      if (selected) selected.classList.remove('is-sel');
      selected = r;
      r.classList.add('is-sel');
      $('#editItem').disabled = false;
      $('#delItem').disabled = false;
    });

    function form(sku) {
      var r = rows.filter(function (x) { return x.dataset.sku === sku; })[0];
      // Column order: 0=pick, 1=kode, 2=nama, 3=toko, 4=jenis, 5=stok, 6=satuan, 7=harga pokok, 8=harga jual, 9=margin
      var cell = function (i) { return r ? r.children[i].textContent.trim() : ''; };
      var currentStoreID = r ? Number(r.dataset.storeId || '0') : (stores.length ? stores[0].ID : 0);
      var storeOpts = '<option value="">— Pilih Toko —</option>' +
        stores.map(function (s) {
          return '<option value="' + s.ID + '"' + (s.ID === currentStoreID ? ' selected' : '') + '>' + esc(s.Name) + '</option>';
        }).join('');
      openModal(
        '<button class="dialog-x" data-close type="button">&times;</button>' +
        '<small class="eyebrow">MASTER ITEM</small>' +
        '<h2>' + (r ? 'Ubah Barang' : 'Tambah Barang') + '</h2>' +
        '<form class="form" id="itemForm">' +
        '<div class="form2">' +
        '<label class="field"><span>Kode Item</span><input name="sku" required value="' + (r ? esc(cell(1)) : '') + '"' + (r ? ' readonly' : '') + '></label>' +
        '<label class="field"><span>Nama Item</span><input name="name" required value="' + (r ? esc(cell(2)) : '') + '"></label>' +
        '</div>' +
        '<label class="field"><span>Toko</span><select name="store_id">' + storeOpts + '</select></label>' +
        '<div class="form3">' +
        '<label class="field"><span>Jenis</span><input name="category" required value="' + (r ? esc(cell(4)) : '') + '" placeholder="Contoh: Kain, Aksesori"></label>' +
        '<label class="field"><span>Satuan</span><select name="unit"><option value="meter"' + (!r || cell(6).toLowerCase() === 'meter' ? ' selected' : '') + '>Meter</option><option value="pcs"' + (r && cell(6).toLowerCase() === 'pcs' ? ' selected' : '') + '>Pcs</option></select></label>' +
        '<label class="field"><span>Stok</span><input name="stock" type="number" min="0" step="' + (r && cell(6).toLowerCase() === 'pcs' ? '1' : '0.1') + '" value="' + (r ? num(cell(5).replace(/[^\d,.-]/g, '').replace(',', '.')) : 0) + '"></label>' +
        '</div>' +
        '<div class="form3">' +
        '<label class="field"><span>Harga Pokok</span><input name="cost" type="text" inputmode="numeric" data-money-input autocomplete="off" value="' + (r ? num(cell(7).replace(/\D/g, '')) : 0) + '"></label>' +
        '<label class="field"><span>Harga Jual</span><input name="price" type="text" inputmode="numeric" data-money-input autocomplete="off" required value="' + (r ? num(cell(8).replace(/\D/g, '')) : 0) + '"></label>' +
        '<label class="field"><span>Margin / satuan (otomatis)</span><input id="itemMarginPreview" value="' + (r ? esc(cell(9)) : 'Rp 0') + '" readonly></label>' +
        '</div>' +
        '<div class="form-actions"><button class="btn btn-out" data-close type="button">Batal</button>' +
        '<button class="btn btn-success" type="submit">Simpan</button></div></form>'
      );

      var itemForm = $('#itemForm');
      var marginPreview = function () {
        var cost = parseMoney(itemForm.elements.namedItem('cost').value);
        var price = parseMoney(itemForm.elements.namedItem('price').value);
        $('#itemMarginPreview').value = money(price - cost);
      };
      itemForm.elements.namedItem('cost').addEventListener('input', marginPreview);
      itemForm.elements.namedItem('price').addEventListener('input', marginPreview);
      itemForm.elements.namedItem('unit').addEventListener('change', function () {
        itemForm.elements.namedItem('stock').step = this.value === 'pcs' ? '1' : '0.1';
        itemForm.elements.namedItem('stock').min = '0';
      });
      marginPreview();
      $('#itemForm').onsubmit = function (e) {
        e.preventDefault();
        var f = e.target;
        var storeEl = f.elements.namedItem('store_id');
        var storeVal = storeEl && storeEl.value ? Number(storeEl.value) : null;
        api('/api/items', 'POST', {
          name: f.elements.namedItem('name').value.trim(),
          sku: f.elements.namedItem('sku').value.trim(),
          category: f.elements.namedItem('category').value.trim(),
          unit: f.elements.namedItem('unit').value,
          price: parseMoney(f.elements.namedItem('price').value),
          cost: parseMoney(f.elements.namedItem('cost').value),
          stock: num(f.elements.namedItem('stock').value),
          store_id: storeVal
        }).then(function () {
          closeModal();
          toast('Item disimpan.', 'ok');
          setTimeout(function () { location.reload(); }, 800);
        }).catch(function (err) { toast(err.message, 'err'); });
      };
    }

    function ensureStores(cb) {
      if (stores && stores.length) { cb(); return; }
      api('/stores', 'GET').then(function (data) {
        stores = Array.isArray(data) ? data : [];
        cb();
      }).catch(function () { cb(); });
    }
    $('#addItem').onclick = function () { ensureStores(function () { form(null); }); };
    $('#editItem').onclick = function () { if (selected) ensureStores(function () { form(selected.dataset.sku); }); };

    $('#delItem').onclick = function () {
      if (!selected) return;
      var sku = selected.dataset.sku;
      confirmBox({ title: 'Hapus item?', body: 'Item <b>' + esc(sku) + '</b> dihapus dari Daftar Item.', ok: 'Hapus' })
        .then(function (ok) {
          if (!ok) return;
          api('/api/items/' + encodeURIComponent(sku), 'DELETE').then(function () {
            toast('Item dihapus.', 'ok');
            setTimeout(function () { location.reload(); }, 800);
          }).catch(function (err) { toast(err.message, 'err'); });
        });
    };

    filter();
    api('/stores', 'GET').then(function (data) {
      stores = Array.isArray(data) ? data : [];
      if (new URLSearchParams(location.search).get('add') === '1') {
        form(null);
        history.replaceState(null, '', location.pathname);
      }
    }).catch(function () {
      if (new URLSearchParams(location.search).get('add') === '1') {
        form(null);
        history.replaceState(null, '', location.pathname);
      }
    });
  }

  function datasheetPage() {
    var input = $('#excelFile');
    var button = $('#importExcel');
    if (!input || !button) return;
    input.addEventListener('change', function () {
      var file = input.files && input.files[0];
      $('#excelFileName').textContent = file ? file.name : 'Belum ada file dipilih';
      button.disabled = !file;
    });
    button.addEventListener('click', function () {
      var file = input.files && input.files[0];
      if (!file) return;
      button.disabled = true;
      button.textContent = 'Mengimpor…';
      var data = new FormData();
      data.append('file', file);
      fetch('/api/datasheet/import', { method: 'POST', body: data }).then(function (response) {
        return response.json().then(function (body) {
          if (!response.ok) throw new Error(body.error || 'Gagal mengimpor file Excel.');
          return body;
        });
      }).then(function (result) {
        toast(result.message || 'Data barang berhasil diimpor.', 'ok');
        setTimeout(function () { location.reload(); }, 650);
      }).catch(function (err) {
        toast(err.message, 'err');
        button.disabled = false;
        button.textContent = 'Impor Data Barang';
      });
    });
  }

  function adminUsersPage() {
    var adminRows = $('#adminRows'), cashierRows = $('#cashierRows');
    if (!adminRows || !cashierRows) return;
    var stores = [];

    function storeName(storeID) {
      if (!storeID) return '—';
      var s = stores.find(function (s) { return s.ID === storeID; });
      return s ? s.Name : '—';
    }

    function actionBtns(u) {
      return u.is_active
        ? '<button class="btn btn-danger btn-sm" type="button" data-deactivate="' + u.id + '" data-name="' + esc(u.full_name) + '">Nonaktifkan</button>'
        : '<button class="btn btn-out btn-sm" type="button" data-activate="' + u.id + '" data-name="' + esc(u.full_name) + '">Aktifkan</button>';
    }

    function statusChip(active) {
      return active ? '<span class="chip chip-ok">Aktif</span>' : '<span class="chip chip-warn">Nonaktif</span>';
    }

    function renderAll(users) {
      if (!users || !Array.isArray(users)) users = [];
      var admins = users.filter(function (u) { return u.role === 'admin'; });
      var cashiers = users.filter(function (u) { return u.role === 'cashier'; });

      adminRows.innerHTML = admins.length ? admins.map(function (u) {
        return '<tr><td class="strong">' + esc(u.full_name) + '</td>' +
          '<td class="mono">' + esc(u.username) + '</td>' +
          '<td>' + statusChip(u.is_active) + '</td>' +
          '<td>' + actionBtns(u) + '</td></tr>';
      }).join('') : '<tr><td colspan="4" class="muted">Belum ada admin.</td></tr>';

      cashierRows.innerHTML = cashiers.length ? cashiers.map(function (u) {
        return '<tr><td class="strong">' + esc(u.full_name) + '</td>' +
          '<td class="mono">' + esc(u.username) + '</td>' +
          '<td>' + esc(storeName(u.store_id)) + '</td>' +
          '<td>' + statusChip(u.is_active) + '</td>' +
          '<td>' + actionBtns(u) + '</td></tr>';
      }).join('') : '<tr><td colspan="5" class="muted">Belum ada kasir.</td></tr>';
    }

    function load() {
      api('/users', 'GET')
        .then(function (u) { renderAll(Array.isArray(u) ? u : (u && u.data) ? u.data : []); })
        .catch(function (err) { toast(err.message, 'err'); });
    }

    function userForm(role) {
      var storeOpts = stores.map(function (s) {
        return '<option value="' + s.ID + '">' + esc(s.Name) + '</option>';
      }).join('');
      var storeField = role === 'cashier'
        ? '<label class="field"><span>Toko (wajib untuk kasir)</span><select name="store_id" required><option value="">Pilih toko...</option>' + storeOpts + '</select></label>'
        : '';
      var title = role === 'cashier' ? 'Tambah Kasir' : 'Tambah Admin';
      var subtitle = role === 'cashier'
        ? 'Kasir dapat memproses transaksi di toko yang ditugaskan.'
        : 'Admin memiliki akses penuh ke semua toko dan laporan.';
      openModal('<button class="dialog-x" data-close type="button">&times;</button>' +
        '<small class="eyebrow">PENDAFTARAN USER</small><h2>' + title + '</h2>' +
        '<p style="color:var(--ink-2);font-size:13px;margin:0 0 16px">' + subtitle + '</p>' +
        '<form id="userRegForm" class="form">' +
        '<div class="form2">' +
        '<label class="field"><span>Nama Lengkap</span><input name="full_name" required autocomplete="name" placeholder="Contoh: Budi Santoso"></label>' +
        '<label class="field"><span>Username</span><input name="username" required autocomplete="off" placeholder="Contoh: budi.kasir" minlength="3" maxlength="50"></label>' +
        '</div>' +
        '<label class="field"><span>Kata Sandi</span><input name="password" type="password" required minlength="6" autocomplete="new-password" placeholder="Minimal 6 karakter"></label>' +
        storeField +
        '<div class="form-actions"><button class="btn btn-out" data-close type="button">Batal</button>' +
        '<button class="btn btn-success" type="submit">' + title + '</button></div></form>');

      $('#userRegForm').onsubmit = function (e) {
        e.preventDefault();
        var f = e.target.elements;
        var payload = {
          full_name: f.namedItem('full_name').value.trim(),
          username:  f.namedItem('username').value.trim(),
          password:  f.namedItem('password').value,
          role:      role
        };
        if (role === 'cashier') {
          var storeVal = f.namedItem('store_id') && f.namedItem('store_id').value;
          if (!storeVal) { toast('Pilih toko untuk kasir ini.', 'warn'); return; }
          payload.store_id = Number(storeVal);
        }
        api('/users', 'POST', payload).then(function () {
          closeModal();
          toast(title + ' berhasil didaftarkan.', 'ok');
          load();
        }).catch(function (err) { toast(err.message, 'err'); });
      };
    }

    $('#addCashier').onclick = function () { userForm('cashier'); };
    $('#addAdmin').onclick = function () { userForm('admin'); };

    // Toggle activate/deactivate from either table.
    [adminRows, cashierRows].forEach(function (tbl) {
      tbl.addEventListener('click', function (e) {
        var deactivateBtn = e.target.closest('[data-deactivate]');
        if (deactivateBtn) {
          var id = deactivateBtn.dataset.deactivate, name = deactivateBtn.dataset.name;
          confirmBox({ title: 'Nonaktifkan user?', body: 'Akun <b>' + esc(name) + '</b> tidak dapat login sampai diaktifkan kembali.', ok: 'Nonaktifkan' })
            .then(function (ok) {
              if (!ok) return;
              var v = false;
              api('/users/' + encodeURIComponent(id), 'PUT', { is_active: v })
                .then(function () { toast('User dinonaktifkan.', 'ok'); load(); })
                .catch(function (err) { toast(err.message, 'err'); });
            });
          return;
        }
        var activateBtn = e.target.closest('[data-activate]');
        if (activateBtn) {
          var actId = activateBtn.dataset.activate, actName = activateBtn.dataset.name;
          confirmBox({ title: 'Aktifkan user?', body: 'Akun <b>' + esc(actName) + '</b> akan bisa login kembali.', ok: 'Aktifkan', danger: false })
            .then(function (ok) {
              if (!ok) return;
              var v = true;
              api('/users/' + encodeURIComponent(actId), 'PUT', { is_active: v })
                .then(function () { toast('User diaktifkan.', 'ok'); load(); })
                .catch(function (err) { toast(err.message, 'err'); });
            });
        }
      });
    });

    api('/stores', 'GET').then(function (data) { stores = Array.isArray(data) ? data : []; load(); }).catch(function () { load(); });
  }

  function salespeoplePage() {
    var body = $('#salespersonRows');
    if (!body) return;

    function renderRows(list) {
      if (!list || !Array.isArray(list)) list = [];
      var active = list.filter(function (sp) { return sp.is_active !== false; });
      $('#salesCount').textContent = active.length + ' sales tersedia untuk dipilih pada transaksi baru.';
      body.innerHTML = active.length ? active.map(function (sp) {
        var rate = sp.commission_rate !== undefined ? (Number(sp.commission_rate) * 100).toFixed(1) + '%' : '—';
        return '<tr>' +
          '<td class="strong">' + esc(sp.name) + '</td>' +
          '<td class="mono">' + rate + '</td>' +
          '<td><span class="chip chip-ok">Aktif</span></td>' +
          '<td><div class="sales-row-actions">' +
          '<button class="btn btn-out" type="button" data-edit-id="' + sp.id + '" data-edit-name="' + esc(sp.name) + '" data-edit-rate="' + (Number(sp.commission_rate) * 100).toFixed(2) + '">Ubah</button>' +
          '<button class="btn btn-danger" type="button" data-del-id="' + sp.id + '" data-del-name="' + esc(sp.name) + '">Hapus</button>' +
          '</div></td></tr>';
      }).join('') : '<tr><td colspan="4" class="muted">Belum ada data sales.</td></tr>';
    }

    function load() {
      api('/salespeople', 'GET').then(renderRows).catch(function (err) { toast(err.message, 'err'); });
    }

    // Tambah Sales — popup modal.
    $('#addSales').onclick = function () {
      openModal('<button class="dialog-x" data-close type="button">&times;</button>' +
        '<small class="eyebrow">MASTER DATA SALES</small><h2>Tambah Sales</h2>' +
        '<form id="salesAddForm" class="form">' +
        '<label class="field"><span>Nama Sales</span><input name="name" required maxlength="100" autocomplete="off" placeholder="Contoh: Budi Santoso"></label>' +
        '<label class="field"><span>Komisi (%)</span><input name="rate" type="number" min="0" max="100" step="0.01" value="1" inputmode="decimal" placeholder="Contoh: 1 untuk 1%"></label>' +
        '<div class="form-actions"><button class="btn btn-out" data-close type="button">Batal</button>' +
        '<button class="btn btn-success" type="submit">Simpan</button></div></form>');
      $('#salesAddForm').onsubmit = function (e) {
        e.preventDefault();
        var els = e.target.elements;
        var name = els.namedItem('name').value.trim();
        var rate = (parseFloat(els.namedItem('rate').value) || 0) / 100;
        api('/salespeople', 'POST', { name: name, commission_rate: String(rate.toFixed(4)) }).then(function () {
          closeModal();
          toast('Sales ditambahkan.', 'ok');
          load();
        }).catch(function (err) { toast(err.message, 'err'); });
      };
    };

    body.addEventListener('click', function (e) {
      var editBtn = e.target.closest('[data-edit-id]');
      if (editBtn) {
        var id = editBtn.dataset.editId;
        var oldName = editBtn.dataset.editName;
        var oldRate = editBtn.dataset.editRate || '1';
        openModal('<button class="dialog-x" data-close type="button">&times;</button>' +
          '<small class="eyebrow">MASTER DATA SALES</small><h2>Ubah Sales</h2>' +
          '<form id="salesEditForm" class="form">' +
          '<label class="field"><span>Nama Sales</span><input name="name" required maxlength="100" value="' + esc(oldName) + '"></label>' +
          '<label class="field"><span>Komisi (%)</span><input name="rate" type="number" min="0" max="100" step="0.01" value="' + esc(oldRate) + '" inputmode="decimal"></label>' +
          '<div class="form-actions"><button class="btn btn-out" data-close type="button">Batal</button>' +
          '<button class="btn btn-success" type="submit">Simpan</button></div></form>');
        $('#salesEditForm').onsubmit = function (e) {
          e.preventDefault();
          var els = e.target.elements;
          var newName = els.namedItem('name').value.trim();
          var newRate = (parseFloat(els.namedItem('rate').value) || 0) / 100;
          api('/salespeople/' + encodeURIComponent(id), 'PUT', { name: newName, commission_rate: String(newRate.toFixed(4)) }).then(function () {
            closeModal();
            toast('Sales diperbarui.', 'ok');
            load();
          }).catch(function (err) { toast(err.message, 'err'); });
        };
        return;
      }

      var delBtn = e.target.closest('[data-del-id]');
      if (!delBtn) return;
      var delId = delBtn.dataset.delId;
      var delName = delBtn.dataset.delName;
      confirmBox({ title: 'Hapus sales?', body: 'Sales <b>' + esc(delName) + '</b> akan dinonaktifkan dari daftar.', ok: 'Hapus' })
        .then(function (ok) {
          if (!ok) return;
          var inactive = false;
          api('/salespeople/' + encodeURIComponent(delId), 'PUT', { is_active: inactive })
            .then(function () { toast('Sales dihapus.', 'ok'); load(); })
            .catch(function (err) { toast(err.message, 'err'); });
        });
    });

    load();
  }

  /* ------------------------------------------------------------ suppliers */
  function supplierDetailPage() {
    var info = $('#supplierDetailInfo');
    var addButton = $('#addSupplierInvoice');
    if (!info || !addButton) return;
    var supplier = info.dataset.supplier || '';

    addButton.onclick = function () {
      openModal('<button class="dialog-x" data-close type="button">&times;</button>' +
        '<small class="eyebrow">NOTA SUPPLIER</small><h2>Tambah Nota Supplier</h2>' +
        '<form id="supplierDetailBillForm" class="form">' +
        '<label class="field"><span>Supplier</span><input value="' + esc(supplier) + '" disabled></label>' +
        '<label class="field"><span>No Nota</span><input name="invoiceNo" required maxlength="80" autocomplete="off"></label>' +
        '<div class="form2"><label class="field"><span>Tanggal Nota</span><input name="invoiceDate" type="date" required></label>' +
        '<label class="field"><span>Tanggal Barang Masuk</span><input name="receivedDate" type="date" required></label></div>' +
        '<div class="form2"><label class="field"><span>Nominal</span><input name="amount" type="text" inputmode="numeric" data-money-input autocomplete="off" required></label>' +
        '<label class="field"><span>Jatuh Tempo</span><input name="dueDate" type="date" required></label></div>' +
        '<label class="field"><span>Status Pembayaran</span><select name="status"><option>Belum Lunas</option><option>Lunas</option></select></label>' +
        '<div class="form-actions"><button class="btn btn-out" data-close type="button">Batal</button><button class="btn btn-success" type="submit">Simpan Nota</button></div></form>');

      var form = $('#supplierDetailBillForm');
      form.elements.namedItem('invoiceDate').value = new Date().toISOString().slice(0, 10);
      form.elements.namedItem('receivedDate').value = new Date().toISOString().slice(0, 10);
      form.onsubmit = function (event) {
        event.preventDefault();
        var fields = event.target.elements;
        api('/api/supplier-bills', 'POST', {
          supplier: supplier,
          invoiceNo: fields.namedItem('invoiceNo').value.trim(),
          invoiceDate: fields.namedItem('invoiceDate').value,
          receivedDate: fields.namedItem('receivedDate').value,
          amount: parseMoney(fields.namedItem('amount').value),
          dueDate: fields.namedItem('dueDate').value,
          status: fields.namedItem('status').value
        }).then(function () {
          location.reload();
        }).catch(function (err) { toast(err.message, 'err'); });
      };
    };
  }

  function supplierBillsPage() {
    var body = $('#supplierBillRows');
    if (!body) return;
    var bills = [];
    var supplierNames = [];

    function filter() {
      var q = $('#supplierSearch').value.trim().toLowerCase();
      var suppliers = {};
      supplierNames.forEach(function (rawName) {
        var name = String(rawName || '').trim();
        if (name && !suppliers[name.toLowerCase()]) suppliers[name.toLowerCase()] = name;
      });
      bills.forEach(function (bill) {
        var name = String(bill.supplier || '').trim();
        var key = name.toLowerCase();
        if (name && !suppliers[key]) suppliers[key] = name;
      });
      var filtered = Object.keys(suppliers).map(function (key) { return suppliers[key]; }).filter(function (name) {
        return !q || name.toLowerCase().indexOf(q) !== -1;
      });
      body.innerHTML = filtered.map(function (name) {
        return '<tr><td><a class="supplier-name-link" href="/supplier/detail?nama=' + encodeURIComponent(name) + '">' + esc(name) + '</a></td></tr>';
      }).join('');
      $('#supplierBillEmpty').hidden = filtered.length > 0;
      $('#supplierBillEmpty').querySelector('b').textContent = Object.keys(suppliers).length ? 'Supplier tidak ditemukan' : 'Belum ada supplier';
      $('#supplierBillEmpty').querySelector('small').textContent = Object.keys(suppliers).length ? 'Coba ubah kata pencarian.' : 'Tambahkan supplier untuk mulai membuat daftar supplier.';
      $('#supplierCount').textContent = Object.keys(suppliers).length;
      $('#supplierListCount').textContent = filtered.length + ' dari ' + Object.keys(suppliers).length + ' supplier';
      var unpaid = bills.filter(function (bill) { return bill.status !== 'Lunas'; });
      var paid = bills.filter(function (bill) { return bill.status === 'Lunas'; });
      $('#supplierPayable').textContent = money(unpaid.reduce(function (sum, bill) { return sum + (Number(bill.amount) || 0); }, 0));
      $('#supplierPaid').textContent = money(paid.reduce(function (sum, bill) { return sum + (Number(bill.amount) || 0); }, 0));
      $('#supplierUnpaidCount').textContent = unpaid.length + ' nota belum lunas';
      $('#supplierPaidCount').textContent = paid.length + ' nota sudah lunas';
    }
    function editForm(bill, returnToDetail) {
      bill = bill || { supplier: '', invoiceNo: '', invoiceDate: '', receivedDate: '', amount: '', dueDate: '', status: 'Belum Lunas' };
      var supplierLocked = !bill.id && bill.lockSupplier;
      openModal('<button class="dialog-x" data-close type="button">&times;</button>' +
        '<small class="eyebrow">NOTA SUPPLIER</small><h2>' + (bill.id ? 'Ubah Nota' : 'Tambah Nota Supplier') + '</h2>' +
        '<form id="supplierBillForm" class="form">' +
        '<label class="field"><span>Supplier</span><input name="supplier" required maxlength="100" value="' + esc(bill.supplier) + '"' + (supplierLocked ? ' disabled' : '') + '></label>' +
        '<label class="field"><span>No Nota</span><input name="invoiceNo" required maxlength="80" value="' + esc(bill.invoiceNo) + '"></label>' +
        '<div class="form2"><label class="field"><span>Tanggal Nota</span><input name="invoiceDate" type="date" required value="' + esc(bill.invoiceDate) + '"></label>' +
        '<label class="field"><span>Tanggal Barang Masuk</span><input name="receivedDate" type="date" required value="' + esc(bill.receivedDate) + '"></label></div>' +
        '<div class="form2"><label class="field"><span>Nominal</span><input name="amount" type="text" inputmode="numeric" data-money-input autocomplete="off" required value="' + esc(bill.amount) + '"></label>' +
        '<label class="field"><span>Jatuh Tempo</span><input name="dueDate" type="date" required value="' + esc(bill.dueDate) + '"></label></div>' +
        '<label class="field"><span>Keterangan / Status Pembayaran</span><select name="status"><option' + (bill.status === 'Lunas' ? ' selected' : '') + '>Lunas</option><option' + (bill.status !== 'Lunas' ? ' selected' : '') + '>Belum Lunas</option></select></label>' +
        '<div class="form-actions"><button class="btn btn-out" data-close type="button">Batal</button><button class="btn btn-success" type="submit">Simpan Nota</button></div></form>');
      $('#supplierBillForm').onsubmit = function (e) {
        e.preventDefault();
        var fields = e.target.elements;
        var updatedBill = {
          id: bill.id || ('nota-' + Date.now().toString(36)),
          supplier: fields.namedItem('supplier').value.trim(),
          invoiceNo: fields.namedItem('invoiceNo').value.trim(),
          invoiceDate: fields.namedItem('invoiceDate').value,
          receivedDate: fields.namedItem('receivedDate').value,
          amount: parseMoney(fields.namedItem('amount').value),
          dueDate: fields.namedItem('dueDate').value,
          status: fields.namedItem('status').value
        };
        function saveBill() {
          api(bill.id ? '/api/supplier-bills/' + encodeURIComponent(bill.id) : '/api/supplier-bills', bill.id ? 'PATCH' : 'POST', updatedBill)
            .then(function (saved) {
              if (bill.id) bills = bills.map(function (item) { return item.id === bill.id ? saved : item; });
              else bills.push(saved);
              if (returnToDetail) {
                location.href = '/supplier/detail?nama=' + encodeURIComponent(saved.supplier);
              } else {
                closeModal(); filter(); toast('Nota supplier tersimpan selama aplikasi berjalan.', 'ok');
              }
            }).catch(function (err) { toast(err.message, 'err'); });
        }
        if (bill.id && updatedBill.status !== bill.status) {
          confirmBox({
            title: 'Konfirmasi perubahan status',
            body: 'Apakah Anda yakin ingin mengubah status nota <b>' + esc(updatedBill.invoiceNo) + '</b> dari <b>' + esc(bill.status) + '</b> menjadi <b>' + esc(updatedBill.status) + '</b>?',
            ok: 'Ya, Ubah Status',
            danger: false
          }).then(function (confirmed) { if (confirmed) saveBill(); else editForm(updatedBill, returnToDetail); });
        } else {
          saveBill();
        }
      };
    }

    function addSupplierForm() {
      openModal('<button class="dialog-x" data-close type="button">&times;</button>' +
        '<small class="eyebrow">MASTER DATA SUPPLIER</small><h2>Tambah Supplier</h2>' +
        '<form id="supplierNameForm" class="form"><label class="field"><span>Nama Supplier</span><input name="name" required maxlength="100" autocomplete="organization" placeholder="Masukkan nama supplier"></label>' +
        '<div class="form-actions"><button class="btn btn-out" data-close type="button">Batal</button><button class="btn btn-success" type="submit">Simpan Supplier</button></div></form>');
      $('#supplierNameForm').onsubmit = function (e) {
        e.preventDefault();
        var name = e.target.elements.namedItem('name').value.trim();
        api('/api/suppliers', 'POST', { name: name }).then(function (saved) {
          supplierNames.push(saved.name);
          closeModal(); filter(); toast('Supplier ditambahkan.', 'ok');
        }).catch(function (err) { toast(err.message, 'err'); });
      };
    }

    var supplierParams = new URLSearchParams(location.search);
    var initialSupplier = supplierParams.get('supplier') || '';
    var openAddForm = supplierParams.get('tambah') === '1';
    var editFromDetail = supplierParams.get('detail') === '1';
    var requestedEdit = supplierParams.get('edit') || '';
    var requestedDelete = supplierParams.get('hapus') || '';
    if (initialSupplier) $('#supplierSearch').value = initialSupplier;
    $('#addSupplier').onclick = addSupplierForm;
    $('#supplierSearch').addEventListener('input', filter);
    Promise.all([api('/api/suppliers', 'GET'), api('/api/supplier-bills', 'GET')]).then(function (result) {
      supplierNames = result[0] || [];
      bills = result[1] || []; filter();
      if (openAddForm) editForm({ supplier: initialSupplier, lockSupplier: true, invoiceNo: '', invoiceDate: '', receivedDate: '', amount: '', dueDate: '', status: 'Belum Lunas' });
      else if (requestedEdit) {
        var target = bills.filter(function (bill) { return String(bill.id) === requestedEdit; })[0];
        if (target) editForm(target, editFromDetail);
      } else if (requestedDelete) {
        var targetDelete = bills.filter(function (bill) { return String(bill.id) === requestedDelete; })[0];
        if (targetDelete) confirmBox({ title: 'Hapus nota supplier?', body: 'Nota <b>' + esc(targetDelete.invoiceNo) + '</b> akan dihapus. Apakah Anda yakin?', ok: 'Hapus' }).then(function (ok) {
          var detailURL = '/supplier/detail?nama=' + encodeURIComponent(initialSupplier);
          if (!ok) { location.href = detailURL; return; }
          api('/api/supplier-bills/' + encodeURIComponent(targetDelete.id), 'DELETE').then(function () { location.href = detailURL; }).catch(function (err) { toast(err.message, 'err'); });
        });
      }
    }).catch(function (err) { toast(err.message, 'err'); });
  }

  /* ----------------------------------------------------------- penjualan */
  function _unusedOldFormPage() {
    var salesSelect = $('#salesName');
    if (salesSelect) {
      var initialSales = $$('option', salesSelect).slice(1).map(function (option) { return option.value; });
      salesSelect.innerHTML = '<option value="">Pilih sales (opsional)</option>' + initialSales.map(function (name) { return '<option>' + esc(name) + '</option>'; }).join('');
    }
    var grid = $('#productGrid');
    if (!grid) return;
    var products = $$('.product', grid);
    var cart = [];

    products.forEach(function (p) {
      p._p = {
        id: Number(p.dataset.id) || 0,
        sku: p.dataset.sku, name: p.dataset.name, price: Number(p.dataset.price), stock: Number(p.dataset.stock),
        unit: normalizeUnit(p.dataset.unit)
      };
    });

    var discountMode = 'percent';
    function totals() {
      var sub = 0, m = 0;
      cart.forEach(function (i) { sub += i.qty * i.price; if (i.unit === 'meter') m += i.qty; });
      var percent = Math.max(0, Math.min(100, num($('#discountPercent').value)));
      var discount = discountMode === 'amount' ? parseMoney($('#discountAmount').value) : Math.round(sub * percent / 100);
      discount = Math.max(0, Math.min(sub, discount));
      var effectivePercent = sub > 0 ? discount / sub * 100 : 0;
      return { sub: sub, discountPercent: effectivePercent, discount: discount, total: Math.max(0, sub - discount), m: m };
    }

    function render() {
      var rows = $('#saleRows');
      rows.innerHTML = cart.map(function (i, ix) {
        return '<tr><td>' + (ix + 1) + '</td><td class="mono strong">' + esc(i.sku) + '</td>' +
          '<td><b>' + esc(i.name) + '</b><small class="sale-row-meta">Stok tersedia ' + qtyText(i.stock, i.unit) + '</small></td>' +
          '<td class="right"><input class="sale-row-qty" type="number" min="' + qtyMinimum(i.unit) + '" step="' + qtyStep(i.unit) + '" value="' + (i.unit === 'pcs' ? rp(i.qty) : i.qty.toFixed(1)) + '" data-qty="' + ix + '" aria-label="Jumlah ' + unitName(i.unit) + ' ' + esc(i.name) + '"></td>' +
          '<td>' + unitName(i.unit) + '</td><td class="right mono">' + money(i.price) + '</td><td class="right mono strong">' + money(i.qty * i.price) + '</td>' +
          '<td class="right"><button class="sale-remove" type="button" data-del="' + ix + '" aria-label="Hapus ' + esc(i.name) + '">Hapus</button></td></tr>';
      }).join('');
      $('#saleEmpty').hidden = cart.length !== 0;
      var t = totals();
      var count = cart.length + ' item · ' + qtySummary(cart);
      $('#count').textContent = count;
      $('#itemCount').textContent = count;
      $('#subtotal').textContent = money(t.sub);
      if (document.activeElement !== $('#discountAmount')) $('#discountAmount').value = groupedMoney(t.discount);
      if (discountMode === 'amount') $('#discountPercent').value = t.discountPercent.toFixed(1);
      $('#discountSummary').textContent = '− ' + money(t.discount);
      $('#total').textContent = money(t.total);
      $('#summaryTotal').textContent = money(t.total);
      $('#pay').disabled = !cart.length;
      $('#clear').disabled = !cart.length;
    }

    function findProduct(value) {
      var v = String(value || '').trim().toLowerCase();
      return products.filter(function (p) {
        return v === p._p.sku.toLowerCase() || v === p._p.name.toLowerCase() ||
          v === (p._p.sku + ' — ' + p._p.name).toLowerCase() || v === (p._p.sku + ' - ' + p._p.name).toLowerCase();
      })[0];
    }

    function addProduct() {
      var selected = findProduct($('#productSearch').value);
      if (!selected) {
        toast('Pilih item dari daftar kode atau nama yang tersedia.', 'warn');
        $('#productSearch').focus();
        return;
      }
      var p = selected._p;
      var qty = normalizeQty(num($('#quantity').value), p.unit);
      if (qty < qtyMinimum(p.unit)) {
        toast(p.unit === 'pcs' ? 'Jumlah minimal 1 pcs.' : 'Jumlah minimal 0,1 meter.', 'warn');
        $('#quantity').focus();
        return;
      }
      var line = cart.filter(function (i) { return i.sku === p.sku; })[0];
      var next = (line ? line.qty : 0) + qty;
      if (next > p.stock) {
        toast('Stok ' + p.name + ' hanya ' + qtyText(p.stock, p.unit) + '.', 'warn');
        return;
      }
      if (line) line.qty = next; else cart.push({ sku: p.sku, name: p.name, price: p.price, stock: p.stock, qty: qty, unit: p.unit });
      $('#productSearch').value = '';
      $('#quantity').value = '1.0';
      $('#quantity').step = '0.1';
      $('#quantity').min = '0.1';
      $('#quantityLabel').textContent = 'Jumlah (meter)';
      $('#productHint').textContent = 'Pilih item untuk melihat harga dan stok yang tersedia.';
      render();
      $('#productSearch').focus();
    }

    $('#productSearch').addEventListener('input', function () {
      var p = findProduct(this.value);
      if (p) {
        $('#quantity').step = qtyStep(p._p.unit);
        $('#quantity').min = String(qtyMinimum(p._p.unit));
        $('#quantityLabel').textContent = 'Jumlah (' + unitName(p._p.unit) + ')';
        $('#productHint').textContent = p._p.name + ' · ' + money(p._p.price) + '/' + unitName(p._p.unit) + ' · ' + qtyText(p._p.stock, p._p.unit) + ' tersedia';
      } else {
        $('#quantity').step = '0.1';
        $('#quantity').min = '0.1';
        $('#quantityLabel').textContent = 'Jumlah (meter)';
        $('#productHint').textContent = 'Pilih item dari daftar untuk melihat harga dan stok.';
      }
    });
    $('#productSearch').addEventListener('keydown', function (e) {
      if (e.key === 'Enter') { e.preventDefault(); addProduct(); }
    });
    $('#addItem').onclick = addProduct;

    $('#saleRows').addEventListener('click', function (e) {
      var del = e.target.closest('[data-del]');
      if (del) {
        cart.splice(Number(del.dataset.del), 1);
        render();
      }
    });

    $('#saleRows').addEventListener('change', function (e) {
      var ix = e.target.dataset.qty;
      if (ix == null) return;
      var qty = normalizeQty(num(e.target.value), cart[ix].unit);
      if (qty < qtyMinimum(cart[ix].unit)) {
        toast(cart[ix].unit === 'pcs' ? 'Jumlah minimal 1 pcs.' : 'Jumlah minimal 0,1 meter.', 'warn');
        qty = qtyMinimum(cart[ix].unit);
      }
      if (qty > cart[ix].stock) {
        toast('Stok ' + cart[ix].name + ' hanya ' + qtyText(cart[ix].stock, cart[ix].unit) + '.', 'warn');
        qty = cart[ix].stock;
      }
      cart[ix].qty = qty;
      render();
    });

    $('#clear').onclick = function () {
      cart = [];
      render();
      toast('Item transaksi dikosongkan.', 'warn');
    };

    var now = new Date();
    $('#saleDate').value = new Intl.DateTimeFormat('id-ID', { dateStyle: 'medium', timeStyle: 'short' }).format(now);

    var payModal = $('#payModal'), method = 'Cash';

    $('#discountPercent').addEventListener('input', function () {
      discountMode = 'percent';
      if (num(this.value) > 100) this.value = '100';
      if (this.value !== '' && num(this.value) < 0) this.value = '0';
      render();
      if (!payModal.hidden) change();
    });

    $('#discountAmount').addEventListener('input', function () {
      discountMode = 'amount';
      render();
      if (!payModal.hidden) change();
    });
    $('#discountAmount').addEventListener('blur', render);

    function change() {
      var total = totals().total;
      var paid = parseMoney($('#cash').value);
      var diff = paid - total;
      var box = $('.change', payModal);
      var isDP = method === 'DP';
      var valid = isDP ? paid > 0 && paid <= total : paid >= total;
      box.classList.toggle('short', !valid);
      box.querySelector('b').textContent = isDP ? money(Math.max(total - paid, 0)) : money(Math.max(diff, 0));
      box.querySelector('span').textContent = isDP ? 'Sisa setelah DP' : (diff < 0 ? 'Kurang bayar' : 'Kembalian');
      $('#done').disabled = !valid;
      $('#done').textContent = isDP ? 'Simpan DP · Bill tetap pending' : 'Selesaikan dan tutup bill';
    }

    $('#pay').onclick = function () {
      var total = totals().total;
      $('#payTotal').textContent = money(total);
      $('#cash').value = method === 'DP' ? '' : groupedMoney(total);
      $('#cashField').hidden = false;
      change();
      payModal.hidden = false;
      if (method === 'Cash') $('#cash').focus();
    };

    $('#payClose').onclick = function () { payModal.hidden = true; };

    $('#methods').onclick = function (e) {
      var b = e.target.closest('button[data-pay]');
      if (!b) return;
      method = b.dataset.pay;
      $$('#methods button').forEach(function (x) { x.classList.toggle('is-active', x === b); });
      $('#cashField').hidden = false;
      $('#cash').value = method === 'DP' ? '' : groupedMoney(totals().total);
      change();
    };

    $('#cash').addEventListener('input', change);

    $('#done').onclick = function () {
      var t = totals();
      var paid = parseMoney($('#cash').value) || t.total;
      var isDP = method === 'DP';
      confirmBox({
        title: 'Konfirmasi Transaksi',
        body: cart.length + ' item · Total <b>' + money(t.total) + '</b>' +
          (t.discount > 0 ? ' (diskon ' + money(t.discount) + ')' : '') +
          '<br>Bayar: <b>' + money(paid) + '</b> · Metode: <b>' + esc(method) + '</b>' +
          (isDP ? '<br><span style="color:var(--amber)">⚠ Bill akan tersimpan sebagai pending.</span>' : ''),
        ok: 'Simpan Transaksi'
      }).then(function (confirmed) {
        if (!confirmed) return;
        var body = {
          customer: $('#customer').value.trim(),
          sales: $('#salesName').value,
          note: $('#saleNote').value.trim(),
          discountPercent: t.discountPercent,
          discountAmount: t.discount,
          payment: method,
          paid: paid,
          items: cart.map(function (i) { return { product_id: i.id, qty: i.qty }; })
        };
      api('/api/sales', 'POST', body).then(function (res) {
        var receiptItems = cart.map(function (item) { return { sku: item.sku, name: item.name, qty: item.qty, unit: item.unit, price: item.price }; });
        var receiptDate = new Date();
        var receiptRows = receiptItems.map(function (item) {
          return '<div class="thermal-item"><b>' + esc(item.name) + '</b><small>' + esc(item.sku) + '</small><div class="thermal-pair"><span>' + qtyText(item.qty, item.unit) + ' × ' + money(item.price) + '</span><b>' + money(item.qty * item.price) + '</b></div></div>';
        }).join('');
        var receiptMarkup = '<section class="thermal-sheet">' + thermalHeader('RESI PENJUALAN', receiptDate.toLocaleString('id-ID')) +
          '<div class="thermal-receipt-meta"><div><span>No. transaksi</span><b>' + esc(res.no) + '</b></div>' +
          '<div><span>Pelanggan</span><b>' + esc(body.customer || 'Pelanggan umum') + '</b></div>' +
          '<div><span>Sales</span><b>' + esc(body.sales || '-') + '</b></div>' +
          '<div><span>Kasir</span><b>ADMIN</b></div></div>' +
          '<div class="thermal-entries">' + receiptRows + '</div>' +
          '<div class="thermal-summary">' + (res.discountAmount > 0 ? '<div class="thermal-pair"><span>Subtotal</span><b>' + money(res.subtotal) + '</b></div><div class="thermal-pair"><span>Diskon (' + esc(res.discountPercent) + '%)</span><b>− ' + money(res.discountAmount) + '</b></div>' : '') +
          '<div class="thermal-pair thermal-total"><span>TOTAL</span><b>' + money(res.total) + '</b></div>' +
          '<div class="thermal-pair"><span>Metode</span><b>' + esc(method) + '</b></div>' +
          '<div class="thermal-pair"><span>Dibayar</span><b>' + money(res.paid) + '</b></div>' +
          (res.pending ? '<div class="thermal-pair thermal-pending"><span>BELUM LUNAS · SISA</span><b>' + money(res.balance) + '</b></div>' : '<div class="thermal-pair"><span>Kembalian</span><b>' + money(res.change) + '</b></div>') +
          '</div><footer class="thermal-footer">' + (res.pending ? 'Simpan resi ini untuk pelunasan.' : 'Terima kasih telah berbelanja.') + '<br>Barang yang sudah dibeli tidak dapat dikembalikan.</footer></section>';
        payModal.hidden = true;
        cart = [];
        discountMode = 'percent';
        $('#discountPercent').value = '0';
        $('#discountAmount').value = '0';
        render();
        var html =
          '<button class="dialog-x" data-close type="button">&times;</button>' +
          '<small class="eyebrow">STRUK</small><h2>' + esc(res.no) + '</h2>' +
          '<div class="big" style="font-size:22px">' + money(res.total) + '</div>' +
          '<div class="change ' + (res.pending ? 'short' : '') + '"><span>' + (res.pending ? 'DP tercatat · Sisa tagihan' : 'Lunas · Kembalian') + '</span><b class="mono">' + (res.pending ? money(res.balance) : money(res.change)) + '</b></div>' +
          '<p>' + (res.pending ? 'Bill tersimpan di riwayat sebagai pending pembayaran. Buka bill tersebut untuk menambah barang atau menerima pelunasan.' : 'Pembayaran diterima dan bill ditutup.') + '</p>' +
          '<div class="form-actions"><button class="btn btn-out" data-close type="button">Tutup</button>' +
          '<button class="btn btn-success" id="rp2" type="button">Pratinjau Resi</button>' +
          '<a class="btn btn-out" href="/kasir">Lihat Riwayat Penjualan</a></div>';
        openModal(html);
        $('#rp2').onclick = function () { thermalPreview('Resi penjualan', receiptMarkup); };
        toast('Transaksi tersimpan.', 'ok');
      }).catch(function (err) { toast(err.message, 'err'); });
      }); // end confirmBox
    };

    render();
  }

  /* --------------------------------------------------------------- opname */
  function opnamePage() {
    var rows = $$('#opRows tr.grow');
    if (!rows.length) return;
    var state = { per: 10 };

    function calc(r) {
      var sys = Number(r.dataset.stock) || 0;
      var inn = num(r.querySelector('[data-k="in"]').value);
      var out = num(r.querySelector('[data-k="out"]').value);
      var res = r.querySelector('[data-k="res"]');
      var note = r.querySelector('[data-k="note"]');
      var diff = res.value === '' ? 0 : num(res.value) + inn - out - sys;
      r.querySelector('[data-c="diff"]').textContent = (diff > 0 ? '+' : '') + qtyText(diff, r.dataset.unit);
      r.querySelector('[data-c="value"]').textContent = money(Math.abs(diff) * (Number(r.dataset.cost) || 0));
      res.classList.toggle('pos', diff > 0);
      res.classList.toggle('neg', diff < 0);
      return { counted: res.value !== '', diff: diff, res: res.value === '' ? 0 : num(res.value) + inn - out, sys: sys };
    }

    function filter() {
      var q = $('#opSearch').value.trim().toLowerCase();
      var list = rows.filter(function (r) {
        return !q || r.dataset.hay.toLowerCase().indexOf(q) > -1;
      });
      var pages = Math.max(1, Math.ceil(list.length / state.per));
      var shown = list.slice(0, state.per);
      rows.forEach(function (r) { r.hidden = shown.indexOf(r) === -1; });
      $('#opNoData').hidden = list.length !== 0;
      $('#opCountInfo').textContent = list.length + ' item · ' + shown.length + ' ditampilkan';
    }

    function summary() {
      var counted = 0, diffVal = 0;
      var diffs = [], quantities = [];
      rows.forEach(function (r) {
        var res = r.querySelector('[data-k="res"]');
        var inn = num(r.querySelector('[data-k="in"]').value);
        var out = num(r.querySelector('[data-k="out"]').value);
        var sys = Number(r.dataset.stock) || 0;
        var actual = res.value === '' ? 0 : num(res.value) + inn - out;
        if (res.value !== '') {
          counted++;
          diffs.push({ qty: actual - sys, unit: r.dataset.unit });
          diffVal += Math.abs(actual - sys) * (Number(r.dataset.cost) || 0);
        }
        quantities.push({ qty: actual, unit: r.dataset.unit });
      });
      $('#opCounted').textContent = counted + ' / ' + rows.length;
      $('#opDiff').textContent = qtySummary(diffs);
      $('#opDiffValue').textContent = money(diffVal);
      $('#opSumMeter').textContent = qtySummary(quantities);
      var pct = rows.length ? Math.round((counted / rows.length) * 100) : 0;
      $('#opBar').style.width = pct + '%';
      $('#opProgress').textContent = pct + '% selesai';
      $('#opDiff').style.color = diffs.some(function (item) { return item.qty < 0; }) ? 'var(--red)' : 'var(--green)';
    }

    $('#opRows').addEventListener('input', function (e) {
      var r = e.target.closest('tr');
      if (!r) return;
      calc(r);
      summary();
    });

    $('#opSearch').addEventListener('input', filter);
    $('#opPerPage').onchange = function () { state.per = num(this.value); filter(); };

    $('#opAddRow').onclick = function () { toast('Baris kosong ditambahkan di sesi berikutnya.', 'warn'); };
    $('#opReset').onclick = function () {
      confirmBox({ title: 'Reset sesi opname?', body: 'Semua hasil hitung pada sesi ini akan dikosongkan.', ok: 'Reset' })
        .then(function (ok) {
          if (!ok) return;
          rows.forEach(function (r) {
            $$('input', r).forEach(function (i) { i.value = ''; i.classList.remove('pos', 'neg'); });
            calc(r);
          });
          summary();
          toast('Sesi opname direset.', 'warn');
        });
    };
    $('#opSave').onclick = function () { toast('Sesi opname disimpan lokal.', 'ok'); };
    $('#opClose').onclick = function () {
      confirmBox({ title: 'Selesaikan opname?', body: 'Pastikan hasil hitung sudah sesuai sebelum menutup sesi.', ok: 'Selesai', danger: false })
        .then(function (ok) { if (ok) toast('Sesi opname ditutup.', 'ok'); });
    };

    var clock = $('#opClock');
    if (clock) setInterval(function () {
      var d = new Date();
      clock.textContent = [d.getHours(), d.getMinutes(), d.getSeconds()]
        .map(function (n) { return String(n).padStart(2, '0'); }).join(':');
    }, 1000);

    rows.forEach(calc);
    filter();
    summary();
  }

  /* --------------------------------------------------------- stock transfers */
  function stockTransferPage() {
    var page = $('.transfer-page');
    if (!page) return;
    var direction = page.dataset.transferType;
    var stores = [];
    api('/stores', 'GET').then(function (data) {
      stores = Array.isArray(data) ? data : [];
      var fromSel = $('#fromStore');
      var toSel = $('#toStore');
      if (fromSel) {
        stores.forEach(function (s) {
          var opt = document.createElement('option');
          opt.value = s.Name || s.name;
          opt.textContent = s.Name || s.name;
          fromSel.appendChild(opt);
        });
      }
      if (toSel) {
        stores.forEach(function (s) {
          var opt = document.createElement('option');
          opt.value = s.Name || s.name;
          opt.textContent = s.Name || s.name;
          toSel.appendChild(opt);
        });
      }
    }).catch(function () {});
    var products = $$('#transferProductCatalog .transfer-product-option').map(function (el) {
      return { id: Number(el.dataset.id) || 0, sku: el.dataset.sku, name: el.dataset.name, stock: Number(el.dataset.stock), storeStocks: [], unit: normalizeUnit(el.dataset.unit) };
    });
    var items = [];

    // Sumber data item: API Daftar Item (/api/items), sama seperti menu Daftar Item.
    function loadCatalogFromApi() {
      return api('/api/items?limit=500', 'GET').then(function (result) {
        var list = result && Array.isArray(result.data) ? result.data : [];
        if (!list.length) return;
        products = list.filter(function (p) { return p.is_active !== false; }).map(function (p) {
          var storeStocks = Array.isArray(p.stock) ? p.stock : [];
          var total = 0;
          storeStocks.forEach(function (s) { total += Number(s.qty) || 0; });
          return {
            id: Number(p.id) || 0,
            sku: p.code || '',
            name: p.name || '',
            stock: total,
            storeStocks: storeStocks,
            unit: normalizeUnit(p.unit)
          };
        });
        var dl = $('#transferProductOptions');
        if (dl) {
          dl.innerHTML = products.map(function (p) {
            return '<option value="' + esc(p.sku + ' — ' + p.name) + '"></option>';
          }).join('');
        }
      }).catch(function () { /* pakai katalog hasil render server sebagai fallback */ });
    }
    loadCatalogFromApi();

    // Stok mengikuti toko asal yang dipilih.
    function stockFor(p) {
      var from = ($('#fromStore') && $('#fromStore').value.trim()) || '';
      if (from && p.storeStocks && p.storeStocks.length) {
        var match = p.storeStocks.filter(function (s) {
          return String(s.store_name || '').toLowerCase() === from.toLowerCase();
        })[0];
        if (match) return Number(match.qty) || 0;
      }
      return p.stock;
    }

    function findProduct(value) {
      var v = String(value || '').trim().toLowerCase();
      return products.filter(function (p) {
        return v === p.sku.toLowerCase() || v === p.name.toLowerCase() ||
          v === (p.sku + ' — ' + p.name).toLowerCase() || v === (p.sku + ' - ' + p.name).toLowerCase();
      })[0];
    }

    function render() {
      $('#transferItems').innerHTML = items.map(function (item, ix) {
        return '<tr><td>' + (ix + 1) + '</td><td class="mono strong">' + esc(item.sku) + '</td><td><b>' + esc(item.name) + '</b></td>' +
          '<td class="right"><input class="transfer-row-qty" type="number" min="' + qtyMinimum(item.unit) + '" step="' + qtyStep(item.unit) + '" value="' + (item.unit === 'pcs' ? rp(item.qty) : item.qty.toFixed(1)) + '" data-transfer-qty="' + ix + '"></td><td>' + unitName(item.unit) + '</td>' +
          '<td class="right mono">' + qtyText(item.stock, item.unit) + '</td><td class="right"><button class="sale-remove" type="button" data-transfer-del="' + ix + '">Hapus</button></td></tr>';
      }).join('');
      $('#transferEmpty').hidden = items.length > 0;
      $('#transferCount').textContent = items.length + ' item';
      $('#transferTotal').textContent = qtySummary(items);
      $('#transferSave').disabled = items.length === 0 || !$('#fromStore').value.trim() || !$('#toStore').value.trim();
    }

    $('#transferProduct').addEventListener('input', function () {
      var p = findProduct(this.value);
      $('#transferHint').textContent = p
        ? p.name + ' · Stok saat ini ' + qtyText(stockFor(p), p.unit) + (direction === 'out' ? ' · Jumlah keluar maksimal sebesar stok.' : '')
        : 'Jumlah mengikuti satuan barang. Stok tersedia akan diperiksa sebelum transaksi disimpan.';
      if (p) {
        $('#transferQty').step = qtyStep(p.unit);
        $('#transferQty').min = String(qtyMinimum(p.unit));
        $('#transferQtyLabel').textContent = 'Jumlah (' + unitName(p.unit) + ')';
      }
    });
    $('#fromStore').addEventListener('change', function () {
      items.forEach(function (item) {
        var p = products.filter(function (x) { return x.sku === item.sku; })[0];
        if (p) item.stock = stockFor(p);
      });
      render();
    });
    $('#toStore').addEventListener('change', render);
    $('#transferProduct').addEventListener('keydown', function (e) {
      if (e.key === 'Enter') { e.preventDefault(); $('#transferAdd').click(); }
    });
    $('#transferAdd').onclick = function () {
      var product = findProduct($('#transferProduct').value);
      if (!product) { toast('Pilih barang dari daftar yang tersedia.', 'warn'); $('#transferProduct').focus(); return; }
      var qty = normalizeQty(num($('#transferQty').value), product.unit);
      if (qty < qtyMinimum(product.unit)) { toast(product.unit === 'pcs' ? 'Jumlah minimum 1 pcs.' : 'Jumlah minimum 0,1 meter.', 'warn'); $('#transferQty').focus(); return; }
      var available = stockFor(product);
      var existing = items.filter(function (item) { return item.sku === product.sku; })[0];
      if (direction === 'out' && (existing ? existing.qty : 0) + qty > available) {
        toast('Stok ' + product.name + ' hanya ' + qtyText(available, product.unit) + '.', 'warn');
        return;
      }
      if (existing) existing.qty += qty;
      else items.push({ id: product.id, sku: product.sku, name: product.name, stock: available, qty: qty, unit: product.unit });
      $('#transferProduct').value = '';
      $('#transferQty').value = '1.0';
      $('#transferQty').step = '0.1';
      $('#transferQty').min = '0.1';
      $('#transferQtyLabel').textContent = 'Jumlah (meter)';
      render();
      $('#transferProduct').focus();
    };
    $('#transferItems').addEventListener('click', function (e) {
      var button = e.target.closest('[data-transfer-del]');
      if (button) { items.splice(Number(button.dataset.transferDel), 1); render(); }
    });
    $('#transferItems').addEventListener('change', function (e) {
      var ix = e.target.dataset.transferQty;
      if (ix == null) return;
      var qty = normalizeQty(num(e.target.value), items[ix].unit);
      if (qty < qtyMinimum(items[ix].unit)) { toast(items[ix].unit === 'pcs' ? 'Jumlah minimum 1 pcs.' : 'Jumlah minimum 0,1 meter.', 'warn'); qty = qtyMinimum(items[ix].unit); }
      if (direction === 'out' && qty > items[ix].stock) {
        toast('Stok ' + items[ix].name + ' hanya ' + qtyText(items[ix].stock, items[ix].unit) + '.', 'warn');
        qty = items[ix].stock;
      }
      items[ix].qty = qty;
      render();
    });
    $('#transferSave').onclick = function () {
      api('/api/stock-transfers', 'POST', {
        type: direction,
        fromStore: $('#fromStore').value.trim(),
        toStore: $('#toStore').value.trim(),
        note: $('#transferNote').value.trim(),
        items: items.map(function (item) { return { product_id: item.id, qty: item.qty }; })
      }).then(function (result) {
        toast('Perpindahan stok ' + (result.transfer_no || result.no) + ' berhasil disimpan.', 'ok');
        setTimeout(function () { location.reload(); }, 500);
      }).catch(function (err) { toast(err.message, 'err'); });
    };
    $('#printTransfers').onclick = function () {
      var historyRows = $$('tbody tr', $('.transfer-history')).filter(function (row) { return row.children.length > 1; });
      var historyQuantities = [];
      var entries = historyRows.map(function (row) {
        var cells = row.children;
        var itemRows = $$('.sale-detail-panel tbody tr', row).map(function (itemRow) {
          var itemCells = itemRow.children;
          var qty = num(itemCells[2].textContent.replace(',', '.'));
          var unit = itemCells[2].textContent.toLowerCase().indexOf('pcs') !== -1 ? 'pcs' : 'meter';
          historyQuantities.push({ qty: qty, unit: unit });
          return '<div class="thermal-pair"><span>' + esc(itemCells[1].textContent.trim()) + ' · ' + qtyText(qty, unit) + '</span><b>' + esc(itemCells[0].textContent.trim()) + '</b></div>';
        }).join('');
        return '<article class="thermal-entry"><div class="thermal-entry-top"><b>' + esc(cells[0].textContent.trim()) + '</b><span>' + esc(cells[1].textContent.trim()) + '</span></div>' +
          '<div>' + esc(cells[2].textContent.trim()) + '</div><div class="thermal-items-list">' + (itemRows || '<div>' + esc(cells[3].textContent.trim()) + '</div>') + '</div>' +
          (cells[5].textContent.trim() ? '<small>' + esc(cells[5].textContent.trim()) + '</small>' : '') + '</article>';
      }).join('');
      var directionLabel = direction === 'in' ? 'BARANG MASUK' : 'BARANG KELUAR';
      thermalPreview('Riwayat ' + (direction === 'in' ? 'barang masuk' : 'barang keluar'),
        '<section class="thermal-sheet">' + thermalHeader('RIWAYAT ' + directionLabel, new Date().toLocaleDateString('id-ID')) +
        '<div class="thermal-summary"><div class="thermal-pair"><span>Jumlah transaksi</span><b>' + historyRows.length + '</b></div>' +
        '<div class="thermal-pair"><span>Total perpindahan</span><b>' + qtySummary(historyQuantities) + '</b></div></div>' +
        '<div class="thermal-entries">' + (entries || '<p class="thermal-empty">Belum ada riwayat.</p>') + '</div>' +
        '<footer class="thermal-footer">Dicetak ' + esc(new Date().toLocaleString('id-ID')) + '</footer></section>');
    };
    render();
  }

  function commissionReportPage() {
    if (!$('#commissionReport')) return;

    function todayISO() {
      var d = new Date();
      return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0');
    }
    function monthStartISO() {
      var d = new Date();
      return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-01';
    }
    var today = todayISO();
    var monthStart = monthStartISO();
    $('#commissionFrom').value = monthStart;
    $('#commissionTo').value = today;

    api('/salespeople', 'GET').then(function (data) {
      var sel = $('#commissionSales');
      (Array.isArray(data) ? data : []).filter(function (s) { return s.is_active; }).forEach(function (s) {
        var o = document.createElement('option');
        o.value = s.id; o.textContent = s.name;
        sel.appendChild(o);
      });
    }).catch(function () {});

    function fetchAndRender() {
      var from = $('#commissionFrom').value || monthStart;
      var to = $('#commissionTo').value || today;
      var spId = $('#commissionSales').value;
      var params = 'date_from=' + encodeURIComponent(from) + '&date_to=' + encodeURIComponent(to);
      if (spId) params += '&salesperson_id=' + encodeURIComponent(spId);

      api('/api/commission?' + params, 'GET').then(function (result) {
        var summary = result.summary || [];
        var details = result.details || [];

        var totalTx = summary.reduce(function (s, r) { return s + r.tx_count; }, 0);
        var totalSalesVal = summary.reduce(function (s, r) { return s + Number(r.total_sales); }, 0);
        var totalComm = summary.reduce(function (s, r) { return s + Number(r.commission_earned); }, 0);
        $('#commissionCount').textContent = String(totalTx);
        $('#commissionSalesTotal').textContent = money(totalSalesVal);
        $('#commissionTotal').textContent = money(totalComm);

        $('#commissionSalesSummary').innerHTML = summary.length
          ? summary.map(function (r) {
              return '<tr>' +
                '<td>' + esc(r.salesperson_name) + '</td>' +
                '<td class="right">' + r.tx_count + '</td>' +
                '<td class="right mono">' + money(Number(r.total_sales)) + '</td>' +
                '<td class="right">' + meter(Number(r.commission_rate)) + '%</td>' +
                '<td class="right mono strong">' + money(Number(r.commission_earned)) + '</td>' +
                '</tr>';
            }).join('')
          : '<tr><td colspan="5" class="muted">Tidak ada data komisi pada periode ini.</td></tr>';

        $('#commissionRows').innerHTML = details.length
          ? details.map(function (d) {
              var date = new Date(d.created_at);
              return '<tr>' +
                '<td class="mono">' + esc(date.toLocaleDateString('id-ID')) + '</td>' +
                '<td class="mono strong">' + esc(d.order_no) + '</td>' +
                '<td>' + esc(d.sales_name || '—') + '</td>' +
                '<td>' + esc(d.customer_name || '—') + '</td>' +
                '<td class="mono">' + esc(d.product_code) + '</td>' +
                '<td>' + esc(d.product_name) + '</td>' +
                '<td class="right">' + qtyText(Number(d.qty), d.unit) + '</td>' +
                '<td class="right">' + meter(Number(d.commission_rate)) + '%</td>' +
                '<td class="right mono">' + money(Number(d.commission_value)) + '</td>' +
                '</tr>';
            }).join('')
          : '<tr><td colspan="9" class="muted">Tidak ada rincian komisi pada periode ini.</td></tr>';
      }).catch(function (err) { toast(err.message || 'Gagal memuat data komisi.', 'err'); });
    }

    $('#commissionApply').onclick = fetchAndRender;
    $('#commissionReset').onclick = function () {
      $('#commissionFrom').value = monthStart;
      $('#commissionTo').value = today;
      $('#commissionSales').value = '';
      fetchAndRender();
    };

    fetchAndRender();
  }

  /* ------------------------------------------------------------- stok global */
  function globalStockPage() {
    if (!$('#gsTable')) return;

    var allProducts = [];
    var stores = [];
    var filteredStoreId = '';
    var searchQ = '';
    var statusFilter = '';

    Promise.all([
      api('/stores', 'GET'),
      api('/api/items?limit=500', 'GET')
    ]).then(function (results) {
      stores = Array.isArray(results[0]) ? results[0] : [];
      var raw = Array.isArray(results[1]) ? results[1] : (results[1].data || []);
      allProducts = raw.map(function (p) {
        // Find stock at this product's home store
        var homeStock = 0;
        (p.stock || []).forEach(function (s) {
          if (p.store_id && s.store_id === p.store_id) homeStock = Number(s.qty);
        });
        return {
          id: p.id,
          code: p.code || '',
          name: p.name || '',
          category: p.category_name || '',
          unit: p.unit || '',
          minStock: Number(p.min_stock) || 0,
          isActive: p.is_active !== false,
          storeId: p.store_id,
          storeName: p.store_name || '',
          qty: homeStock
        };
      });

      var sel = $('#gsStore');
      stores.forEach(function (s) {
        var o = document.createElement('option');
        o.value = s.ID; o.textContent = s.Name;
        sel.appendChild(o);
      });

      render();
    }).catch(function (err) { toast(err.message || 'Gagal memuat stok.', 'err'); });

    function render() {
      $('#gsTableHead').innerHTML = '<tr>' +
        '<th>Kode Item</th><th>Nama Item</th><th>Toko</th><th>Jenis</th>' +
        '<th class="right">Stok</th><th>Satuan</th><th class="right">Min. Stok</th>' +
        '</tr>';

      var list = allProducts.filter(function (p) {
        if (filteredStoreId && String(p.storeId) !== filteredStoreId) return false;
        if (!p.isActive && statusFilter === 'active') return false;
        if (searchQ && p.code.toLowerCase().indexOf(searchQ) === -1 && p.name.toLowerCase().indexOf(searchQ) === -1) return false;
        if (statusFilter === 'low') {
          if (p.minStock <= 0 || p.qty >= p.minStock) return false;
        }
        return true;
      });

      $('#gsNodata').hidden = list.length !== 0;
      $('#gsCount').textContent = list.length + ' dari ' + allProducts.length + ' item';

      if (!list.length) {
        $('#gsTableBody').innerHTML = '<tr><td colspan="7" class="muted" style="text-align:center;padding:24px">Tidak ada item ditemukan.</td></tr>';
        return;
      }

      $('#gsTableBody').innerHTML = list.map(function (p) {
        var isLow = p.minStock > 0 && p.qty < p.minStock;
        return '<tr>' +
          '<td class="mono strong">' + esc(p.code) + '</td>' +
          '<td class="strong">' + esc(p.name) +
            (isLow ? ' <span class="chip chip-low" style="font-size:10px;padding:1px 6px;vertical-align:middle">Rendah</span>' : '') +
            (!p.isActive ? ' <span class="chip chip-soft" style="font-size:10px;padding:1px 6px;vertical-align:middle">Nonaktif</span>' : '') +
          '</td>' +
          '<td><span class="chip chip-soft">' + esc(p.storeName || '—') + '</span></td>' +
          '<td><span class="chip chip-soft">' + esc(p.category || 'Umum') + '</span></td>' +
          '<td class="right mono strong"' + (isLow ? ' style="color:#c62828"' : '') + '>' + qtyText(p.qty, p.unit) + '</td>' +
          '<td>' + esc(p.unit) + '</td>' +
          '<td class="right mono">' + (p.minStock > 0 ? qtyText(p.minStock, p.unit) : '<span class="muted">—</span>') + '</td>' +
          '</tr>';
      }).join('');
    }

    $('#gsSearch').addEventListener('input', function () { searchQ = this.value.trim().toLowerCase(); render(); });
    $('#gsStore').addEventListener('change', function () { filteredStoreId = this.value; render(); });
    $('#gsStatus').addEventListener('change', function () { statusFilter = this.value; render(); });
  }

  /* ------------------------------------------------------------------ login */
  function loginPage() {
    var form = $('#loginForm');
    if (!form) return;
    var btn = $('#loginBtn');
    var errBox = $('#loginError');

    form.onsubmit = function (e) {
      e.preventDefault();
      var username = $('#loginUsername').value.trim();
      var password = $('#loginPassword').value;
      if (!username || !password) return;

      btn.disabled = true;
      btn.textContent = 'Memproses…';
      errBox.hidden = true;

      api('/auth/login', 'POST', { username: username, password: password })
        .then(function () {
          var next = new URLSearchParams(location.search).get('next') || '/dashboard';
          location.href = next;
        })
        .catch(function (err) {
          errBox.textContent = err.message || 'Nama pengguna atau kata sandi salah.';
          errBox.hidden = false;
          btn.disabled = false;
          btn.textContent = 'Masuk';
          $('#loginPassword').value = '';
          $('#loginPassword').focus();
        });
    };
  }

  /* ------------------------------------------------------------------ logout */
  function wireLogout() {
    var btn = $('#logoutBtn');
    if (!btn) return;
    btn.onclick = function () {
      api('/auth/logout', 'POST')
        .finally(function () {
          location.href = '/login';
        });
    };
  }

  /* ------------------------------------------------------------------ boot */
  document.addEventListener('DOMContentLoaded', function () {
    clearOldBrowserData();
    document.addEventListener('input', function (event) {
      var input = event.target;
      if (!input.matches || !input.matches('[data-money-input]')) return;
      var cursor = input.selectionStart || 0;
      var digitsBefore = input.value.slice(0, cursor).replace(/\D/g, '').length;
      input.value = groupedMoney(input.value);
      var nextCursor = 0, seen = 0;
      while (nextCursor < input.value.length && seen < digitsBefore) {
        if (/\d/.test(input.value.charAt(nextCursor))) seen++;
        nextCursor++;
      }
      input.setSelectionRange(nextCursor, nextCursor);
    }, true);
    var page = document.body.dataset.page;
    if (page === 'login') {
      loginPage();
      return;
    }
    try {
      chrome();
      wireLogout();
    } catch (err) {
      console.error('chrome', err);
    }
    try {
      if (page === 'kasir') kasirPage();
      else if (page === 'penjualan') penjualanPage();
      else if (page === 'void-orders') voidOrdersPage();
      else if (page === 'stok-global') globalStockPage();
      else if (page === 'transaksi') transaksiPage();
      else if (page === 'items') itemsPage();
      else if (page === 'suppliers') supplierBillsPage();
      else if (page === 'supplier-detail') supplierDetailPage();
      else if (page === 'opname') opnamePage();
      else if (page === 'stock-in' || page === 'stock-out' || page === 'stock-transfer') stockTransferPage();
      else if (page === 'datasheet') datasheetPage();
      else if (page === 'admin-users') adminUsersPage();
      else if (page === 'salespeople') salespeoplePage();
      else if (page === 'komisi-sales') commissionReportPage();
    } catch (err) {
      console.error(err);
      toast('Terjadi kesalahan pada halaman ini.', 'err');
    }
  });
})();
