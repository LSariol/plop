// App state
const state = {
    files: new Set(),
    tags: new Set(),
};

// DOM elements
const sendForm      = document.getElementById('sendForm');
const messageInput  = document.getElementById('message');
const uploadBox     = document.getElementById('uploadBox');
const fileInput     = document.getElementById('fileInput');
const previewGrid   = document.getElementById('previewGrid');
const descriptionInput = document.getElementById('description');
const tagsInput     = document.getElementById('tagsInput');
const tagsContainer = document.getElementById('tagsContainer');
const sendBtn       = document.getElementById('sendBtn');
const statusMessage = document.getElementById('statusMessage');

document.addEventListener('DOMContentLoaded', () => {
    setupEventListeners();
    registerServiceWorker();
    connectDeliveryEvents();
    messageInput.focus();
});

function setupEventListeners() {
    // File upload
    uploadBox.addEventListener('click', () => fileInput.click());
    uploadBox.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            fileInput.click();
        }
    });

    fileInput.addEventListener('change', (e) => handleFiles(e.target.files));

    // Drag and drop — use CSS classes, no inline styles
    uploadBox.addEventListener('dragover', (e) => {
        e.preventDefault();
        uploadBox.classList.add('drag-over');
    });

    uploadBox.addEventListener('dragleave', () => {
        uploadBox.classList.remove('drag-over');
    });

    uploadBox.addEventListener('drop', (e) => {
        e.preventDefault();
        uploadBox.classList.remove('drag-over');
        handleFiles(e.dataTransfer.files);
    });

    // Tags
    tagsInput.addEventListener('keydown', handleTagInput);

    // Form submit
    sendForm.addEventListener('submit', handleSubmit);
}

// ─── Files ────────────────────────────────────────────────────────────────

function handleFiles(files) {
    for (const file of files) {
        state.files.add(file);
    }
    updatePreviewGrid();
    updateUploadZoneState();
}

function removeFile(file) {
    state.files.delete(file);
    // Reset file input so the same file can be re-added
    fileInput.value = '';
    updatePreviewGrid();
    updateUploadZoneState();
}

function updateUploadZoneState() {
    if (state.files.size > 0) {
        uploadBox.classList.add('has-files');
    } else {
        uploadBox.classList.remove('has-files');
    }
}

// ─── Preview grid ─────────────────────────────────────────────────────────

function updatePreviewGrid() {
    // Revoke any existing object URLs before clearing
    previewGrid.querySelectorAll('img[data-object-url]').forEach((img) => {
        URL.revokeObjectURL(img.dataset.objectUrl);
    });
    previewGrid.innerHTML = '';

    if (state.files.size === 0) return;

    state.files.forEach((file) => {
        const item = document.createElement('div');
        item.className = 'preview-item';

        if (file.type.startsWith('image/')) {
            const url = URL.createObjectURL(file);
            const img = document.createElement('img');
            img.alt = file.name;
            img.loading = 'lazy';
            img.src = url;
            img.dataset.objectUrl = url;
            item.appendChild(img);
        } else {
            item.classList.add('file-card');
            const icon = document.createElement('div');
            icon.className = 'file-icon';
            icon.textContent = getFileIcon(file);
            const name = document.createElement('div');
            name.className = 'file-name';
            name.textContent = file.name;
            item.appendChild(icon);
            item.appendChild(name);
        }

        const removeBtn = document.createElement('button');
        removeBtn.type = 'button';
        removeBtn.className = 'preview-remove';
        removeBtn.setAttribute('aria-label', `Remove ${file.name}`);
        removeBtn.textContent = '×';
        removeBtn.addEventListener('click', () => {
            const img = item.querySelector('img[data-object-url]');
            if (img) URL.revokeObjectURL(img.dataset.objectUrl);
            removeFile(file);
        });
        item.appendChild(removeBtn);

        previewGrid.appendChild(item);
    });
}

function getFileIcon(file) {
    const t = file.type;
    if (t.startsWith('video/'))       return '🎬';
    if (t.startsWith('audio/'))       return '🎵';
    if (t === 'application/pdf')      return '📄';
    if (t.includes('zip') || t.includes('archive') || t.includes('compressed')) return '🗜️';
    if (t.includes('word') || t.includes('document')) return '📝';
    if (t.includes('sheet') || t.includes('excel'))   return '📊';
    return '📎';
}

