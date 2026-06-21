/**
 * CODEX Validation Entry Point
 */

// Import styles
import './styles.css';

// Detect and apply theme
const savedTheme = localStorage.getItem('theme') || 'dark';
document.documentElement.setAttribute('data-theme', savedTheme);
console.log(`🎯 CODEX Validation loaded (Theme: ${savedTheme})`);

async function runValidation() {
    const results = document.getElementById('results');
    results.style.display = 'block';

    // Clear cleanup results when running validation
    document.getElementById('cleanup-results').innerHTML = '';

    try {
        const response = await fetch('/api/validation/run', { method: 'POST' });
        const data = await response.json();

        document.getElementById('totalFiles').textContent = data.stats.total_files;
        document.getElementById('errors').textContent = data.stats.error_count;
        document.getElementById('warnings').textContent = data.stats.warning_count;
        document.getElementById('info').textContent = data.stats.info_count;

        const issuesDiv = document.getElementById('issues');
        issuesDiv.innerHTML = '';

        if (data.issues.length === 0) {
            issuesDiv.innerHTML = '<div class="codex-section win95-inset pixel-clip" style="text-align: center; color: #50fa7b; padding: 20px;">✓ NO ISSUES FOUND!</div>';
        } else {
            data.issues.forEach(issue => {
                const div = document.createElement('div');
                div.className = 'codex-section win95-inset pixel-clip mb-10';
                div.style.padding = '12px';
                div.style.borderLeft = '4px solid ' + (issue.type === 'error' ? '#ff5555' : issue.type === 'warning' ? '#f1fa8c' : '#8be9fd');

                let icon = issue.type === 'error' ? '❌' : issue.type === 'warning' ? '⚠️' : 'ℹ️';
                let color = issue.type === 'error' ? '#ff5555' : issue.type === 'warning' ? '#f1fa8c' : '#8be9fd';

                div.innerHTML =
                    '<div style="font-weight: bold; margin-bottom: 5px; color: ' + color + ';">' + icon + ' [' + issue.category.toUpperCase() + '] ' + issue.message + '</div>' +
                    '<div class="text-muted" style="font-size: 12px;">' + issue.file + (issue.field ? ' → ' + issue.field : '') + '</div>';
                issuesDiv.appendChild(div);
            });
        }
    } catch (error) {
        alert('Error running validation: ' + error.message);
    }
}

async function runCleanup(dryRun) {
    const results = document.getElementById('results');
    results.style.display = 'block';

    // Clear previous results
    document.getElementById('issues').innerHTML = '';
    const cleanupResultsDiv = document.getElementById('cleanup-results');
    cleanupResultsDiv.innerHTML = '<div style="text-align: center; padding: 20px;">Processing cleanup...</div>';

    try {
        const url = '/api/validation/cleanup' + (dryRun ? '?dry_run=true' : '');
        const response = await fetch(url, { method: 'POST' });
        const data = await response.json();

        // Update stats
        document.getElementById('totalFiles').textContent = data.files_processed;
        document.getElementById('errors').textContent = '0';
        document.getElementById('warnings').textContent = '0';
        document.getElementById('info').textContent = data.changes.length;

        cleanupResultsDiv.innerHTML = '';

        if (dryRun) {
            const header = document.createElement('div');
            header.className = 'codex-section win95-inset pixel-clip mb-10';
            header.style = 'background: #f1fa8c; color: #000; padding: 12px; font-weight: bold;';
            header.textContent = '👁️ PREVIEW MODE - No files were modified';
            cleanupResultsDiv.appendChild(header);
        } else {
            const header = document.createElement('div');
            header.className = 'codex-section win95-inset pixel-clip mb-10';
            header.style = 'background: #50fa7b; color: #000; padding: 12px; font-weight: bold;';
            header.textContent = `✅ CLEANUP COMPLETE - ${data.files_modified} files modified`;
            cleanupResultsDiv.appendChild(header);
        }

        if (data.changes.length === 0) {
            cleanupResultsDiv.innerHTML += '<div class="codex-section win95-inset pixel-clip" style="text-align: center; color: #50fa7b; padding: 20px;">✓ NO CHANGES NEEDED!</div>';
        } else {
            // Group changes by file
            const changesByFile = {};
            data.changes.forEach(change => {
                if (!changesByFile[change.file]) {
                    changesByFile[change.file] = [];
                }
                changesByFile[change.file].push(change);
            });

            // Display changes grouped by file
            Object.keys(changesByFile).forEach(file => {
                const fileDiv = document.createElement('div');
                fileDiv.className = 'codex-section win95-inset pixel-clip mb-10';
                fileDiv.style.padding = '15px';

                const fileHeader = document.createElement('div');
                fileHeader.className = 'text-highlighted';
                fileHeader.style = 'font-weight: bold; margin-bottom: 10px;';
                fileHeader.textContent = file;
                fileDiv.appendChild(fileHeader);

                changesByFile[file].forEach(change => {
                    const changeDiv = document.createElement('div');
                    changeDiv.style = 'margin-left: 15px; margin-bottom: 5px; font-size: 14px;';

                    let color = '#f8f8f2';
                    let icon = '•';
                    if (change.type === 'added') {
                        color = '#50fa7b';
                        icon = '+';
                    } else if (change.type === 'removed') {
                        color = '#ff5555';
                        icon = '-';
                    } else if (change.type === 'fixed') {
                        color = '#f1fa8c';
                        icon = '!';
                    } else if (change.type === 'reordered') {
                        color = '#bd93f9';
                        icon = '↔';
                    }

                    changeDiv.innerHTML = `<span style="color: ${color}; font-weight: bold;">${icon}</span> ${change.message}`;
                    fileDiv.appendChild(changeDiv);
                });

                cleanupResultsDiv.appendChild(fileDiv);
            });
        }
    } catch (error) {
        cleanupResultsDiv.innerHTML = '<div style="color: #ff5555; padding: 20px;">Error running cleanup: ' + error.message + '</div>';
    }
}

