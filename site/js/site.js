/* The site's own script: the theme switch, search, copy buttons and the phone menu. Every string from the index or the address is shown as text. */
(() => {
  const root = document.documentElement;
  const THEME_KEY = "venom-theme";
  const THEMES = ["auto", "light", "dark"];
  const MAX_HITS = 8;

  const DEFAULT_UI = {
    copy: "Copy",
    copied: "Copied",
    copy_failed: "Copy failed",
    copy_aria: "Copy this code",
    search_aria: "Search the documentation",
    search_label: "Search",
    search_placeholder: "Search the documentation",
    search_empty: "Type to search the documentation.",
    search_none: "Nothing matches that search.",
    search_loading: "Loading the search index...",
    search_unavailable: "Search is not available right now. Use the sidebar to browse the pages.",
    result_one: "result",
    result_other: "results",
    theme_aria: "Theme: {name}. Change theme",
    theme_title: "Theme: {name}",
    theme_names: { auto: "Auto", light: "Light", dark: "Dark" },
  };

  // The page's strings come from the `data-ui` attribute of <body>; a missing or broken one leaves the English defaults.
  const readUi = () => {
    try {
      const given = JSON.parse(document.body.dataset.ui || "{}");
      return { ...DEFAULT_UI, ...given, theme_names: { ...DEFAULT_UI.theme_names, ...given.theme_names } };
    } catch {
      return DEFAULT_UI;
    }
  };

  const ui = readUi();

  const currentTheme = () => {
    const set = root.dataset.theme;
    return set === "light" || set === "dark" ? set : "auto";
  };

  const showTheme = (button) => {
    const name = ui.theme_names[currentTheme()];
    button.setAttribute("aria-label", ui.theme_aria.replace("{name}", name));
    button.setAttribute("title", ui.theme_title.replace("{name}", name));
  };

  const storeTheme = (next, button) => {
    try {
      if (next === "auto") {
        globalThis.localStorage.removeItem(THEME_KEY);
      } else {
        globalThis.localStorage.setItem(THEME_KEY, next);
      }
    } catch {
      button.dataset.unsaved = "true";
    }
  };

  const cycleTheme = (button, buttons) => {
    const next = THEMES[(THEMES.indexOf(currentTheme()) + 1) % THEMES.length];
    if (next === "auto") {
      delete root.dataset.theme;
    } else {
      root.dataset.theme = next;
    }
    storeTheme(next, button);
    buttons.forEach(showTheme);
  };

  const setupTheme = () => {
    const buttons = document.querySelectorAll(".theme-button");
    for (const button of buttons) {
      showTheme(button);
      button.addEventListener("click", () => cycleTheme(button, buttons));
    }
  };

  const setupMenu = () => {
    const toggle = document.querySelector(".menu-toggle");
    if (!toggle) {
      return;
    }
    const isOpen = () => toggle.getAttribute("aria-expanded") === "true";
    const setOpen = (open) => {
      root.classList.toggle("menu-open", open);
      toggle.setAttribute("aria-expanded", open ? "true" : "false");
    };
    toggle.addEventListener("click", () => setOpen(!isOpen()));
    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape" && isOpen()) {
        setOpen(false);
        toggle.focus();
      }
    });
  };

  const flash = (button, label, state) => {
    button.textContent = label;
    button.dataset.state = state;
    globalThis.setTimeout(() => {
      button.textContent = ui.copy;
      delete button.dataset.state;
    }, 1800);
  };

  const copyBlock = (block) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "copy-button";
    button.textContent = ui.copy;
    button.setAttribute("aria-label", ui.copy_aria);
    button.addEventListener("click", () => {
      if (!navigator.clipboard) {
        flash(button, ui.copy_failed, "failed");
        return;
      }
      const text = (block.querySelector("code") || block).textContent;
      navigator.clipboard.writeText(text).then(
        () => flash(button, ui.copied, "copied"),
        () => flash(button, ui.copy_failed, "failed"));
    });
    block.appendChild(button);
  };

  const setupCopy = () => {
    for (const block of document.querySelectorAll(".prose pre")) {
      copyBlock(block);
    }
  };

  const search = {
    index: null,
    failed: false,
    overlay: null,
    input: null,
    list: null,
    status: null,
    opener: null,
    hits: [],
    selected: 0,
  };

  const loadIndex = async () => {
    if (search.index || search.failed) {
      return;
    }
    try {
      const response = await fetch("search-index.json");
      if (!response.ok) {
        throw new Error("index");
      }
      search.index = await response.json();
    } catch {
      search.failed = true;
    }
  };

  const terms = (query) => query.toLowerCase().split(/\s+/).filter((word) => word.length > 0);

  const has = (text, words) => {
    const lower = text.toLowerCase();
    return words.every((word) => lower.includes(word));
  };

  const titleScore = (title, words) => {
    const lower = title.toLowerCase();
    const joined = words.join(" ");
    if (lower === joined) {
      return 100;
    }
    if (lower.startsWith(joined)) {
      return 80;
    }
    return has(title, words) ? 60 : 0;
  };

  const score = (entry, words) => {
    let best = titleScore(entry.title, words);
    const heading = entry.headings.find((item) => has(item.text, words)) || null;
    if (heading) {
      best = Math.max(best, 30);
    }
    if (has(entry.description, words)) {
      best = Math.max(best, 15);
    }
    if (best === 0 && has(entry.text, words)) {
      best = 5;
    }
    return { score: best, heading };
  };

  const find = (query) => {
    const words = terms(query);
    if (words.length === 0 || !search.index) {
      return [];
    }
    const found = [];
    search.index.forEach((entry, position) => {
      const result = score(entry, words);
      if (result.score > 0) {
        found.push({ entry, heading: result.heading, score: result.score, position });
      }
    });
    found.sort((a, b) => b.score - a.score || a.position - b.position);
    return found.slice(0, MAX_HITS);
  };

  const mark = () => {
    search.hits.forEach((link, position) => {
      link.parentNode.setAttribute("aria-selected", position === search.selected ? "true" : "false");
    });
    const current = search.hits[search.selected];
    if (current) {
      search.input.setAttribute("aria-activedescendant", current.parentNode.id);
      current.scrollIntoView({ block: "nearest" });
    } else {
      search.input.removeAttribute("aria-activedescendant");
    }
  };

  const note = (message) => {
    search.list.textContent = "";
    const item = document.createElement("li");
    item.className = "search-note";
    item.setAttribute("role", "presentation");
    item.textContent = message;
    search.list.appendChild(item);
    search.status.textContent = message;
    search.hits = [];
    mark();
  };

  const hitItem = (result, position) => {
    const item = document.createElement("li");
    const link = document.createElement("a");
    const title = document.createElement("strong");
    const place = document.createElement("span");
    const { entry, heading } = result;
    item.setAttribute("role", "option");
    item.id = `search-hit-${position}`;
    link.className = "search-hit";
    link.tabIndex = -1;
    const fragment = heading ? `#${heading.anchor}` : "";
    const section = entry.group ? `${entry.group} / ` : "";
    link.href = `${entry.slug}.html${fragment}`;
    title.textContent = entry.title;
    place.textContent = section + (heading ? heading.text : entry.description);
    link.append(title, place);
    item.appendChild(link);
    return item;
  };

  const emptyMessage = () => (terms(search.input.value).length === 0 ? ui.search_empty : ui.search_none);

  const render = () => {
    if (search.failed) {
      note(ui.search_unavailable);
      return;
    }
    if (!search.index) {
      note(ui.search_loading);
      return;
    }
    const found = find(search.input.value);
    if (found.length === 0) {
      note(emptyMessage());
      return;
    }
    search.list.textContent = "";
    const items = found.map((result, position) => hitItem(result, position));
    search.list.append(...items);
    search.hits = items.map((item) => item.firstChild);
    search.selected = 0;
    search.status.textContent = `${found.length} ${found.length === 1 ? ui.result_one : ui.result_other}`;
    mark();
  };

  const closeSearch = (restoreFocus) => {
    search.overlay.hidden = true;
    if (restoreFocus && search.opener) {
      search.opener.focus();
    }
  };

  const openSearch = (from) => {
    if (!search.overlay.hidden) {
      return;
    }
    search.opener = from || document.activeElement;
    search.overlay.hidden = false;
    search.input.value = "";
    note(ui.search_loading);
    search.input.focus();
    loadIndex().then(render);
  };

  const moveSelection = (step) => {
    search.selected = (search.selected + step + search.hits.length) % search.hits.length;
    mark();
  };

  const onInputKey = (event) => {
    if (event.isComposing || event.keyCode === 229) {
      return;
    }
    const { key } = event;
    if ((key === "ArrowDown" || key === "ArrowUp") && search.hits.length > 0) {
      moveSelection(key === "ArrowDown" ? 1 : -1);
      event.preventDefault();
    } else if (key === "Enter" && search.hits[search.selected]) {
      closeSearch(false);
      globalThis.location.href = search.hits[search.selected].href;
    } else if (key === "Tab") {
      event.preventDefault();
    }
  };

  const isOpenShortcut = (event) => {
    const typing = /^(input|textarea|select)$/i.test(event.target.tagName) || event.target.isContentEditable;
    const withModifier = (event.key || "").toLowerCase() === "k" && (event.ctrlKey || event.metaKey);
    return withModifier || (event.key === "/" && !typing);
  };

  const buildSearch = () => {
    const overlay = document.createElement("div");
    const box = document.createElement("div");
    const input = document.createElement("input");
    const list = document.createElement("ul");
    const status = document.createElement("p");
    overlay.className = "search-overlay";
    overlay.hidden = true;
    box.className = "search-box";
    box.setAttribute("role", "dialog");
    box.setAttribute("aria-modal", "true");
    box.setAttribute("aria-label", ui.search_aria);
    input.type = "search";
    input.className = "search-input";
    input.setAttribute("aria-label", ui.search_label);
    input.setAttribute("placeholder", ui.search_placeholder);
    input.setAttribute("role", "combobox");
    input.setAttribute("aria-expanded", "true");
    input.setAttribute("aria-controls", "search-results");
    list.className = "search-results";
    list.id = "search-results";
    list.setAttribute("role", "listbox");
    status.className = "search-status";
    status.setAttribute("role", "status");
    box.append(input, status, list);
    overlay.appendChild(box);
    document.body.appendChild(overlay);
    Object.assign(search, { overlay, input, list, status });
  };

  const setupSearch = () => {
    buildSearch();
    search.list.addEventListener("click", (event) => {
      if (event.target.closest(".search-hit")) {
        closeSearch(false);
      }
    });
    search.input.addEventListener("input", render);
    search.input.addEventListener("keydown", onInputKey);
    search.overlay.addEventListener("click", (event) => {
      if (event.target === search.overlay) {
        closeSearch(true);
      }
    });
    for (const button of document.querySelectorAll(".search-button")) {
      button.hidden = false;
      button.addEventListener("click", () => openSearch(button));
    }
    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape" && !search.overlay.hidden) {
        closeSearch(true);
      } else if (isOpenShortcut(event)) {
        event.preventDefault();
        openSearch(null);
      }
    });
  };

  // Without the script the language links are plain links; with it they keep the section the visitor is reading,
  // and the open dropdown closes on Escape and on a click outside it.
  const setupLanguageMenu = () => {
    for (const link of document.querySelectorAll(".lang-menu a")) {
      link.addEventListener("click", () => {
        link.hash = globalThis.location.hash;
      });
    }
    const closeDropdowns = (except) => {
      for (const dropdown of document.querySelectorAll(".lang-dropdown[open]")) {
        if (dropdown !== except) {
          dropdown.open = false;
        }
      }
    };
    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape") {
        const focused = [...document.querySelectorAll(".lang-dropdown")].find((dropdown) => dropdown.contains(document.activeElement));
        closeDropdowns(null);
        focused?.querySelector("summary")?.focus();
      }
    });
    document.addEventListener("click", (event) => {
      const inside = [...document.querySelectorAll(".lang-dropdown")].find((dropdown) => dropdown.contains(event.target));
      closeDropdowns(inside ?? null);
    });
  };

  setupTheme();
  setupLanguageMenu();
  setupMenu();
  setupCopy();
  setupSearch();
})();
