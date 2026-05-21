document.addEventListener('DOMContentLoaded', () => {
    loadDesktops();
    document.getElementById('pairBtn').addEventListener('click', generatePairingCode);
    document.getElementById('logoutBtn').addEventListener('click', doLogout);
});

// ─── Paired Desktops ──────────────────────────────────────────────────────

async function loadDesktops() {
    const list = document.getElementById('desktopList');
    try {
        const res = await fetch('/desktops', { credentials: 'include' });
        if (res.status === 401) { window.location.href = '/login'; return; }
        if (!res.ok) throw new Error(`Server error: ${res.status}`);

        const desktops = await res.json();
        if (desktops.length === 0) {
            list.innerHTML = '<p class="helper-text" style="text-align:left;">No desktops paired yet.</p>';
            return;
        }

        list.innerHTML = '';
        desktops.forEach(d => list.appendChild(buildDesktopRow(d)));
    } catch (err) {
        list.innerHTML = '<p class="helper-text" style="text-align:left; color:var(--error);">Failed to load desktops.</p>';
    }
}

function buildDesktopRow(desktop) {
    const row = document.createElement('div');
    row.id = 'desktop-' + desktop.id;
    row.style.cssText = 'display:flex; align-items:center; justify-content:space-between; padding: 0.75rem 0; border-bottom: 1px solid var(--border);';

    const info = document.createElement('div');
    const name = document.createElement('div');
    name.style.cssText = 'font-weight: 600; color: var(--text);';
    name.textContent = desktop.name;
    const date = document.createElement('div');
    date.className = 'helper-text';
    date.style.textAlign = 'left';
    date.textContent = 'Paired ' + formatRelativeDate(desktop.created_at);
    info.appendChild(name);
    info.appendChild(date);

    const revokeBtn = document.createElement('button');
    revokeBtn.type = 'button';
    revokeBtn.className = 'btn btn-outline';
    revokeBtn.style.cssText = 'padding: 0.4rem 0.9rem; font-size: 0.85rem; color: var(--error); border-color: var(--error);';
    revokeBtn.textContent = 'Revoke';
    revokeBtn.addEventListener('click', () => revokeDesktop(desktop.id, revokeBtn));

    row.appendChild(info);
    row.appendChild(revokeBtn);
    return row;
}

async function revokeDesktop(id, btn) {
    btn.disabled = true;
    btn.textContent = 'Revoking…';
    try {
        const res = await fetch('/desktops/' + id, {
            method: 'DELETE',
            credentials: 'include',
        });
        if (!res.ok) throw new Error(`Server error: ${res.status}`);
        document.getElementById('desktop-' + id)?.remove();
        const list = document.getElementById('desktopList');
        if (!list.hasChildNodes() || list.innerHTML.trim() === '') {
            list.innerHTML = '<p class="helper-text" style="text-align:left;">No desktops paired yet.</p>';
        }
    } catch (err) {
        btn.disabled = false;
        btn.textContent = 'Revoke';
    }
}

// ─── Sign Out ─────────────────────────────────────────────────────────────

async function doLogout() {
    const btn = document.getElementById('logoutBtn');
    btn.disabled = true;
    btn.textContent = 'Signing out…';

    try {
        const res = await fetch('/auth/logout', { method: 'POST', credentials: 'include' });
        if (res.ok) {
            window.location.href = '/';
            return;
        }
        throw new Error(`Server error: ${res.status}`);
    } catch (err) {
        btn.disabled = false;
        btn.textContent = 'Sign Out';
    }
}

function formatRelativeDate(iso) {
    const d = new Date(iso);
    const diff = Date.now() - d.getTime();
    const days = Math.floor(diff / 86400000);
    if (days === 0) return 'today';
    if (days === 1) return 'yesterday';
    if (days < 30) return days + ' days ago';
    return d.toLocaleDateString();
}

// ─── Pair New Desktop ─────────────────────────────────────────────────────

let pairCountdownInterval = null;

async function generatePairingCode() {
    const btn = document.getElementById('pairBtn');
    btn.disabled = true;
    btn.textContent = 'Generating…';

    try {
        const res = await fetch('/pairing-code', { method: 'POST', credentials: 'include' });
        if (res.status === 401) { window.location.href = '/login'; return; }
        if (!res.ok) throw new Error(`Server error: ${res.status}`);

        const { code, expires_in } = await res.json();

        document.getElementById('pairCode').textContent = code.slice(0, 4) + ' ' + code.slice(4);
        document.getElementById('pairResult').style.display = 'block';
        btn.textContent = 'Generate New Code';

        if (pairCountdownInterval) clearInterval(pairCountdownInterval);
        let remaining = expires_in;
        const countdown = document.getElementById('pairCountdown');
        const tick = () => {
            if (remaining <= 0) {
                clearInterval(pairCountdownInterval);
                document.getElementById('pairCode').textContent = '— expired —';
                countdown.textContent = 'Generate a new code to pair another desktop.';
                return;
            }
            const m = Math.floor(remaining / 60);
            const s = remaining % 60;
            countdown.textContent = `Expires in ${m}:${String(s).padStart(2, '0')}`;
            remaining--;
        };
        tick();
        pairCountdownInterval = setInterval(tick, 1000);
    } catch (err) {
        btn.textContent = 'Generate Pairing Code';
    } finally {
        btn.disabled = false;
    }
}