async function runSchemaCheck() {
    const results = document.getElementById('results');
    results.style.display = 'block';

    document.getElementById('issues').innerHTML = '';
    document.getElementById('cleanup-results').innerHTML = '';
    const out = document.getElementById('schema-results');
    out.innerHTML = '<div style="text-align: center; padding: 20px;">Running schema check...</div>';

    try {
        const response = await fetch('/api/validation/schema', { method: 'POST' });
        const data = await response.json();

        const okCount = (data.ok_files || []).length;
        const shapeErrs = data.shape_errors || [];
        const refErrs = data.ref_errors || [];
        const refWarns = data.ref_warnings || [];

        document.getElementById('totalFiles').textContent = okCount;
        document.getElementById('errors').textContent = shapeErrs.length + refErrs.length;
        document.getElementById('warnings').textContent = refWarns.length;
        document.getElementById('info').textContent = '0';

        out.innerHTML = '';

        if (shapeErrs.length === 0 && refErrs.length === 0 && refWarns.length === 0) {
            out.innerHTML = '<div class="codex-section win95-inset pixel-clip" style="text-align: center; color: #50fa7b; padding: 20px;">✓ SCHEMA CHECK PASSED (' + okCount + ' files)</div>';
            return;
        }

        const renderBlock = (lines, color, label) => {
            if (lines.length === 0) return;
            const div = document.createElement('div');
            div.className = 'codex-section win95-inset pixel-clip mb-10';
            div.style.padding = '12px';
            div.style.borderLeft = '4px solid ' + color;
            const header = document.createElement('div');
            header.style = 'font-weight: bold; margin-bottom: 8px; color: ' + color + ';';
            header.textContent = label + ' (' + lines.length + ')';
            div.appendChild(header);
            lines.forEach(line => {
                const pre = document.createElement('pre');
                pre.style = 'white-space: pre-wrap; font-size: 12px; margin: 4px 0; color: #f8f8f2;';
                pre.textContent = line;
                div.appendChild(pre);
            });
            out.appendChild(div);
        };

        renderBlock(shapeErrs, '#ff5555', '❌ SHAPE ERRORS');
        renderBlock(refErrs, '#ff5555', '❌ REFERENCE ERRORS');
        renderBlock(refWarns, '#f1fa8c', '⚠️ MISSING NPCs (need to be created)');
    } catch (error) {
        out.innerHTML = '<div style="color: #ff5555; padding: 20px;">Error running schema check: ' + error.message + '</div>';
    }
}

// Expose functions to window for onclick handlers
window.runValidation = runValidation;
window.runCleanup = runCleanup;
window.runSchemaCheck = runSchemaCheck;

console.log('🎯 CODEX Validation loaded');
