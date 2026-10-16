// The browser resource's helpers inside the page. Go calls this one function
// with Runtime.callFunctionOn: `this` is the document or the element a step
// acts on, `op` names the helper and the rest are its arguments. It is
// evaluated afresh on every call, so a navigation never leaves it missing.
function (op, ...args) {
	// Text as a person reads it: runs of whitespace (including the
	// non-breaking spaces editors insert) are one space.
	const norm = (s) => String(s == null ? '' : s).replace(/\s+/g, ' ').trim();
	const squash = (s) => String(s == null ? '' : s).replace(/\s+/g, '').toLowerCase();

	const visible = (el) => {
		if (!el || !el.isConnected) return false;
		// An option of a closed <select> has no box of its own.
		if (el.tagName === 'OPTION') return visible(el.closest('select, datalist'));
		if (el.checkVisibility && !el.checkVisibility({ visibilityProperty: true })) return false;
		const r = el.getBoundingClientRect();
		return r.width > 0 && r.height > 0;
	};

	const role = (el) => (el.getAttribute && el.getAttribute('role')) || '';
	const roleIn = (el, roles) => roles.includes(role(el));

	const TEXT_INPUT = 'input:not([type=checkbox]):not([type=radio]):not([type=button]):not([type=submit])' +
		':not([type=reset]):not([type=hidden]):not([type=file]):not([type=image]):not([type=range]):not([type=color]), textarea';
	const editable = (el) => el.matches(TEXT_INPUT) || el.isContentEditable ||
		roleIn(el, ['textbox', 'searchbox', 'combobox', 'spinbutton']);
	const isCheckbox = (el) => el.matches('input[type=checkbox]') || roleIn(el, ['checkbox', 'switch', 'menuitemcheckbox']);
	const isChoice = (el) => el.matches('input[type=radio], option') || roleIn(el, ['radio', 'option', 'menuitemradio', 'tab', 'treeitem']);
	const isSelect = (el) => el.matches('select') || roleIn(el, ['combobox', 'listbox']);
	const INTERACTIVE = 'a[href], button, input, select, textarea, summary, option, label, [contenteditable=""], [contenteditable=true], [tabindex]';
	const INTERACTIVE_ROLES = ['button', 'link', 'checkbox', 'radio', 'switch', 'tab', 'option', 'menuitem', 'menuitemcheckbox',
		'menuitemradio', 'textbox', 'searchbox', 'combobox', 'listbox', 'slider', 'spinbutton', 'treeitem'];
	const interactive = (el) => el.matches(INTERACTIVE) || roleIn(el, INTERACTIVE_ROLES);

	// What a step names, resolved to the element it acts on. A label stands
	// for its control, so "checks "Austria"" finds the checkbox whichever of
	// the two carries the text.
	const control = (el, pred) => {
		if (pred(el)) return el;
		const label = el.tagName === 'LABEL' ? el : el.closest('label');
		if (label && label.control && pred(label.control)) return label.control;
		return null;
	};
	const KINDS = {
		click: (el) => el,
		any: (el) => el.closest(INTERACTIVE) || el,
		field: (el) => control(el, editable),
		checkbox: (el) => control(el, isCheckbox),
		choice: (el) => control(el, isChoice),
		select: (el) => control(el, isSelect),
		// The paragraph a piece of text sits in, or the field itself.
		block: (el) => (el.matches(TEXT_INPUT) ? el : blockOf(el)),
	};
	const NOUNS = {
		click: 'element', any: 'element', field: 'field', checkbox: 'checkbox',
		choice: 'option or radio button', select: 'select box', block: 'text',
	};

	const SKIP = new Set(['SCRIPT', 'STYLE', 'NOSCRIPT', 'TEMPLATE', 'HEAD', 'TITLE', 'META', 'LINK']);

	// The deepest visible elements whose text is target, or contains it. The
	// text is what the page renders, or what its source says when the two
	// differ only in case: CSS text-transform shows "Apply" as "APPLY", and
	// either name finds it. Text in hidden children never counts.
	const byText = (target, contains) => {
		const want = norm(target);
		const loose = squash(target);
		const found = [];
		if (!document.body || !want) return found;
		const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_ELEMENT);
		for (let el = walker.currentNode; el; el = walker.nextNode()) {
			if (SKIP.has(el.tagName)) continue;
			// textContent is cheap and holds at least the rendered text, so
			// it rules out most of the page before innerText lays it out.
			if (!squash(el.textContent).includes(loose)) continue;
			if (!visible(el)) continue;
			const shown = norm(el.innerText);
			const source = norm(el.textContent);
			const texts = shown.toLowerCase() === source.toLowerCase() ? [shown, source] : [shown];
			if (texts.some((text) => (contains ? text.includes(want) : text === want))) found.push(el);
		}
		return found.filter((el) => !found.some((other) => other !== el && el.contains(other)));
	};

	// The block a piece of text sits in: the paragraph, not the <strong>
	// inside it.
	const blockOf = (el) => {
		let cur = el;
		while (cur && cur !== document.body) {
			const display = getComputedStyle(cur).display;
			if (!display.startsWith('inline') && display !== 'contents') return cur;
			cur = cur.parentElement;
		}
		return el;
	};

	const describe = (el) => {
		if (!el || !el.tagName) return 'nothing';
		let s = el.tagName.toLowerCase();
		if (el.id) s += '#' + el.id;
		const cls = typeof el.className === 'string' ? el.className.trim().split(/\s+/).filter(Boolean).slice(0, 2) : [];
		if (cls.length) s += '.' + cls.join('.');
		const text = norm(el.innerText || el.value || el.getAttribute('aria-label') || '');
		if (text) s += ' "' + (text.length > 40 ? text.slice(0, 40) + '…' : text) + '"';
		return s;
	};

	const disabled = (el) => {
		if (el.matches(':disabled')) return true;
		return !!el.closest('[aria-disabled=true]');
	};

	const checked = (el) => {
		if (el.matches('input[type=checkbox], input[type=radio]')) return el.checked;
		if (el.tagName === 'OPTION') return el.selected;
		const attr = el.getAttribute('aria-checked') || el.getAttribute('aria-selected');
		return attr === 'true';
	};

	const ops = {
		// locate(kind, target, ...candidates): the one element target names.
		// The candidates are what the accessibility tree calls target; when
		// it has none, the visible text decides.
		locate(kind, target, ...candidates) {
			const want = KINDS[kind];
			const pick = (els) => {
				const out = [];
				for (const el of els) {
					const c = el && want(el);
					if (c && !out.includes(c) && visible(c)) out.push(c);
				}
				return out;
			};
			let found = pick(candidates);
			if (found.length === 0) {
				found = pick(byText(target, false));
			}
			if (found.length === 0 && kind === 'block') {
				found = pick(byText(target, true));
			}
			// "Apply" can be both a button and a heading above it; a person
			// clicking "Apply" means the button.
			if (found.length > 1) {
				const usable = found.filter(interactive);
				if (usable.length > 0) found = usable;
			}
			if (found.length === 1) return found[0];
			if (found.length === 0) return 'no ' + NOUNS[kind] + ' "' + target + '" on the page';
			return found.length + ' elements match "' + target + '": ' + found.slice(0, 5).map(describe).join(', ');
		},

		// text(): everything the page shows, as one string.
		text() {
			const parts = [document.body ? document.body.innerText : ''];
			for (const el of document.querySelectorAll('input, textarea')) {
				if (['hidden', 'password', 'checkbox', 'radio', 'file', 'image'].includes(el.type)) continue;
				if (visible(el) && el.value) parts.push(el.value);
			}
			return norm(parts.join('\n'));
		},

		// point(): where to click this element, once it is in view, enabled
		// and not under something else.
		point() {
			const el = this;
			if (disabled(el)) return { error: describe(el) + ' is disabled' };
			let target = el;
			const hits = (t) => {
				const r = t.getBoundingClientRect();
				const x = r.left + r.width / 2;
				const y = r.top + r.height / 2;
				const hit = document.elementFromPoint(x, y);
				const ok = !!hit && (hit === t || t.contains(hit) || (hit.tagName === 'LABEL' && hit.control === el) ||
					(el.labels && Array.from(el.labels).some((l) => l === hit || l.contains(hit))));
				return { x, y, ok, hit };
			};
			el.scrollIntoView({ block: 'center', inline: 'center', behavior: 'instant' });
			let at = hits(target);
			// A styled checkbox hides the input and shows its label.
			if (!at.ok && el.labels && el.labels.length > 0) {
				const label = Array.from(el.labels).find(visible);
				if (label) {
					label.scrollIntoView({ block: 'center', inline: 'center', behavior: 'instant' });
					target = label;
					at = hits(label);
				}
			}
			if (!at.ok) return { error: describe(el) + ' is covered by ' + describe(at.hit) };
			return { x: at.x, y: at.y };
		},

		// arm(): from now until disarm, a click counts only if it lands on
		// this element or its label. The page can move between finding where
		// to click and the click arriving (a status line appears above, a
		// banner slides in); a click that lands elsewhere is stopped before
		// anything sees it, and the step tries again.
		arm() {
			const el = this;
			const own = (t) => !!t && (t === el || el.contains(t) ||
				(!!el.labels && Array.from(el.labels).some((l) => l === t || l.contains(t))));
			const state = { hit: null, on: null, events: ['pointerdown', 'mousedown', 'pointerup', 'mouseup', 'click'] };
			state.guard = (e) => {
				if (state.hit === null) {
					state.hit = own(e.target);
					state.on = e.target;
				}
				if (!state.hit) {
					e.preventDefault();
					e.stopImmediatePropagation();
				}
			};
			for (const type of state.events) window.addEventListener(type, state.guard, true);
			window.__tomatoClick = state;
			return '';
		},

		// disarm(): '' when the click landed where it was meant to, or where
		// it landed instead.
		disarm() {
			const state = window.__tomatoClick;
			if (!state) return '';
			delete window.__tomatoClick;
			for (const type of state.events) window.removeEventListener(type, state.guard, true);
			if (state.hit === false) return 'the click landed on ' + describe(state.on) + ' instead';
			if (state.hit === null) return 'the click reached nothing on the page';
			return '';
		},

		disabled() {
			return disabled(this);
		},

		checked() {
			return checked(this);
		},

		describe() {
			return describe(this);
		},

		// focusAll(): focus a field and select what it holds, so typing
		// replaces it.
		focusAll() {
			const el = this;
			if (disabled(el)) return describe(el) + ' is disabled';
			if (el.readOnly) return describe(el) + ' is read-only';
			el.scrollIntoView({ block: 'center', inline: 'center', behavior: 'instant' });
			el.focus();
			if (el.matches(TEXT_INPUT)) {
				el.select();
				return '';
			}
			const range = document.createRange();
			range.selectNodeContents(el);
			const sel = window.getSelection();
			sel.removeAllRanges();
			sel.addRange(range);
			return '';
		},

		// caretAtEnd(): put the caret after the last character of this
		// element, in whatever editor holds it.
		caretAtEnd() {
			const el = this;
			if (el.matches(TEXT_INPUT)) {
				if (disabled(el)) return describe(el) + ' is disabled';
				el.scrollIntoView({ block: 'center', inline: 'center', behavior: 'instant' });
				el.focus();
				const end = el.value.length;
				el.setSelectionRange(end, end);
				return '';
			}
			if (!el.isContentEditable) return describe(el) + ' cannot be edited';
			let host = el;
			while (host.parentElement && host.parentElement.isContentEditable) host = host.parentElement;
			el.scrollIntoView({ block: 'center', inline: 'center', behavior: 'instant' });
			host.focus();
			const range = document.createRange();
			range.selectNodeContents(el);
			range.collapse(false);
			const sel = window.getSelection();
			sel.removeAllRanges();
			sel.addRange(range);
			return '';
		},

		// focused(): what has focus, or '' when nothing that takes text does.
		focused() {
			const el = document.activeElement;
			if (!el || el === document.body || el === document.documentElement) return '';
			return describe(el);
		},

		// selectOption(text): choose an option of a native <select> by its
		// text or value, firing the events a person's choice fires.
		selectOption(text) {
			const sel = this.tagName === 'OPTION' ? this.closest('select') : this;
			if (!sel || sel.tagName !== 'SELECT') return 'not a native select';
			if (disabled(sel)) return describe(sel) + ' is disabled';
			const option = this.tagName === 'OPTION' ? this :
				Array.from(sel.options).find((o) => norm(o.text) === norm(text) || o.value === text);
			if (!option) {
				return describe(sel) + ' has no option "' + text + '" (it has: ' +
					Array.from(sel.options).map((o) => '"' + norm(o.text) + '"').join(', ') + ')';
			}
			if (option.disabled) return 'option "' + norm(option.text) + '" is disabled';
			sel.value = option.value;
			option.selected = true;
			sel.dispatchEvent(new Event('input', { bubbles: true }));
			sel.dispatchEvent(new Event('change', { bubbles: true }));
			return '';
		},

		isNativeSelect() {
			return this.tagName === 'SELECT' || (this.tagName === 'OPTION' && !!this.closest('select'));
		},
	};

	return ops[op].apply(this, args);
}
