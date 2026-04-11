/* ─── Card number formatting ─────────────────────────────────────────────── */
const cardInput = document.getElementById('card_number');
const expiryInput = document.getElementById('expiry');
const cardNumberPreview = document.querySelector('.card-preview__number');
const cardExpPreview = document.getElementById('card-exp-preview');

if (cardInput) {
  cardInput.addEventListener('input', () => {
    let val = cardInput.value.replace(/\D/g, '').slice(0, 16);
    cardInput.value = val.match(/.{1,4}/g)?.join(' ') ?? val;
    if (cardNumberPreview) {
      const padded = val.padEnd(16, '•');
      cardNumberPreview.textContent = padded.match(/.{1,4}/g).join(' ');
    }
  });
}

if (expiryInput) {
  expiryInput.addEventListener('input', () => {
    let val = expiryInput.value.replace(/\D/g, '').slice(0, 4);
    if (val.length >= 3) val = val.slice(0, 2) + '/' + val.slice(2);
    expiryInput.value = val;
    if (cardExpPreview) cardExpPreview.textContent = val || 'MM/YY';
  });
}

/* ─── Checkout form loading state ────────────────────────────────────────── */
const checkoutForm = document.getElementById('checkout-form');
const payBtn = document.getElementById('pay-btn');

if (checkoutForm && payBtn) {
  checkoutForm.addEventListener('submit', (e) => {
    payBtn.disabled = true;
    payBtn.innerHTML = '⏳ Обработка...';
  });
}

/* ─── Auto-submit status selects (admin) ─────────────────────────────────── */
document.querySelectorAll('.inline-form select').forEach(sel => {
  sel.addEventListener('change', () => sel.closest('form').submit());
});

/* ─── Qty auto-submit on cart ────────────────────────────────────────────── */
document.querySelectorAll('.qty-form input[type=number]').forEach(input => {
  let timer;
  input.addEventListener('change', () => {
    clearTimeout(timer);
    timer = setTimeout(() => input.closest('form').submit(), 350);
  });
});

/* ─── Flash auto-dismiss ─────────────────────────────────────────────────── */
const flash = document.querySelector('.flash');
if (flash) {
  setTimeout(() => {
    flash.style.transition = 'opacity .4s';
    flash.style.opacity = '0';
    setTimeout(() => flash.remove(), 400);
  }, 4000);
}

/* ─── User dropdown (click-based) ───────────────────────────────────────── */
const dropdown = document.getElementById('user-dropdown');
const dropdownBtn = document.getElementById('user-dropdown-btn');

if (dropdown && dropdownBtn) {
  dropdownBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    dropdown.classList.toggle('open');
  });

  // Close when clicking anywhere outside
  document.addEventListener('click', (e) => {
    if (!dropdown.contains(e.target)) {
      dropdown.classList.remove('open');
    }
  });

  // Close on Escape key
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') dropdown.classList.remove('open');
  });
}

/* ==========================================================================
   Cyberpunk UI Features (Toasts, AJAX Cart, Live Search, Tilt, Wishlist)
   ========================================================================== */

// 1. Toast Notifications
function showToast(message, type = 'success') {
  let container = document.getElementById('toast-container');
  if (!container) {
    container = document.createElement('div');
    container.id = 'toast-container';
    document.body.appendChild(container);
  }

  const toast = document.createElement('div');
  toast.className = `toast ${type}`;
  toast.innerText = message;

  container.appendChild(toast);

  // Trigger anim
  setTimeout(() => toast.classList.add('show'), 10);

  setTimeout(() => {
    toast.classList.add('removing');
    setTimeout(() => toast.remove(), 300);
  }, 5000);
}

