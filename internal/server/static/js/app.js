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

    // Context menus
    const feedContextMenu = document.getElementById('feedContextMenu');
    const folderContextMenu = document.getElementById('folderContextMenu');
    const deleteFeedBtn = document.getElementById('deleteFeedBtn');
    const updateFeedBtn = document.getElementById('updateFeedBtn');
    const deleteFolderBtn = document.getElementById('deleteFolderBtn');
    const updateFolderBtn = document.getElementById('updateFolderBtn');

    // Confirm modal
    const confirmModal = document.getElementById('confirmModal');
    const closeConfirm = document.getElementById('closeConfirm');
    const confirmNo = document.getElementById('confirmNo');
    const confirmYes = document.getElementById('confirmYes');
    const confirmMessage = document.getElementById('confirmMessage');

    // Add Feed modal
    const addFeedModal = document.getElementById('addFeedModal');
    const closeAddFeed = document.getElementById('closeAddFeed');
    const feedUrlInput = document.getElementById('feedUrlInput');
    const addFeedFolderId = document.getElementById('addFeedFolderId');
    const submitAddFeed = document.getElementById('submitAddFeed');
    const addFeedSettingsBtn = document.getElementById('addFeedSettingsBtn');
    const addFeedFolderBtn = document.getElementById('addFeedFolderBtn');

    // Add Folder modal
    const addFolderModal = document.getElementById('addFolderModal');
    const closeAddFolder = document.getElementById('closeAddFolder');
    const folderNameInput = document.getElementById('folderNameInput');
    const submitAddFolder = document.getElementById('submitAddFolder');
    const addFolderSettingsBtn = document.getElementById('addFolderSettingsBtn');

    // Sidebar toggle (mobile)
    if (sidebarToggle) sidebarToggle.onclick = () => sidebar.classList.toggle('open');

    // Collapsible folders - use arrow click area only, not the whole link
    const FOLDER_STATE_KEY = 'infovore_folder_state';
    function getFolderState() {
        try {
            return JSON.parse(localStorage.getItem(FOLDER_STATE_KEY)) || {};
        } catch { return {}; }
    }
    function saveFolderState(state) {
        localStorage.setItem(FOLDER_STATE_KEY, JSON.stringify(state));
    }
    document.querySelectorAll('.folder-toggle').forEach(toggle => {
        const folderId = toggle.dataset.folderId;
        const feedsContainer = document.getElementById('folder-' + folderId);
        if (!feedsContainer) return;

        // Restore saved state (default to expanded)
        const state = getFolderState();
        if (state[folderId] === false) {
            feedsContainer.classList.add('collapsed');
            toggle.classList.add('collapsed');
        }

        // Toggle collapse on click of the ::before arrow area (first 20px)
        toggle.addEventListener('click', (e) => {
            const rect = toggle.getBoundingClientRect();
            const clickX = e.clientX - rect.left;
            // If clicked on the arrow area (first 24px), toggle collapse
            if (clickX < 24) {
                e.preventDefault();
                const isCollapsed = feedsContainer.classList.toggle('collapsed');
                toggle.classList.toggle('collapsed', isCollapsed);
                const state = getFolderState();
                state[folderId] = !isCollapsed;
                saveFolderState(state);
            }
            // Otherwise, let the link navigate to the folder view
        });
    });

    // Context menu state
    let contextFeedId = null;
    let contextFolderId = null;
    let pendingConfirmAction = null;

    // Feed context menu
    document.querySelectorAll('.feed-item[data-feed-id]').forEach(feedItem => {
        feedItem.addEventListener('contextmenu', (e) => {
            e.preventDefault();
            hideAllContextMenus();
            contextFeedId = feedItem.dataset.feedId;
            feedContextMenu.style.left = e.clientX + 'px';
            feedContextMenu.style.top = e.clientY + 'px';
            feedContextMenu.classList.add('active');
        });
    });

    // Folder context menu
    document.querySelectorAll('.folder-toggle[data-folder-id]').forEach(folderToggle => {
        folderToggle.addEventListener('contextmenu', (e) => {
            e.preventDefault();
            hideAllContextMenus();
            contextFolderId = folderToggle.dataset.folderId;
            folderContextMenu.style.left = e.clientX + 'px';
            folderContextMenu.style.top = e.clientY + 'px';
            folderContextMenu.classList.add('active');
        });
    });

    function hideAllContextMenus() {
        feedContextMenu?.classList.remove('active');
        folderContextMenu?.classList.remove('active');
        contextFeedId = null;
        contextFolderId = null;
    }

    // Hide context menu on click elsewhere
    document.addEventListener('click', (e) => {
        if (!e.target.closest('.context-menu')) {
            hideAllContextMenus();
        }
    });

    // Update feed
    if (updateFeedBtn) {
        updateFeedBtn.onclick = async () => {
            if (!contextFeedId) return;
            const feedId = contextFeedId;
            hideAllContextMenus();

            showToast('Updating feed...', 30000);
            try {
                const res = await fetch(`/api/refresh-feed/${feedId}`, { method: 'POST' });
                const data = await res.json();
                if (res.ok) {
                    showToast(`Fetched ${data.new_items} new items`);
                    setTimeout(() => location.reload(), 1000);
                } else {
                    showToast(data.error || 'Update failed');
                }
            } catch (e) {
                showToast('Error updating feed');
            }
        };
    }

    // Delete feed
    if (deleteFeedBtn) {
        deleteFeedBtn.onclick = async () => {
            if (!contextFeedId) return;
            if (!confirm('Remove this feed and all its items?')) return;

            showToast('Removing feed...');
            try {
                const res = await fetch(`/api/feed/${contextFeedId}`, { method: 'DELETE' });
                if (res.ok) {
                    showToast('Feed removed');
                    setTimeout(() => location.reload(), 1000);
                } else {
                    showToast('Failed to remove feed');
                }
            } catch (e) {
                showToast('Error removing feed');
            }
            hideAllContextMenus();
        };
    }

    // Update folder
    if (updateFolderBtn) {
        updateFolderBtn.onclick = async () => {
            if (!contextFolderId) return;
            const folderId = contextFolderId;
            hideAllContextMenus();

            showToast('Updating folder feeds...', 60000);
            try {
                const res = await fetch(`/api/refresh-folder/${folderId}`, { method: 'POST' });
                const data = await res.json();
                if (res.ok) {
                    showToast(`Fetched ${data.new_items} new items from ${data.feeds} feeds`);
                    setTimeout(() => location.reload(), 1000);
                } else {
                    showToast(data.error || 'Update failed');
                }
            } catch (e) {
                showToast('Error updating folder');
            }
        };
    }

    // Delete folder - show confirm modal
    if (deleteFolderBtn) {
        deleteFolderBtn.onclick = () => {
            if (!contextFolderId) return;
            pendingConfirmAction = { type: 'deleteFolder', folderId: contextFolderId };
            confirmMessage.textContent = 'Are you sure you want to delete this folder and ALL feeds inside it? This cannot be undone.';
            confirmModal.classList.add('active');
            hideAllContextMenus();
        };
    }

    // Confirm modal handlers
    function closeConfirmModal() {
        confirmModal?.classList.remove('active');
        pendingConfirmAction = null;
    }

    if (closeConfirm) closeConfirm.onclick = closeConfirmModal;
    if (confirmNo) confirmNo.onclick = closeConfirmModal;
    confirmModal?.addEventListener('click', e => { if (e.target === confirmModal) closeConfirmModal(); });

    if (confirmYes) {
        confirmYes.onclick = async () => {
            if (!pendingConfirmAction) return;

            if (pendingConfirmAction.type === 'deleteFolder') {
                const folderId = pendingConfirmAction.folderId;
                closeConfirmModal();

                showToast('Deleting folder...');
                try {
                    const res = await fetch(`/api/folder/${folderId}`, { method: 'DELETE' });
                    if (res.ok) {
                        showToast('Folder deleted');
                        setTimeout(() => location.href = '/', 1000);
                    } else {
                        const data = await res.json();
                        showToast(data.error || 'Failed to delete folder');
                    }
                } catch (e) {
                    showToast('Error deleting folder');
                }
            }
        };
    }

    // Settings modal
    if (menuBtn) menuBtn.onclick = () => settingsModal.classList.add('active');
    if (closeSettings) closeSettings.onclick = () => settingsModal.classList.remove('active');
    settingsModal?.addEventListener('click', e => { if (e.target === settingsModal) settingsModal.classList.remove('active'); });

    // Add Feed modal helpers
    function openAddFeedModal(folderId = null) {
        if (addFeedFolderId) addFeedFolderId.value = folderId || '';
        if (feedUrlInput) feedUrlInput.value = '';
        addFeedModal?.classList.add('active');
        feedUrlInput?.focus();
    }

    function closeAddFeedModal() {
        addFeedModal?.classList.remove('active');
    }

    // Add Feed from settings menu (no folder)
    if (addFeedSettingsBtn) {
        addFeedSettingsBtn.onclick = () => {
            settingsModal?.classList.remove('active');
            openAddFeedModal(null);
        };
    }

    // Add Feed from folder context menu
    if (addFeedFolderBtn) {
        addFeedFolderBtn.onclick = () => {
            if (!contextFolderId) return;
            const folderId = contextFolderId; // Save before hide clears it
            hideAllContextMenus();
            openAddFeedModal(folderId);
        };
    }

    // Add Feed modal close handlers
    if (closeAddFeed) closeAddFeed.onclick = closeAddFeedModal;
    addFeedModal?.addEventListener('click', e => { if (e.target === addFeedModal) closeAddFeedModal(); });

    // Submit Add Feed
    if (submitAddFeed) {
        submitAddFeed.onclick = async () => {
            const url = feedUrlInput?.value?.trim();
            if (!url) {
                showToast('Please enter a feed URL');
                return;
            }

            const folderId = addFeedFolderId?.value ? parseInt(addFeedFolderId.value, 10) : null;
            closeAddFeedModal();
            showToast('Adding feed...', 10000);

            try {
                const res = await fetch('/api/feed', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ url, folder_id: folderId })
                });
                const data = await res.json();
                if (res.ok) {
                    showToast(data.is_new ? 'Feed added! Updating...' : 'Feed already exists');
                    if (data.is_new) {
                        // Fetch the new feed immediately
                        await fetch(`/api/refresh-feed/${data.feed_id}`, { method: 'POST' });
                    }
                    setTimeout(() => location.reload(), 500);
                } else {
                    showToast(data.error || 'Failed to add feed');
                }
            } catch (e) {
                showToast('Error adding feed');
            }
        };
    }

    // Allow Enter key to submit
    feedUrlInput?.addEventListener('keypress', e => {
        if (e.key === 'Enter') submitAddFeed?.click();
    });

    // Add Folder modal helpers
    function openAddFolderModal() {
        if (folderNameInput) folderNameInput.value = '';
        addFolderModal?.classList.add('active');
        folderNameInput?.focus();
    }

    function closeAddFolderModal() {
        addFolderModal?.classList.remove('active');
    }

    // Add Folder from settings menu
    if (addFolderSettingsBtn) {
        addFolderSettingsBtn.onclick = () => {
            settingsModal?.classList.remove('active');
            openAddFolderModal();
        };
    }

    // Add Folder modal close handlers
    if (closeAddFolder) closeAddFolder.onclick = closeAddFolderModal;
    addFolderModal?.addEventListener('click', e => { if (e.target === addFolderModal) closeAddFolderModal(); });

    // Submit Add Folder
    if (submitAddFolder) {
        submitAddFolder.onclick = async () => {
            const name = folderNameInput?.value?.trim();
            if (!name) {
                showToast('Please enter a folder name');
                return;
            }

            closeAddFolderModal();
            showToast('Creating folder...', 5000);

            try {
                const res = await fetch('/api/folder', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ name })
                });
                const data = await res.json();
                if (res.ok) {
                    showToast('Folder created');
                    setTimeout(() => location.reload(), 500);
                } else {
                    showToast(data.error || 'Failed to create folder');
                }
            } catch (e) {
                showToast('Error creating folder');
            }
        };
    }

    // Allow Enter key to submit folder
    folderNameInput?.addEventListener('keypress', e => {
        if (e.key === 'Enter') submitAddFolder?.click();
    });

    // Database settings
    const dbUrlInput = document.getElementById('dbUrlInput');
    const saveDbBtn = document.getElementById('saveDbBtn');

    // Load current database URL when settings modal opens
    if (menuBtn && dbUrlInput) {
        menuBtn.addEventListener('click', async () => {
            try {
                const res = await fetch('/api/database-settings');
                if (res.ok) {
                    const data = await res.json();
                    dbUrlInput.value = data.db_url || '';
                }
            } catch (e) {
                console.error('Failed to load database settings:', e);
            }
        });
    }

    // Save database URL
    if (saveDbBtn) {
        saveDbBtn.onclick = async () => {
            const dbUrl = dbUrlInput?.value?.trim() || '';

            // Validate URL format if provided
            if (dbUrl && !dbUrl.startsWith('postgres://') && !dbUrl.startsWith('postgresql://') && !dbUrl.startsWith('sqlite://')) {
                showToast('Invalid URL. Use postgres://... or sqlite://... or leave empty for default SQLite');
                return;
            }

            showToast('Saving database settings...');

            try {
                const res = await fetch('/api/database-settings', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ db_url: dbUrl })
                });
                const data = await res.json();
                if (res.ok) {
                    showToast('Database settings saved. Restart to apply changes.', 5000);
                } else {
                    showToast(data.error || 'Failed to save database settings');
                }
            } catch (e) {
                showToast('Error saving database settings');
            }
        };
    }

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
        refreshBtn.disabled = true;
        showToast('Updating feeds... This may take a while.', 60000);
        try {
            const res = await fetch('/api/refresh', { method: 'POST' });
            const data = await res.json();
            showToast(`Fetched ${data.new_items} new items from ${data.feeds} feeds`);
            setTimeout(() => location.reload(), 1500);
        } catch (e) {
            showToast('Refresh failed');
            refreshBtn.disabled = false;
        }
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
            showToast(`Imported ${data.imported} of ${data.total} feeds. Click "Update Feeds" to fetch items.`);
            setTimeout(() => location.reload(), 2000);
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

    // Drag and drop for feeds
    let draggedFeed = null;

    // Helper to move feed to folder
    async function moveFeedToFolder(feedId, targetFolderId) {
        // Don't move if dropped in same folder
        const currentFolder = draggedFeed?.closest('.drop-zone');
        if (currentFolder && currentFolder.dataset.folderId === targetFolderId) return;

        showToast('Moving feed...');
        try {
            const res = await fetch(`/api/feed/${feedId}/move`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ folder_id: targetFolderId === '0' ? null : parseInt(targetFolderId, 10) })
            });
            if (res.ok) {
                showToast('Feed moved');
                setTimeout(() => location.reload(), 500);
            } else {
                const data = await res.json();
                showToast(data.error || 'Failed to move feed');
            }
        } catch (e) {
            showToast('Error moving feed');
        }
    }

    // Clear all drag-over styles
    function clearDragStyles() {
        document.querySelectorAll('.drop-zone').forEach(z => z.classList.remove('drag-over'));
        document.querySelectorAll('.folder').forEach(f => f.classList.remove('drag-over'));
    }

    document.querySelectorAll('.feed-item[draggable="true"]').forEach(feedItem => {
        feedItem.addEventListener('dragstart', (e) => {
            draggedFeed = feedItem;
            feedItem.classList.add('dragging');
            e.dataTransfer.effectAllowed = 'move';
            e.dataTransfer.setData('text/plain', feedItem.dataset.feedId);
        });

        feedItem.addEventListener('dragend', () => {
            feedItem.classList.remove('dragging');
            draggedFeed = null;
            clearDragStyles();
        });
    });

    // Handle drop zones (the container divs inside folders)
    document.querySelectorAll('.drop-zone').forEach(zone => {
        zone.addEventListener('dragover', (e) => {
            e.preventDefault();
            e.dataTransfer.dropEffect = 'move';
            zone.classList.add('drag-over');
        });

        zone.addEventListener('dragleave', (e) => {
            // Only remove if leaving the zone entirely
            if (!zone.contains(e.relatedTarget)) {
                zone.classList.remove('drag-over');
            }
        });

        zone.addEventListener('drop', async (e) => {
            e.preventDefault();
            clearDragStyles();
            if (!draggedFeed) return;
            await moveFeedToFolder(draggedFeed.dataset.feedId, zone.dataset.folderId);
        });
    });

    // Also handle drops on folder toggles (the folder name/icon)
    document.querySelectorAll('.folder').forEach(folder => {
        const folderId = folder.dataset.folderId;

        folder.addEventListener('dragover', (e) => {
            e.preventDefault();
            e.dataTransfer.dropEffect = 'move';
            folder.classList.add('drag-over');
        });

        folder.addEventListener('dragleave', (e) => {
            if (!folder.contains(e.relatedTarget)) {
                folder.classList.remove('drag-over');
            }
        });

        folder.addEventListener('drop', async (e) => {
            e.preventDefault();
            clearDragStyles();
            if (!draggedFeed) return;
            await moveFeedToFolder(draggedFeed.dataset.feedId, folderId);
        });
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
                    item.classList.add('read');
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

    // Delete read items when navigating away
    let markedReadIds = [];

    // Collect all items marked as read for deletion
    const collectReadItems = () => {
        document.querySelectorAll('.item.read').forEach(item => {
            const id = parseInt(item.dataset.itemId, 10);
            if (id && !markedReadIds.includes(id)) {
                markedReadIds.push(id);
            }
        });
    };

    // Send remaining read items and delete them on unload
    window.addEventListener('beforeunload', () => {
        // First mark any pending items as read
        if (readItems.size > 0) {
            navigator.sendBeacon('/api/mark-read', JSON.stringify({ item_ids: Array.from(readItems) }));
        }

        // Collect all read items
        collectReadItems();

        // Delete all read items
        if (markedReadIds.length > 0) {
            navigator.sendBeacon('/api/delete-read', JSON.stringify({ item_ids: markedReadIds }));
        }
    });

    // Also handle navigation via sidebar links
    document.querySelectorAll('.sidebar-nav a').forEach(link => {
        link.addEventListener('click', () => {
            // Mark any pending reads
            if (readItems.size > 0) {
                const ids = Array.from(readItems);
                readItems.clear();
                fetch('/api/mark-read', {
                    method: 'POST', headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ item_ids: ids })
                }).catch(() => { });
            }

            // Collect and delete read items
            collectReadItems();
            if (markedReadIds.length > 0) {
                fetch('/api/delete-read', {
                    method: 'POST', headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ item_ids: markedReadIds })
                }).catch(() => { });
            }
        });
    });
})();
