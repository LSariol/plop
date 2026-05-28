// App state
const state = {
    files: new Set(),
    tags: new Set(),
};

// DOM elements
const sendForm         = document.getElementById('sendForm');
const messageInput     = document.getElementById('message');
const uploadBox        = document.getElementById('uploadBox');
const fileInput        = document.getElementById('fileInput');
const previewGrid      = document.getElementById('previewGrid');
const descriptionInput = document.getElementById('description');
const tagsInput        = document.getElementById('tagsInput');
const tagsContainer    = document.getElementById('tagsContainer');
const sendBtn          = document.getElementById('sendBtn');
const statusMessage    = document.getElementById('statusMessage');
const progressSection  = document.getElementById('progressSection');
const returnBtn        = document.getElementById('returnBtn');

// Tracks the payload ID we're waiting on delivery confirmation for.
let pendingDeliveryId = null;
let deliveryTimeout   = null;

document.addEventListener('DOMContentLoaded', () => {
    setupEventListeners();
    registerServiceWorker();
    connectDeliveryEvents();
    returnBtn.addEventListener('click', showForm);
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
    if (t.startsWith('video/'))       return 'video';
    if (t.startsWith('audio/'))       return 'audio';
    if (t === 'application/pdf')      return 'pdf';
    if (t.includes('zip') || t.includes('archive') || t.includes('compressed')) return 'zip';
    if (t.includes('word') || t.includes('document')) return 'doc';
    if (t.includes('sheet') || t.includes('excel'))   return 'xls';
    return 'file';
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

    showProgress();
    setStep('step1', 'active');

    try {
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
            const data = await res.json().catch(() => ({}));
            setStep('step1', 'failed', data.error || `Upload failed (${res.status})`);
            showReturnBtn();
            return;
        }

        const data = await res.json();
        setStep('step1', 'complete');
        setStep('step2', 'complete');

        if (!data.desktop_connected) {
            setStep('step3', 'cached', 'Cached to server — desktop will receive when online');
            showReturnBtn();
            return;
        }

        setStep('step3', 'active');
        pendingDeliveryId = data.id;
        deliveryTimeout = setTimeout(() => {
            if (pendingDeliveryId === data.id) {
                pendingDeliveryId = null;
                setStep('step3', 'cached', 'Cached to server — desktop will receive when online');
                showReturnBtn();
            }
        }, 30000);

    } catch (err) {
        console.error('Send failed:', err);
        setStep('step1', 'failed', 'Could not reach server — check your connection');
        showReturnBtn();
    }
}

// ─── Progress helpers ─────────────────────────────────────────────────────

function showProgress() {
    sendForm.style.display = 'none';
    progressSection.style.display = 'flex';
    ['step1','step2','step3','step4','step5'].forEach(id => setStep(id, 'pending'));
    returnBtn.style.display = 'none';
}

function showForm() {
    clearTimeout(deliveryTimeout);
    pendingDeliveryId = null;
    progressSection.style.display = 'none';
    sendForm.style.display = '';
    resetForm();
}

function setStep(id, state, errorText) {
    const el = document.getElementById(id);
    if (!el) return;
    el.className = 'step-card ' + state;
    if (id === 'step3') {
        const errEl = document.getElementById('step3Error');
        if (errEl) errEl.textContent = (state === 'failed' || state === 'cached') && errorText ? errorText : '';
    }
    if (state === 'cached' && errorText) {
        const label = el.querySelector('.step-label');
        if (label) label.textContent = errorText;
    }
}

function showReturnBtn() {
    returnBtn.style.display = 'block';
}

const STEP_LABELS = {
    step1: 'Uploading to server',
    step2: 'Upload complete',
    step3: 'Sending to desktop',
    step4: 'Desktop transfer complete',
    step5: 'Plop completed',
};

function resetForm() {
    sendForm.reset();
    previewGrid.querySelectorAll('img[data-object-url]').forEach((img) => {
        URL.revokeObjectURL(img.dataset.objectUrl);
    });
    state.files.clear();
    state.tags.clear();
    updatePreviewGrid();
    updateTagsDisplay();
    updateUploadZoneState();
    // Restore step labels that may have been mutated by the cached state.
    Object.entries(STEP_LABELS).forEach(([id, label]) => {
        const el = document.getElementById(id);
        if (el) {
            const labelEl = el.querySelector('.step-label');
            if (labelEl) labelEl.textContent = label;
        }
    });
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
        let payload;
        try { payload = JSON.parse(e.data); } catch (_) { return; }

        if (pendingDeliveryId && payload.id === pendingDeliveryId) {
            clearTimeout(deliveryTimeout);
            pendingDeliveryId = null;
            setStep('step3', 'complete');
            setStep('step4', 'complete');
            setStep('step5', 'complete');
            showReturnBtn();
        }
    });

    es.onerror = () => {
        // Browser auto-reconnects on error — no action needed.
    };
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