// 2. AJAX Add-to-Cart
async function ajaxAddToCart(productId, quantity, btn) {
  const originalText = btn ? btn.innerHTML : '';
  if (btn) { btn.innerHTML = '⏳'; btn.disabled = true; }

  const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content;
  try {
    const res = await fetch('/api/cart/add', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': csrfToken
      },
      body: JSON.stringify({ product_id: productId, quantity: quantity })
    });

    if (res.status === 401 || res.status === 403) {
      showToast('Войдите, чтобы добавить в корзину', 'error');
      setTimeout(() => window.location.href = '/auth/login', 1200);
      return;
    }

    const data = await res.json();
    if (data.success) {
      showToast('✅ Товар добавлен в корзину!', 'success');
      // Animate badge pulse
      const badge = document.querySelector('#nav-cart-badge');
      if (badge) {
        badge.style.transform = 'scale(1.6)';
        setTimeout(() => { badge.style.transform = ''; }, 300);
      }
      // Update header nav (badge + total)
      await updateHeaderNavFields();
      // Also update bottom-nav badge if present
      const bottomBadge = document.querySelector('.bottom-nav__badge');
      if (data.cart_count > 0) {
        const cartNavItem = document.querySelector('.bottom-nav__item[data-path="/cart"] .bottom-nav__icon');
        if (cartNavItem) {
          let bb = cartNavItem.querySelector('.bottom-nav__badge');
          if (!bb) {
            bb = document.createElement('span');
            bb.className = 'bottom-nav__badge';
            cartNavItem.appendChild(bb);
          }
          bb.textContent = data.cart_count;
        }
      }
    } else {
      showToast('Ошибка добавления в корзину', 'error');
    }
  } catch (err) {
    showToast('Ошибка соединения', 'error');
  } finally {
    if (btn) { btn.innerHTML = originalText; btn.disabled = false; }
  }
}

document.querySelectorAll('form[action="/cart/add"]').forEach(form => {
  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    const btn = form.querySelector('button[type="submit"]');
    const fd = new FormData(form);
    await ajaxAddToCart(
      parseInt(fd.get('product_id')),
      parseInt(fd.get('quantity') || 1),
      btn
    );
  });
});

// 3. Live Search
const searchInput = document.getElementById('live-search');
const searchResults = document.getElementById('search-results');
let searchTimeout = null;

if (searchInput && searchResults) {
  searchInput.addEventListener('input', (e) => {
    const q = e.target.value.trim();
    clearTimeout(searchTimeout);

    if (q.length < 2) {
      searchResults.classList.remove('active');
      return;
    }

    searchTimeout = setTimeout(async () => {
      try {
        const res = await fetch('/api/search?q=' + encodeURIComponent(q));
        const items = await res.json();
        
        searchResults.innerHTML = '';
        if (items && items.length > 0) {
          items.forEach(item => {
            const a = document.createElement('a');
            a.href = '/catalog/' + item.id;
            a.className = 'search-result-item';
            a.innerHTML = `
              <img src="${item.image_url || ''}" class="search-result-img" alt="">
              <div>
                <div style="font-weight: 600;">${item.name}</div>
                <div style="color: var(--primary);">${parseFloat(item.price).toFixed(2)} ₽</div>
              </div>
            `;
            searchResults.appendChild(a);
          });
        } else {
          searchResults.innerHTML = '<div style="padding: 1rem; color: #aaa;">Ничего не найдено</div>';
        }
        searchResults.classList.add('active');
      } catch (err) {
        console.error('Search failed', err);
      }
    }, 300); // Debounce 300ms
  });

  // Hide on click outside
  document.addEventListener('click', (e) => {
    if (!searchInput.contains(e.target) && !searchResults.contains(e.target)) {
      searchResults.classList.remove('active');
    }
  });
}