// ─── Tags ─────────────────────────────────────────────────────────────────

function handleTagInput(e) {
    if (e.key === 'Enter' || e.key === ',') {
        e.preventDefault();
        const tag = tagsInput.value.trim().replace(/,/g, '');
        if (tag && !state.tags.has(tag)) {
            state.tags.add(tag);
            updateTagsDisplay();
            tagsInput.value = '';
        }
    }
}

function updateTagsDisplay() {
    tagsContainer.innerHTML = '';
    state.tags.forEach((tag) => {
        const chip = document.createElement('button');
        chip.type = 'button';
        chip.className = 'tag-chip';
        chip.innerHTML = `${escapeHtml(tag)} <span class="remove" aria-hidden="true">×</span>`;
        chip.setAttribute('aria-label', `Remove tag: ${tag}`);
        chip.addEventListener('click', () => {
            state.tags.delete(tag);
            updateTagsDisplay();
        });
        tagsContainer.appendChild(chip);
    });
}

// ─── Submit ───────────────────────────────────────────────────────────────

async function handleSubmit(e) {
    e.preventDefault();

    const message = messageInput.value.trim();
    if (!message && state.files.size === 0) {
        showStatus('Add a message or attach at least one file.', 'error');
        messageInput.focus();
        return;
    }

    const formData = new FormData();
    formData.append('text', message);
    formData.append('tags', Array.from(state.tags).join(','));
    if (descriptionInput.value.trim()) {
        formData.append('description', descriptionInput.value.trim());
    }
    state.files.forEach((file) => formData.append('files', file));

    try {
        sendBtn.disabled = true;
        showStatus('Sending to desktop…', 'info');

        const res = await fetch('/upload', {
            method: 'POST',
            credentials: 'include',
            body: formData,
        });

        if (res.status === 401) {
            window.location.href = '/login';
            return;
        }
        if (!res.ok) {
            throw new Error(`Server error: ${res.status}`);
        }

        showStatus('✓ Sent to desktop!', 'success');
        resetForm();

        setTimeout(() => {
            statusMessage.className = 'status';
        }, 3500);
    } catch (err) {
        console.error('Send failed:', err);
        showStatus('Failed to send — check your connection and try again.', 'error');
    } finally {
        sendBtn.disabled = false;
    }
}

function resetForm() {
    sendForm.reset();
    // Revoke object URLs before clearing
    previewGrid.querySelectorAll('img[data-object-url]').forEach((img) => {
        URL.revokeObjectURL(img.dataset.objectUrl);
    });
    state.files.clear();
    state.tags.clear();
    updatePreviewGrid();
    updateTagsDisplay();
    updateUploadZoneState();
    messageInput.focus();
}

// ─── Helpers ──────────────────────────────────────────────────────────────

function showStatus(message, type) {
    statusMessage.textContent = message;
    statusMessage.className = 'status ' + type;
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Dismiss status on Escape
document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
        statusMessage.className = 'status';
        statusMessage.textContent = '';
    }
});

// ─── Delivery Confirmation ────────────────────────────────────────────────

function connectDeliveryEvents() {
    const es = new EventSource('/events');

    es.addEventListener('delivered', (e) => {
        try {
            JSON.parse(e.data); // validate
        } catch (_) { return; }
        showDeliveryToast();
    });

    es.onerror = () => {
        // Browser auto-reconnects on error — no action needed.
    };
}

function showDeliveryToast() {
    const toast = document.createElement('div');
    toast.className = 'status success';
    toast.style.cssText = 'position:fixed;bottom:1.5rem;left:50%;transform:translateX(-50%);z-index:100;padding:0.75rem 1.25rem;border-radius:var(--radius-md);box-shadow:var(--shadow-md);';
    toast.textContent = '✓ Delivered to desktop';
    document.body.appendChild(toast);
    setTimeout(() => toast.remove(), 4000);
}

// ─── Service Worker ───────────────────────────────────────────────────────

async function registerServiceWorker() {
    if ('serviceWorker' in navigator) {
        try {
            const reg = await navigator.serviceWorker.register('/sw.js');
            // Force an immediate re-check on every load so updates are picked
            // up without waiting for the browser's 24-hour throttle.
            reg.update();
        } catch (err) {
            console.warn('Service Worker registration failed:', err);
        }
    }
}
