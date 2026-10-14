// Vista del penell flotant: pinta l'ànim que arriba pel pont (l'angle i
// l'animació els posa mascot.css segons data-mood). Tot el penell
// s'arrossega: mantenir premut i moure (més de DRAG_PX) mou la finestra amb
// pointer events i IPC (-webkit-app-region no deixa clicar el mateix
// element); un clic curt torna a la finestra principal, o desplega el
// puntet si és plegat. En passar-hi per sobre surten tres controls: plegar,
// enganxar a un racó i amagar. Esc el plega.
(() => {
    const bridge = window.gregalMascot;
    const stage = document.querySelector('.mascot-stage');
    const btn = document.getElementById('mascot');
    const svg = btn && btn.querySelector('.mascot-svg');
    const ctlCollapse = document.getElementById('mascotCollapse');
    const ctlSnap = document.getElementById('mascotSnap');
    const ctlHide = document.getElementById('mascotHide');
    const MOODS = ['idle', 'working', 'waiting', 'done', 'error'];
    const DRAG_PX = 4;
    if (!bridge || !stage || !btn) return;
    let collapsed = false;
    let label = 'Gregal';
    let strings = {};
    let press = null;
    let suppressUntil = 0;
    let frame = 0;

    const setLabel = (el, text) => { if (el && text) { el.setAttribute('aria-label', text); el.title = text; } };
    function paintLabels() {
        const hint = collapsed ? strings.expand : strings.drag;
        const text = hint ? label + ' — ' + hint : label;
        btn.setAttribute('aria-label', text);
        btn.title = text;
        if (svg) svg.setAttribute('aria-label', label);
        setLabel(ctlCollapse, strings.collapse);
        setLabel(ctlSnap, strings.snap);
        setLabel(ctlHide, strings.hide);
    }

    bridge.onState(s => {
        stage.dataset.mood = MOODS.includes(s.mood) ? s.mood : 'idle';
        collapsed = !!s.collapsed;
        stage.dataset.collapsed = collapsed ? '1' : '0';
        label = s.label || 'Gregal';
        strings = s.strings || {};
        paintLabels();
    });

    const inControls = target => !!(target && target.closest && target.closest('.mascot-ctrls'));
    const delta = e => ({ dx: Math.round(e.screenX - press.sx), dy: Math.round(e.screenY - press.sy) });

    stage.addEventListener('pointerdown', e => {
        if (e.button !== 0 || inControls(e.target)) return;
        press = { id: e.pointerId, sx: e.screenX, sy: e.screenY, moved: false, last: { dx: 0, dy: 0 } };
        try { stage.setPointerCapture(e.pointerId); } catch (_) {}
    });
    stage.addEventListener('pointermove', e => {
        if (!press || e.pointerId !== press.id) return;
        const d = delta(e);
        press.last = d;
        if (!press.moved) {
            if (Math.abs(d.dx) < DRAG_PX && Math.abs(d.dy) < DRAG_PX) return;
            press.moved = true;
            stage.classList.add('dragging');
            bridge.drag('start', 0, 0);
        }
        // Un missatge per fotograma n'hi ha prou.
        if (frame) return;
        frame = requestAnimationFrame(() => {
            frame = 0;
            if (press && press.moved) bridge.drag('move', press.last.dx, press.last.dy);
        });
    });
    function finish(e) {
        if (!press || (e && e.pointerId !== press.id)) return;
        const p = press;
        press = null;
        if (frame) { cancelAnimationFrame(frame); frame = 0; }
        try { stage.releasePointerCapture(p.id); } catch (_) {}
        if (!p.moved) return;
        stage.classList.remove('dragging');
        bridge.drag('end', p.last.dx, p.last.dy);
        // El clic que ve després de deixar anar no és un clic.
        suppressUntil = Date.now() + 300;
    }
    stage.addEventListener('pointerup', e => {
        if (press && e.pointerId === press.id && press.moved) press.last = delta(e);
        finish(e);
    });
    stage.addEventListener('pointercancel', finish);

    btn.addEventListener('click', e => {
        if (Date.now() < suppressUntil) { suppressUntil = 0; e.preventDefault(); return; }
        if (collapsed) bridge.setCollapsed(false);
        else bridge.activate();
    });
    if (ctlCollapse) ctlCollapse.addEventListener('click', e => { e.stopPropagation(); bridge.setCollapsed(true); });
    if (ctlSnap) ctlSnap.addEventListener('click', e => { e.stopPropagation(); bridge.snap(); });
    if (ctlHide) ctlHide.addEventListener('click', e => { e.stopPropagation(); bridge.hide(); });
    document.addEventListener('keydown', e => {
        if (e.key === 'Escape' && !collapsed) { e.preventDefault(); bridge.setCollapsed(true); }
    });
})();