// 4. AJAX Wishlist Toggle
document.querySelectorAll('.btn-wishlist').forEach(btn => {
  btn.addEventListener('click', async (e) => {
    e.preventDefault();
    const pid = btn.dataset.productId;
    const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content;
    
    if (!csrfToken) {
      showToast('Необходимо авторизоваться', 'error');
      setTimeout(() => window.location.href = '/auth/login', 1000);
      return;
    }

    try {
      const btnIcon = btn.innerHTML;
      btn.innerHTML = '...';
      const res = await fetch('/api/wishlist/toggle', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': csrfToken
        },
        body: JSON.stringify({ product_id: parseInt(pid) })
      });
      
      const data = await res.json();
      btn.innerHTML = btnIcon; // restore icon
      if (data.success) {
        if (data.added) {
          btn.classList.add('active');
          showToast('Добавлено в избранное', 'success');
        } else {
          btn.classList.remove('active');
          showToast('Удалено из избранного', 'success');
        }
      }
    } catch (err) {
      showToast('Ошибка Избранного. Вы авторизованы?', 'error');
    }
  });
});

// 5. 3D Tilt Effect on Product Cards (Vanilla approach)
document.querySelectorAll('.tilt-card').forEach(card => {
  card.addEventListener('mousemove', e => {
    const rect = card.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    
    // Normalize coordinates to -1 to 1
    const xNorm = (x / rect.width - 0.5) * 2;
    const yNorm = (y / rect.height - 0.5) * 2;
    
    // Max rotation 10 degrees
    card.style.transform = `perspective(1000px) rotateX(${-yNorm * 8}deg) rotateY(${xNorm * 8}deg) scale3d(1.02, 1.02, 1.02)`;
    card.style.transition = 'none';
  });
  
  card.addEventListener('mouseleave', () => {
    card.style.transform = `perspective(1000px) rotateX(0) rotateY(0) scale3d(1, 1, 1)`;
    card.style.transition = 'transform 0.4s ease';
  });
});

// Remove skeletons when images load
document.querySelectorAll('img').forEach(img => {
  if (img.complete) {
    const wrapper = img.closest('.skeleton');
    if (wrapper) wrapper.classList.remove('skeleton');
  } else {
    img.addEventListener('load', () => {
      const wrapper = img.closest('.skeleton');
      if (wrapper) wrapper.classList.remove('skeleton');
    });
  }
});

/* ==========================================================================
   Cart Drawer (Slide from left)
   ========================================================================== */
const cartBtn = document.getElementById('nav-cart-btn');
const cartOverlay = document.getElementById('cart-overlay');
const cartDrawer = document.getElementById('cart-drawer');
const cartClose = document.getElementById('cart-drawer-close');
const cartItemsContainer = document.getElementById('cart-drawer-items');
const cartTotalDiv = document.getElementById('cart-drawer-total');

function closeCartDrawer() {
  if(cartDrawer) cartDrawer.classList.remove('open');
  if(cartOverlay) cartOverlay.classList.remove('active');
}

