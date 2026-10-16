/**
 * Vault controls, after the OSRS bank: a quantity bar at the bottom (1 / 5 /
 * 10 / X / All) that sets what a left-click moves, right-click menus with every
 * amount, and a prompt for "X".
 *
 * The chosen quantity is a per-player convenience kept in localStorage.
 *
 * @module ui/vaultControls
 */

import { getItemById } from '../state/staticData.js';

const QTY_KEY = 'pq_vault_qty';
const X_KEY = 'pq_vault_x';

/** The left-click quantity mode: '1' | '5' | '10' | 'x' | 'all'. */
export function getQuantityMode() {
    try {
        const v = localStorage.getItem(QTY_KEY);
        if (['1', '5', '10', 'x', 'all'].includes(v)) return v;
    } catch (_) { /* storage blocked */ }
    return '1';
}

function setQuantityMode(mode) {
    try { localStorage.setItem(QTY_KEY, mode); } catch (_) { /* storage blocked */ }
}

/** The last "X" amount (0 if never set). */
export function getXAmount() {
    try { return Math.max(0, parseInt(localStorage.getItem(X_KEY), 10) || 0); } catch (_) { return 0; }
}

function setXAmount(n) {
    try { localStorage.setItem(X_KEY, String(n)); } catch (_) { /* storage blocked */ }
}

/**
 * How many a left-click moves, given how many are there.
 * @param {number} available - the stack (or vault entry) size
 * @returns {number}
 */
export function leftClickAmount(available) {
    const mode = getQuantityMode();
    if (mode === 'all') return available;
    if (mode === 'x') return Math.min(available, getXAmount() || 1);
    return Math.min(available, parseInt(mode, 10));
}

/** OSRS-style stack label: 99999, 100K, 10M. */
export function formatCount(n) {
    if (n >= 10_000_000) return { text: `${Math.floor(n / 1_000_000)}M`, color: '#4ade80' };
    if (n >= 100_000) return { text: `${Math.floor(n / 1000)}K`, color: '#ffffff' };
    return { text: String(n), color: '#facc15' };
}

// ─── Amount prompt ───────────────────────────────────────────────────────────

/**
 * Ask for an amount in a small box over the scene. Resolves to a positive
 * integer, or null if cancelled.
 * @param {string} title
 * @returns {Promise<number|null>}
 */
export function askAmount(title) {
    return new Promise((resolve) => {
        document.getElementById('vault-amount-prompt')?.remove();
        const box = document.createElement('div');
        box.id = 'vault-amount-prompt';
        box.style.cssText = 'position: fixed; inset: 0; z-index: 400; display: flex; align-items: center; justify-content: center; background: rgba(0,0,0,0.5);';
        box.innerHTML = `
            <div class="bg-win95-bgDark" style="border: 2px solid #808080; padding: 8px; width: 180px;">
                <div class="text-white font-bold mb-1" style="font-size: 8px;">${title}</div>
                <input type="number" min="1" step="1" class="w-full text-white mb-2"
                       style="font-size: 9px; background: #111; border: 2px solid #4a4a4a; padding: 2px 4px;">
                <div class="flex gap-1">
                    <button data-ok style="flex:1; font-size: 8px; padding: 3px; color: #fff; font-weight: bold; background: #166534; border-top: 2px solid #22c55e; border-left: 2px solid #22c55e; border-right: 2px solid #14532d; border-bottom: 2px solid #14532d;">OK</button>
                    <button data-cancel style="flex:1; font-size: 8px; padding: 3px; color: #fff; background: #2a2a2a; border-top: 2px solid #4a4a4a; border-left: 2px solid #4a4a4a; border-right: 2px solid #1a1a1a; border-bottom: 2px solid #1a1a1a;">Cancel</button>
                </div>
            </div>`;
        document.body.appendChild(box);
        const input = box.querySelector('input');
        const done = (value) => { box.remove(); resolve(value); };
        const submit = () => {
            const n = parseInt(input.value, 10);
            done(Number.isFinite(n) && n > 0 ? n : null);
        };
        box.querySelector('[data-ok]').onclick = submit;
        box.querySelector('[data-cancel]').onclick = () => done(null);
        box.addEventListener('click', (e) => { if (e.target === box) done(null); });
        input.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') submit();
            if (e.key === 'Escape') done(null);
        });
        if (getXAmount()) input.value = String(getXAmount());
        setTimeout(() => { input.focus(); input.select(); }, 0);
    });
}

// ─── Quantity bar ────────────────────────────────────────────────────────────

const BAR_BTN = 'font-size: 8px; padding: 2px 6px; font-weight: bold; color: #fff; cursor: pointer;';
const BTN_UP = 'background: #2a2a2a; border-top: 2px solid #4a4a4a; border-left: 2px solid #4a4a4a; border-right: 2px solid #1a1a1a; border-bottom: 2px solid #1a1a1a;';
const BTN_DOWN = 'background: #7c5c12; border-top: 2px solid #1a1a1a; border-left: 2px solid #1a1a1a; border-right: 2px solid #c9a227; border-bottom: 2px solid #c9a227;';

