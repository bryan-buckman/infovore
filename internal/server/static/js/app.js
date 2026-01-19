// Infovore App JS
(function () {
    const settingsModal = document.getElementById('settingsModal');
    const menuBtn = document.getElementById('menuBtn');
    const closeSettings = document.getElementById('closeSettings');
    const saveSettings = document.getElementById('saveSettings');
    const refreshBtn = document.getElementById('refreshBtn');
    const importBtn = document.getElementById('importBtn');
    const cleanupBtn = document.getElementById('cleanupBtn');
    const toast = document.getElementById('toast');
    const toastMessage = document.getElementById('toastMessage');
    const toastProgress = document.getElementById('toastProgress');
    const sidebarToggle = document.getElementById('sidebarToggle');
    const sidebar = document.getElementById('sidebar');
    const itemsContainer = document.getElementById('itemsContainer');

    // Sidebar toggle (mobile)
    if (sidebarToggle) sidebarToggle.onclick = () => sidebar.classList.toggle('open');

    // Settings modal
    if (menuBtn) menuBtn.onclick = () => settingsModal.classList.add('active');
    if (closeSettings) closeSettings.onclick = () => settingsModal.classList.remove('active');
    settingsModal?.addEventListener('click', e => { if (e.target === settingsModal) settingsModal.classList.remove('active'); });

    // Toast helper
    function showToast(msg, duration = 3000) {
        toastMessage.textContent = msg;
        toast.classList.add('active');
        toastProgress.style.width = '100%';
        setTimeout(() => { toastProgress.style.width = '0%'; }, 50);
        setTimeout(() => { toast.classList.remove('active'); }, duration);
    }

    // Save settings
    if (saveSettings) saveSettings.onclick = async () => {
        const interval = parseInt(document.getElementById('pollingInterval').value, 10);
        showToast('Saving settings...');
        try {
            const res = await fetch('/api/settings', {
                method: 'POST', headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ polling_interval: interval })
            });
            const data = await res.json();
            showToast(`Saved! Interval: ${data.polling_interval}m`);
            settingsModal.classList.remove('active');
        } catch (e) { showToast('Error saving settings'); }
    };

    // Refresh feeds
    if (refreshBtn) refreshBtn.onclick = async () => {
        showToast('Refreshing feeds...', 10000);
        try {
            const res = await fetch('/api/refresh', { method: 'POST' });
            const data = await res.json();
            showToast(`Fetched ${data.new_items} new items`);
            setTimeout(() => location.reload(), 1500);
        } catch (e) { showToast('Refresh failed'); }
    };

    // Import OPML
    if (importBtn) importBtn.onclick = async () => {
        const fileInput = document.getElementById('opmlFile');
        if (!fileInput.files.length) { showToast('Select a file first'); return; }
        const formData = new FormData();
        formData.append('opml', fileInput.files[0]);
        showToast('Importing...', 15000);
        try {
            const res = await fetch('/api/import-opml', { method: 'POST', body: formData });
            const data = await res.json();
            showToast(`Imported ${data.imported} of ${data.total} feeds`);
            setTimeout(() => location.reload(), 1500);
        } catch (e) { showToast('Import failed'); }
    };

    // Cleanup
    if (cleanupBtn) cleanupBtn.onclick = async () => {
        showToast('Cleaning up...', 5000);
        try {
            const res = await fetch('/api/cleanup', { method: 'POST' });
            const data = await res.json();
            showToast(`Deleted ${data.deleted} items`);
            setTimeout(() => location.reload(), 1500);
        } catch (e) { showToast('Cleanup failed'); }
    };

    // Expand items on click
    itemsContainer?.addEventListener('click', e => {
        const item = e.target.closest('.item');
        if (item && !e.target.closest('a')) item.classList.toggle('expanded');
    });

    // Mark items as read on scroll using IntersectionObserver
    const readItems = new Set();
    const observer = new IntersectionObserver(entries => {
        entries.forEach(entry => {
            if (entry.isIntersecting && entry.intersectionRatio >= 0.5) {
                const item = entry.target;
                const id = parseInt(item.dataset.itemId, 10);
                if (id && !readItems.has(id) && item.classList.contains('unread')) {
                    readItems.add(id);
                    item.classList.remove('unread');
                }
            }
        });
    }, { threshold: 0.5 });

    document.querySelectorAll('.item.unread').forEach(item => observer.observe(item));

    // Periodically send read items to server
    setInterval(() => {
        if (readItems.size === 0) return;
        const ids = Array.from(readItems);
        readItems.clear();
        fetch('/api/mark-read', {
            method: 'POST', headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ item_ids: ids })
        }).catch(() => { });
    }, 5000);

    // Send remaining on unload
    window.addEventListener('beforeunload', () => {
        if (readItems.size > 0) {
            navigator.sendBeacon('/api/mark-read', JSON.stringify({ item_ids: Array.from(readItems) }));
        }
    });
})();