async function loadCartDrawer() {
  cartItemsContainer.innerHTML = '<div class="skeleton" style="height: 80px; width: 100%; margin-bottom: 15px; border-radius: var(--radius)"></div><div class="skeleton" style="height: 80px; width: 100%; border-radius: var(--radius)"></div>';
  try {
    const res = await fetch('/api/cart');
    if (res.ok) {
      const data = await res.json();
      
      cartItemsContainer.innerHTML = '';
      const itemsArr = data.Items || data.items || [];
      if (itemsArr.length > 0) {
        itemsArr.forEach(line => {
          const prod = line.Product || line.product;
          const cartObj = line.Item || line.item;
          const qty = cartObj.Quantity || cartObj.quantity;
          const id = prod.id || prod.ID;
          
          const el = document.createElement('div');
          el.className = 'drawer-item';
          
          let imgHTML = '';
          if (prod.image_url) {
            imgHTML = `<img src="${prod.image_url}" class="drawer-item__img" alt="">`;
          } else {
            imgHTML = `<div class="drawer-item__img" style="display:flex;align-items:center;justify-content:center;font-size:2rem;background:rgba(255,255,255,0.05);">📦</div>`;
          }

          el.innerHTML = `
            ${imgHTML}
            <div class="drawer-item__info">
              <div style="display:flex; justify-content:space-between; align-items:flex-start;">
                <a href="/catalog/${id}" class="drawer-item__name" style="margin-bottom:0">${prod.name || prod.Name}</a>
                <button class="drawer-item__del" data-id="${id}" style="background:none; border:none; color:var(--c-muted); cursor:pointer; font-size:1.1rem; padding:0; margin-left:0.5rem;" title="Удалить">✕</button>
              </div>
              <div style="display:flex; justify-content:space-between; align-items:center; margin-top:0.5rem;">
                <div class="drawer-item__qty-ctrl" style="display:flex; align-items:center; gap:0.5rem; background:rgba(255,255,255,0.05); padding:0.25rem; border-radius:var(--radius)">
                  <button class="drawer-qty-btn minus" data-id="${id}" data-qty="${qty}" style="background:none; border:none; color:var(--c-text); cursor:pointer; width:24px; height:24px; border-radius:4px;">-</button>
                  <span style="font-size:0.9rem; min-width:20px; text-align:center">${qty}</span>
                  <button class="drawer-qty-btn plus" data-id="${id}" data-qty="${qty}" style="background:none; border:none; color:var(--c-text); cursor:pointer; width:24px; height:24px; border-radius:4px;">+</button>
                </div>
                <div class="drawer-item__price">${(qty * parseFloat(prod.price || prod.Price)).toFixed(2)} ₽</div>
              </div>
            </div>
          `;
          cartItemsContainer.appendChild(el);
        });

        // Bind delete buttons
        document.querySelectorAll('.drawer-item__del').forEach(btn => {
          btn.addEventListener('click', async (e) => {
            const pid = btn.dataset.id;
            const csrfMeta = document.querySelector('meta[name="csrf-token"]');
            await fetch('/api/cart/remove', { 
              method: 'POST', 
              headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfMeta ? csrfMeta.content : '' }, 
              body: JSON.stringify({ product_id: parseInt(pid) }) 
            });
            updateHeaderNavFields();
            loadCartDrawer();
          });
        });

        // Bind qty buttons
        document.querySelectorAll('.drawer-qty-btn').forEach(btn => {
          btn.addEventListener('click', async (e) => {
            const pid = btn.dataset.id;
            let q = parseInt(btn.dataset.qty);
            if (btn.classList.contains('minus')) q--; else q++;
            
            const csrfMeta = document.querySelector('meta[name="csrf-token"]');
            await fetch('/api/cart/update', { 
              method: 'POST', 
              headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfMeta ? csrfMeta.content : '' }, 
              body: JSON.stringify({ product_id: parseInt(pid), quantity: q }) 
            });
            updateHeaderNavFields();
            loadCartDrawer();
          });
        });

        const totalRaw = data.Total || data.total || 0;
        cartTotalDiv.textContent = parseFloat(totalRaw).toFixed(2) + ' ₽';
      } else {
        cartItemsContainer.innerHTML = '<div style="color: var(--c-muted); text-align: center; margin-top: 2rem;">Ваша корзина пуста</div>';
        cartTotalDiv.textContent = '0.00 ₽';
      }
    } else {
      cartItemsContainer.innerHTML = '<div style="color: var(--c-red); text-align: center; margin-top: 2rem;">Пожалуйста, войдите в систему</div>';
    }
  } catch (e) {
    cartItemsContainer.innerHTML = '<div style="color: var(--c-red);">Ошибка загрузки корзины</div>';
  }
}

