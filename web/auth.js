function initAuth() {
    const loginForm = document.getElementById('loginForm');
    const createForm = document.getElementById('createForm');

    if (loginForm) {
        loginForm.addEventListener('submit', handleLoginSubmit);
    }

    if (createForm) {
        createForm.addEventListener('submit', handleCreateSubmit);
    }
}

async function handleLoginSubmit(event) {
    event.preventDefault();

    const form = event.currentTarget;
    const status = document.getElementById('authStatus');

    const formData = new FormData(form);
    const username = formData.get('username')?.trim();
    const password = formData.get('password')?.trim();

    if (!username || !password) {
        showAuthStatus(status, 'Please enter both username and password.', 'error');
        return;
    }

    showAuthStatus(status, 'Logging in…', 'info');

    try {
        const res = await fetch('/auth/login', {
            method: 'POST',
            credentials: 'include',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password }),
        });

        if (!res.ok) {
            showAuthStatus(status, 'Login failed — check your username and password.', 'error');
            return;
        }

        window.location.href = '/app';
    } catch (err) {
        showAuthStatus(status, 'Network error — try again.', 'error');
    }
}

async function handleCreateSubmit(event) {
    event.preventDefault();

    const form = event.currentTarget;
    const status = document.getElementById('authStatus');

    const formData = new FormData(form);
    const username = formData.get('username')?.trim();
    const password = formData.get('password')?.trim();

    if (!username || !password) {
        showAuthStatus(status, 'Please fill in all fields.', 'error');
        return;
    }
    if (username.length < 2) {
        showAuthStatus(status, 'Username must be at least 2 characters.', 'error');
        return;
    }
    if (password.length < 8) {
        showAuthStatus(status, 'Password must be at least 8 characters.', 'error');
        return;
    }

    showAuthStatus(status, 'Creating your account…', 'info');

    try {
        const res = await fetch('/auth/register', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password }),
        });

        if (res.status === 409) {
            showAuthStatus(status, 'That username is already taken — try another.', 'error');
            return;
        }
        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            showAuthStatus(status, data.error || 'Registration failed — try again.', 'error');
            return;
        }

        showAuthStatus(status, 'Account created! Redirecting to login…', 'success');
        setTimeout(() => { window.location.href = '/login'; }, 1200);
    } catch (err) {
        showAuthStatus(status, 'Network error — try again.', 'error');
    }
}

function showAuthStatus(container, message, type) {
    if (!container) return;
    container.textContent = message;
    container.className = 'status ' + type;
}

document.addEventListener('DOMContentLoaded', initAuth);