/**
 * The bottom bar: Quantity 1 / 5 / 10 / X / All, plus Deposit inventory.
 * @param {() => void} onDepositAll - deposit everything carried
 * @returns {HTMLElement}
 */
export function createQuantityBar(onDepositAll) {
    const bar = document.createElement('div');
    bar.className = 'flex items-center gap-1 mt-1 flex-wrap';

    const label = document.createElement('span');
    label.className = 'text-gray-400';
    label.style.fontSize = '8px';
    label.textContent = 'Quantity:';
    bar.appendChild(label);

    const buttons = [];
    const paint = () => {
        const mode = getQuantityMode();
        for (const b of buttons) b.style.cssText = BAR_BTN + (b.dataset.mode === mode ? BTN_DOWN : BTN_UP);
        const xBtn = buttons.find(b => b.dataset.mode === 'x');
        if (xBtn) xBtn.textContent = getXAmount() ? `X (${getXAmount()})` : 'X';
    };
    for (const [mode, text] of [['1', '1'], ['5', '5'], ['10', '10'], ['x', 'X'], ['all', 'All']]) {
        const b = document.createElement('button');
        b.dataset.mode = mode;
        b.textContent = text;
        b.title = mode === 'x' ? 'Click to set an amount' : `Left-click moves ${text === 'All' ? 'the whole stack' : text}`;
        b.addEventListener('click', async () => {
            if (mode === 'x') {
                const n = await askAmount('Amount for left-click:');
                if (!n) return;
                setXAmount(n);
            }
            setQuantityMode(mode);
            paint();
        });
        buttons.push(b);
        bar.appendChild(b);
    }
    paint();

    const spacer = document.createElement('span');
    spacer.style.flex = '1';
    bar.appendChild(spacer);

    const depositAll = document.createElement('button');
    depositAll.textContent = '⬇ Deposit inventory';
    depositAll.title = 'Put everything you carry into the vault (not what you wear or containers with things inside)';
    depositAll.style.cssText = BAR_BTN + BTN_UP;
    depositAll.addEventListener('click', onDepositAll);
    bar.appendChild(depositAll);

    return bar;
}

// ─── Right-click menus ───────────────────────────────────────────────────────

let _menu = null;

function closeMenu() {
    _menu?.remove();
    _menu = null;
}

document.addEventListener('click', (e) => {
    if (_menu && !e.target.closest('#vault-context-menu')) closeMenu();
});

/**
 * A right-click menu for the vault: every amount, then Examine.
 * @param {number} x
 * @param {number} y
 * @param {'Withdraw'|'Deposit'} verb
 * @param {string} itemId
 * @param {number} available - how many are there
 * @param {(n: number) => Promise<void>} move - move n of the item
 */
export function showVaultMenu(x, y, verb, itemId, available, move) {
    closeMenu();
    const name = getItemById(itemId)?.name ?? itemId;
    const options = [
        [`${verb}-1`, 1],
        [`${verb}-5`, 5],
        [`${verb}-10`, 10],
    ];
    if (getXAmount()) options.push([`${verb}-${getXAmount()}`, getXAmount()]);
    options.push([`${verb}-X…`, 'x'], [`${verb}-All`, available]);
    if (verb === 'Withdraw' && available > 1) options.push([`${verb}-All-but-1`, available - 1]);

    const menu = document.createElement('div');
    menu.id = 'vault-context-menu';
    menu.className = 'fixed bg-gray-800 border-2 border-gray-600 shadow-lg';
    menu.style.cssText += `left: ${x}px; top: ${y}px; min-width: 120px; z-index: 300;`;

    const header = document.createElement('div');
    header.className = 'px-2 py-1 text-yellow-400 font-bold';
    header.style.fontSize = '8px';
    header.textContent = name;
    menu.appendChild(header);

    const add = (label, onClick) => {
        const row = document.createElement('div');
        row.className = 'px-2 py-1 hover:bg-gray-700 cursor-pointer text-white';
        row.style.fontSize = '9px';
        row.textContent = label;
        row.addEventListener('click', async (e) => {
            e.stopPropagation();
            closeMenu();
            await onClick();
        });
        menu.appendChild(row);
    };
    for (const [label, amount] of options) {
        add(label, async () => {
            let n = amount;
            if (amount === 'x') {
                n = await askAmount(`${verb} how many ${name}?`);
                if (!n) return;
                setXAmount(n);
            }
            await move(Math.min(n, available));
        });
    }
    add('Examine', async () => {
        const item = getItemById(itemId);
        window.showMessage?.(`${name}: ${item?.description ?? 'Nothing special.'}`, 'info');
    });

    document.body.appendChild(menu);
    _menu = menu;
    const rect = menu.getBoundingClientRect();
    if (rect.right > window.innerWidth) menu.style.left = `${window.innerWidth - rect.width - 5}px`;
    if (rect.bottom > window.innerHeight) menu.style.top = `${window.innerHeight - rect.height - 5}px`;
}