async function updateHeaderNavFields() {
  const res = await fetch('/api/cart');
  if (res.ok) {
    const data = await res.json();
    const count = data.Count || data.count || 0;
    const total = data.Total || data.total || 0;
    const cartIcon = document.getElementById('nav-cart-btn');
    if(cartIcon) {
      if(count > 0) {
        let badge = cartIcon.querySelector('#nav-cart-badge') || cartIcon.querySelector('.cart-badge');
        if (!badge) {
          badge = document.createElement('span');
          badge.className = 'cart-badge';
          badge.id = 'nav-cart-badge';
          cartIcon.appendChild(badge);
        }
        badge.textContent = count;
        
        let totalSpan = cartIcon.querySelector('#nav-cart-total');
        if (!totalSpan) {
          totalSpan = document.createElement('span');
          totalSpan.className = 'cart-total';
          totalSpan.id = 'nav-cart-total';
          totalSpan.style.marginLeft = '0.5rem';
          totalSpan.style.fontWeight = '600';
          totalSpan.style.color = 'var(--c-cyan)';
          cartIcon.appendChild(totalSpan);
        }
        totalSpan.textContent = parseFloat(total).toFixed(2) + ' ₽';
      } else {
        const badge = cartIcon.querySelector('.cart-badge');
        if(badge) badge.remove();
        const tspan = cartIcon.querySelector('.cart-total');
        if(tspan) tspan.remove();
      }
    }
  }
}

if (cartBtn && cartDrawer && cartOverlay) {
  cartBtn.addEventListener('click', async (e) => {
    e.preventDefault();
    cartOverlay.classList.add('active');
    cartDrawer.classList.add('open');
    loadCartDrawer();
  });

  cartClose.addEventListener('click', closeCartDrawer);
  cartOverlay.addEventListener('click', closeCartDrawer);
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') closeCartDrawer();
  });
}

/* ─── Quick add-to-cart (catalog cards) ──────────────────────────────────── */
document.addEventListener('click', async (e) => {
  const btn = e.target.closest('.quick-cart-btn');
  if (!btn) return;
  e.preventDefault();
  e.stopPropagation();
  const pid = parseInt(btn.dataset.productId);
  if (pid) await ajaxAddToCart(pid, 1, btn);
});

/* ==========================================================================
   Catalog Sidebar — Category toggle (moved from inline script for CSP)
   ========================================================================== */
(function() {
  // Event delegation for sidebar toggle buttons
  document.addEventListener('click', function(e) {
    const btn = e.target.closest('.sidebar__toggle');
    if (!btn) return;
    const open = btn.getAttribute('data-open') === 'true';
    const sub = btn.closest('.sidebar__group').nextElementSibling;
    if (open) {
      btn.removeAttribute('data-open');
      sub.classList.remove('sidebar__sub--open');
    } else {
      btn.setAttribute('data-open', 'true');
      sub.classList.add('sidebar__sub--open');
    }
  });

  // Auto-open parent if a child is active
  document.querySelectorAll('.sidebar__sub--open').forEach(function(sub) {
    const prev = sub.previousElementSibling;
    const btn = prev && prev.querySelector('.sidebar__toggle');
    if (btn) btn.setAttribute('data-open', 'true');
  });

  // Mobile sidebar toggle
  const mobileSidebarToggle = document.getElementById('mobile-sidebar-toggle');
  const sidebar = document.querySelector('.sidebar');
  if (mobileSidebarToggle && sidebar) {
    mobileSidebarToggle.addEventListener('click', function() {
      sidebar.classList.toggle('sidebar--open');
      mobileSidebarToggle.innerHTML = sidebar.classList.contains('sidebar--open') ? '✕ Скрыть' : '☰ Фильтры';
    });
  }
})();

/* ==========================================================================
   Mobile Menu (moved from inline script for CSP compliance)
   ========================================================================== */
