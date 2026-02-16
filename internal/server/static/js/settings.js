// Infovore Settings Page JS
(function () {
    'use strict';

    // ----- DOM References -----
    var pollingInterval   = document.getElementById('pollingInterval');
    var opmlFile          = document.getElementById('opmlFile');
    var importBtn         = document.getElementById('importBtn');
    var cleanupBtn        = document.getElementById('cleanupBtn');

    var kalshiApiKeyId    = document.getElementById('kalshiApiKeyId');
    var kalshiPrivateKey  = document.getElementById('kalshiPrivateKey');
    var kalshiCategories  = document.getElementById('kalshiCategories');
    var kalshiScanInterval = document.getElementById('kalshiScanInterval');
    var testKalshiBtn     = document.getElementById('testKalshiBtn');
    var triggerScanBtn    = document.getElementById('triggerScanBtn');

    var dbUrlInput        = document.getElementById('dbUrlInput');
    var saveDbBtn         = document.getElementById('saveDbBtn');

    var saveRssBtn        = document.getElementById('saveRssBtn');
    var saveKalshiBtn     = document.getElementById('saveKalshiBtn');
    var toast             = document.getElementById('toast');
    var toastMessage      = document.getElementById('toastMessage');

    // ----- Constants -----
    var ALL_CATEGORIES = [
        'Climate and Weather',
        'Companies',
        'Crypto',
        'Economics',
        'Elections',
        'Entertainment',
        'Financials',
        'Health',
        'Mentions',
        'Politics',
        'Science and Technology',
        'Social',
        'Sports',
        'World'
    ];

    // Track whether the user has touched the private key field
    var privateKeyDirty = false;

    // ----- Toast Helper -----
    function showToast(msg, duration) {
        if (!toast || !toastMessage) return;
        toastMessage.textContent = msg;
        toast.classList.add('visible');
        clearTimeout(window._toastTimer);
        window._toastTimer = setTimeout(function () {
            toast.classList.remove('visible');
        }, duration || 3000);
    }

    // ----- Category Checkboxes -----
    function renderCategories(selectedCSV) {
        if (!kalshiCategories) return;

        var selected = {};
        if (selectedCSV) {
            selectedCSV.split(',').forEach(function (cat) {
                var trimmed = cat.trim();
                if (trimmed) selected[trimmed] = true;
            });
        }

        kalshiCategories.innerHTML = '';

        ALL_CATEGORIES.forEach(function (cat) {
            var label = document.createElement('label');
            label.className = 'category-checkbox';

            var checkbox = document.createElement('input');
            checkbox.type = 'checkbox';
            checkbox.name = 'kalshi_category';
            checkbox.value = cat;
            if (selected[cat]) checkbox.checked = true;

            var span = document.createElement('span');
            span.textContent = cat;

            label.appendChild(checkbox);
            label.appendChild(span);
            kalshiCategories.appendChild(label);
        });
    }

    function getSelectedCategories() {
        if (!kalshiCategories) return '';
        var checked = kalshiCategories.querySelectorAll('input[name="kalshi_category"]:checked');
        var values = [];
        checked.forEach(function (cb) {
            values.push(cb.value);
        });
        return values.join(',');
    }

    // ----- Load Settings -----
    function loadSettings() {
        fetch('/api/settings')
            .then(function (res) { return res.json(); })
            .then(function (data) {
                if (pollingInterval && data.polling_interval != null) {
                    pollingInterval.value = data.polling_interval;
                }

                if (kalshiApiKeyId && data.kalshi_api_key_id != null) {
                    kalshiApiKeyId.value = data.kalshi_api_key_id;
                }

                if (kalshiPrivateKey) {
                    kalshiPrivateKey.value = '';
                    privateKeyDirty = false;
                    if (data.kalshi_private_key_configured) {
                        kalshiPrivateKey.placeholder = '\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022';
                        // Add a hint below the textarea if not already present
                        var hint = kalshiPrivateKey.parentElement.querySelector('.key-configured-hint');
                        if (!hint) {
                            hint = document.createElement('small');
                            hint.className = 'key-configured-hint';
                            hint.textContent = 'Key is configured. Enter a new key to replace it.';
                            kalshiPrivateKey.parentElement.appendChild(hint);
                        }
                    } else {
                        kalshiPrivateKey.placeholder = 'Paste your private key here';
                        var existingHint = kalshiPrivateKey.parentElement.querySelector('.key-configured-hint');
                        if (existingHint) existingHint.remove();
                    }
                }

                if (kalshiScanInterval && data.kalshi_scan_interval_hours != null) {
                    kalshiScanInterval.value = data.kalshi_scan_interval_hours;
                }

                renderCategories(data.kalshi_categories || '');

                if (dbUrlInput && data.db_url != null) {
                    dbUrlInput.value = data.db_url;
                }
            })
            .catch(function (err) {
                console.error('Failed to load settings:', err);
                showToast('Failed to load settings');
            });
    }

    // ----- Track Private Key Changes -----
    if (kalshiPrivateKey) {
        kalshiPrivateKey.addEventListener('input', function () {
            privateKeyDirty = true;
        });
    }

    // ----- Save RSS Settings -----
    if (saveRssBtn) {
        saveRssBtn.onclick = async function () {
            var payload = {};

            if (pollingInterval) {
                var interval = parseInt(pollingInterval.value, 10);
                if (!isNaN(interval) && interval > 0) {
                    payload.polling_interval = interval;
                }
            }

            showToast('Saving RSS settings...');

            try {
                var res = await fetch('/api/settings', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload)
                });
                if (res.ok) {
                    showToast('RSS settings saved');
                } else {
                    var data = await res.json();
                    showToast(data.error || 'Failed to save settings');
                }
            } catch (e) {
                showToast('Error saving settings');
            }
        };
    }

    // ----- Save Kalshi Settings -----
    if (saveKalshiBtn) {
        saveKalshiBtn.onclick = async function () {
            var payload = {};

            if (kalshiApiKeyId) {
                payload.kalshi_api_key_id = kalshiApiKeyId.value.trim();
            }

            // Only send the private key if the user actually typed something new
            if (kalshiPrivateKey && privateKeyDirty) {
                var keyVal = kalshiPrivateKey.value.trim();
                if (keyVal) {
                    payload.kalshi_private_key = keyVal;
                }
            }

            payload.kalshi_categories = getSelectedCategories();

            if (kalshiScanInterval) {
                var scanHours = parseInt(kalshiScanInterval.value, 10);
                if (!isNaN(scanHours) && scanHours > 0) {
                    payload.kalshi_scan_interval_hours = scanHours;
                }
            }

            showToast('Saving Kalshi settings...');

            try {
                var res = await fetch('/api/settings', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload)
                });
                if (res.ok) {
                    showToast('Kalshi settings saved');
                    // Reset private key dirty state after successful save
                    privateKeyDirty = false;
                    if (kalshiPrivateKey) {
                        kalshiPrivateKey.value = '';
                        kalshiPrivateKey.placeholder = '\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022';
                        var hint = kalshiPrivateKey.parentElement.querySelector('.key-configured-hint');
                        if (!hint && payload.kalshi_private_key) {
                            hint = document.createElement('small');
                            hint.className = 'key-configured-hint';
                            hint.textContent = 'Key is configured. Enter a new key to replace it.';
                            kalshiPrivateKey.parentElement.appendChild(hint);
                        }
                    }
                } else {
                    var data = await res.json();
                    showToast(data.error || 'Failed to save settings');
                }
            } catch (e) {
                showToast('Error saving settings');
            }
        };
    }

    // ----- Test Kalshi Credentials -----
    if (testKalshiBtn) {
        testKalshiBtn.onclick = async function () {
            testKalshiBtn.disabled = true;
            showToast('Testing Kalshi credentials...');

            try {
                var res = await fetch('/api/settings/test-kalshi', { method: 'POST' });
                var data = await res.json();
                if (res.ok && data.status === 'ok') {
                    showToast('Kalshi credentials are valid');
                } else {
                    showToast(data.error || 'Kalshi credential test failed');
                }
            } catch (e) {
                showToast('Error testing Kalshi credentials');
            } finally {
                testKalshiBtn.disabled = false;
            }
        };
    }

    // ----- Trigger Kalshi Scan -----
    if (triggerScanBtn) {
        triggerScanBtn.onclick = async function () {
            triggerScanBtn.disabled = true;
            showToast('Triggering Kalshi scan...', 30000);

            try {
                var res = await fetch('/api/kalshi/refresh', { method: 'POST' });
                var data = await res.json();
                if (res.ok) {
                    showToast('Scan complete: ' + (data.message || 'done'));
                } else {
                    showToast(data.error || 'Scan failed');
                }
            } catch (e) {
                showToast('Error triggering scan');
            } finally {
                triggerScanBtn.disabled = false;
            }
        };
    }

    // ----- Import OPML -----
    if (importBtn) {
        importBtn.onclick = async function () {
            if (!opmlFile || !opmlFile.files.length) {
                showToast('Select a file first');
                return;
            }

            var formData = new FormData();
            formData.append('opml', opmlFile.files[0]);
            showToast('Importing...', 15000);

            try {
                var res = await fetch('/api/import-opml', { method: 'POST', body: formData });
                var data = await res.json();
                if (res.ok) {
                    showToast('Imported ' + data.imported + ' of ' + data.total + ' feeds', 5000);
                } else {
                    showToast(data.error || 'Import failed');
                }
            } catch (e) {
                showToast('Import failed');
            }
        };
    }

    // ----- Cleanup Read Items -----
    if (cleanupBtn) {
        cleanupBtn.onclick = async function () {
            showToast('Cleaning up...', 5000);

            try {
                var res = await fetch('/api/cleanup', { method: 'POST' });
                var data = await res.json();
                if (res.ok) {
                    showToast('Deleted ' + (data.deleted || 0) + ' items');
                } else {
                    showToast(data.error || 'Cleanup failed');
                }
            } catch (e) {
                showToast('Cleanup failed');
            }
        };
    }

    // ----- Save Database URL -----
    if (saveDbBtn) {
        saveDbBtn.onclick = async function () {
            var dbUrl = dbUrlInput ? dbUrlInput.value.trim() : '';

            if (dbUrl && !dbUrl.startsWith('postgres://') && !dbUrl.startsWith('postgresql://') && !dbUrl.startsWith('sqlite://')) {
                showToast('Invalid URL. Use postgres://... or sqlite://... or leave empty for default SQLite');
                return;
            }

            showToast('Saving database settings...');

            try {
                var res = await fetch('/api/database-settings', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ db_url: dbUrl })
                });
                var data = await res.json();
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

    // ----- Initialize on Page Load -----
    loadSettings();
})();