(function() {
  const burger  = document.getElementById('nav-burger');
  const menu    = document.getElementById('nav-mobile-menu');
  const overlay = document.getElementById('nav-mobile-overlay');
  const closeBtn= document.getElementById('nav-mobile-close');

  function openMenu() {
    menu.classList.add('open');
    overlay.classList.add('open');
    burger.classList.add('open');
    burger.setAttribute('aria-expanded', 'true');
    burger.innerHTML = '✕';
    document.body.style.overflow = 'hidden';
  }
  function closeMenu() {
    menu.classList.remove('open');
    overlay.classList.remove('open');
    burger.classList.remove('open');
    burger.setAttribute('aria-expanded', 'false');
    burger.innerHTML = '☰';
    document.body.style.overflow = '';
  }

  burger && burger.addEventListener('click', () => menu.classList.contains('open') ? closeMenu() : openMenu());
  closeBtn && closeBtn.addEventListener('click', closeMenu);
  overlay && overlay.addEventListener('click', closeMenu);

  // Close on Escape
  document.addEventListener('keydown', e => { if (e.key === 'Escape') closeMenu(); });

  // Swipe left to close
  let touchStartX = 0;
  menu && menu.addEventListener('touchstart', e => { touchStartX = e.touches[0].clientX; }, { passive: true });
  menu && menu.addEventListener('touchend', e => {
    if (e.changedTouches[0].clientX - touchStartX < -60) closeMenu();
  }, { passive: true });

  // ─── Bottom nav: mark active tab ─────────────────────────────────────
  const path = window.location.pathname;
  document.querySelectorAll('.bottom-nav__item[data-path]').forEach(el => {
    const p = el.dataset.path;
    if (p === '/' ? path === '/' : path.startsWith(p)) {
      el.classList.add('active');
    }
  });

  // ─── Mobile search toggle ─────────────────────────────────────────────
  const searchBtn = document.getElementById('bottom-nav-search-btn');
  const searchBox = document.querySelector('.search-container');
  if (searchBtn && searchBox) {
    searchBtn.addEventListener('click', () => {
      searchBox.classList.toggle('mobile-open');
      if (searchBox.classList.contains('mobile-open')) {
        searchBox.querySelector('input')?.focus();
      }
    });
  }

  // ─── Mobile first-visit hint ──────────────────────────────────────────
  const isMobile = window.matchMedia('(max-width: 640px)').matches;
  if (isMobile && !sessionStorage.getItem('mob_hint_shown')) {
    sessionStorage.setItem('mob_hint_shown', '1');
    setTimeout(() => {
      if (window.showToast) {
        window.showToast('📱 Сайт оптимизирован для мобильных устройств', 'success');
      }
    }, 1200);
  }
})();

/* ==========================================================================
   Recently Viewed Products (localStorage-based)
   ========================================================================== */
(function() {
  const STORAGE_KEY = 'magaz_recently_viewed';
  const MAX_ITEMS   = 10;

  // Track current product page visit
  const match = window.location.pathname.match(/^\/catalog\/item\/(\d+)$/);
  if (match) {
    const pid = parseInt(match[1]);
    let viewed = JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]');
    viewed = viewed.filter(id => id !== pid); // Remove if already present
    viewed.unshift(pid);                       // Add to front
    if (viewed.length > MAX_ITEMS) viewed = viewed.slice(0, MAX_ITEMS);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(viewed));
  }

  // Render recently viewed section on product pages
  const container = document.getElementById('recently-viewed');
  if (container) {
    let viewed = JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]');
    // Exclude current product
    if (match) {
      viewed = viewed.filter(id => id !== parseInt(match[1]));
    }
    if (viewed.length > 0) {
      fetch('/api/products/by-ids?ids=' + viewed.slice(0, 6).join(','))
        .then(r => r.json())
        .then(products => {
          if (!products || products.length === 0) return;
          container.style.display = 'block';
          const grid = container.querySelector('.recently-viewed__grid');
          if (!grid) return;
          products.forEach(p => {
            const card = document.createElement('a');
            card.href = '/catalog/item/' + p.id;
            card.className = 'recently-viewed__card';
            card.innerHTML = `
              <img src="${p.image_url || '/static/img/placeholder.png'}" alt="" class="recently-viewed__img">
              <div class="recently-viewed__name">${p.name}</div>
              <div class="recently-viewed__price">${parseFloat(p.price).toFixed(2)} ₽</div>
            `;
            grid.appendChild(card);
          });
        })
        .catch(() => {}); // Silently fail
    }
  }
})();

