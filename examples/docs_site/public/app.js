const state = {
  siteCache: new Map(),
  searchCache: new Map(),
  site: null,
  route: null,
  searchToken: 0,
  hasNavigated: false,
};

function parseHash() {
  const raw = window.location.hash.replace(/^#\/?/, "");
  const parts = raw.split("/").filter(Boolean);
  const lang = parts[0] === "en" ? "en" : "zh";
  const kind = parts[1] === "example" ? "example" : "page";
  const slug = parts[2] || (kind === "example" ? "examples--hello" : "README");
  return { lang, kind, slug };
}

function setRoute(route) {
  const next = `#/${route.lang}/${route.kind}/${route.slug}`;
  if (window.location.hash !== next) {
    window.location.hash = next;
    return;
  }
  void renderRoute(route);
}

async function fetchJson(path) {
  const response = await fetch(path);
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json();
}

async function loadSite(lang) {
  if (!state.siteCache.has(lang)) {
    state.siteCache.set(lang, fetchJson(`/api/site/${lang}`));
  }
  return state.siteCache.get(lang);
}

async function loadSearchIndex(lang) {
  if (!state.searchCache.has(lang)) {
    state.searchCache.set(
      lang,
      fetchJson(`/api/search/${lang}`).then((payload) => payload.items || []),
    );
  }
  return state.searchCache.get(lang);
}

function byId(id) {
  return document.getElementById(id);
}

function scrollPrimaryPanel(panelId) {
  const panel = byId(panelId);
  if (!panel) {
    state.hasNavigated = true;
    return;
  }

  const behavior = state.hasNavigated ? "smooth" : "auto";
  requestAnimationFrame(() => {
    panel.scrollIntoView({ block: "start", behavior });
    state.hasNavigated = true;
  });
}

function text(value) {
  return value == null ? "" : String(value);
}

function clear(node) {
  while (node.firstChild) {
    node.removeChild(node.firstChild);
  }
}

function makeButton(className, title, summary, docPath, onClick) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = className;
  const strong = document.createElement("strong");
  strong.textContent = title;
  const span = document.createElement("span");
  span.innerHTML = renderInline(summary, docPath || "");
  button.append(strong, span);
  button.addEventListener("click", onClick);
  return button;
}

function makeStaticChip(value) {
  const chip = document.createElement("div");
  chip.className = "chip-link";
  chip.textContent = value;
  return chip;
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function escapeAttribute(value) {
  return escapeHtml(value).replaceAll("`", "&#96;");
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function highlightText(value, query) {
  const source = text(value);
  if (!query) {
    return escapeHtml(source);
  }
  const pattern = new RegExp(`(${escapeRegExp(query)})`, "gi");
  const parts = source.split(pattern);
  return parts
    .map((part) => {
      if (part.toLowerCase() === query.toLowerCase()) {
        return `<mark class="hit">${escapeHtml(part)}</mark>`;
      }
      return escapeHtml(part);
    })
    .join("");
}

function makeSearchButton(item, route, query, snippet, onClick) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "result-item";

  const strong = document.createElement("strong");
  strong.innerHTML = highlightText(item.title, query);

  const span = document.createElement("span");
  span.innerHTML = highlightText(snippet, query);

  button.append(strong, span);
  button.addEventListener("click", onClick);
  return button;
}

function normalizeRoute(site, route) {
  const list = route.kind === "example" ? site.examples : site.pages;
  const key = route.kind === "example" ? "name" : "slug";
  if (list.some((item) => item[key] === route.slug)) {
    return route;
  }
  const fallback = list[0];
  if (!fallback) {
    return route;
  }
  return {
    lang: route.lang,
    kind: route.kind,
    slug: fallback[key],
  };
}

function renderShell(site, route) {
  state.site = site;
  document.documentElement.lang = route.lang;
  document.title = `${site.brand} · ${site.tagline}`;
  byId("brand").textContent = site.brand;
  byId("tagline").textContent = site.tagline;
  byId("heroTitle").textContent = site.hero_title;
  byId("heroDeck").textContent = site.hero_deck;
  byId("navTitle").textContent = site.nav_title;
  byId("examplesTitle").textContent = site.examples_title;
  byId("searchTitle").textContent = site.search_title;
  byId("searchInput").placeholder = site.search_placeholder;
  byId("footerNote").textContent = site.footer_note;
  byId("sourceMeta").textContent = site.source_meta;

  for (const button of document.querySelectorAll(".lang-chip")) {
    button.classList.toggle("active", button.dataset.lang === route.lang);
  }

  const navList = byId("navList");
  clear(navList);
  for (const item of site.pages) {
    const button = makeButton("nav-item", item.title, item.summary, item.path, () => {
      setRoute({ lang: route.lang, kind: "page", slug: item.slug });
    });
    button.classList.toggle("active", route.kind === "page" && route.slug === item.slug);
    navList.appendChild(button);
  }

  const exampleList = byId("exampleList");
  clear(exampleList);
  for (const item of site.examples) {
    const button = makeButton("nav-item", item.title, item.summary, item.path, () => {
      setRoute({ lang: route.lang, kind: "example", slug: item.name });
    });
    button.classList.toggle("active", route.kind === "example" && route.slug === item.name);
    exampleList.appendChild(button);
  }
}

function normalizeRepoPath(value) {
  const parts = [];
  for (const raw of text(value).split("/")) {
    const part = raw.trim();
    if (!part || part === ".") {
      continue;
    }
    if (part === "..") {
      parts.pop();
      continue;
    }
    parts.push(part);
  }
  return parts.join("/");
}

function encodePathSegments(value) {
  return text(value)
    .split("/")
    .filter(Boolean)
    .map((part) => encodeURIComponent(part))
    .join("/");
}

function dirname(value) {
  const parts = text(value).split("/");
  parts.pop();
  return parts.join("/");
}

function resolveRepoPath(fromPath, href) {
  const raw = text(href).split("#")[0].split("?")[0].trim();
  if (!raw || raw.startsWith("http://") || raw.startsWith("https://") || raw.startsWith("mailto:")) {
    return null;
  }
  if (raw.startsWith("/")) {
    return normalizeRepoPath(raw);
  }
  const base = dirname(fromPath);
  return normalizeRepoPath(base ? `${base}/${raw}` : raw);
}

function buildRepoAssetURL(path) {
  const normalized = normalizeRepoPath(path);
  if (normalized.startsWith("docs/")) {
    return `/repo/docs/${encodePathSegments(normalized.slice("docs/".length))}`;
  }
  if (normalized.startsWith("examples/")) {
    return `/repo/examples/${encodePathSegments(normalized.slice("examples/".length))}`;
  }
  return null;
}

function findPageByPath(path) {
  return (state.site?.pages || []).find((item) => item.path === path) || null;
}

function findExampleByPath(path) {
  return (state.site?.examples || []).find((item) => item.path === path) || null;
}

function renderRepoLink(label, href, docPath) {
  const cleanLabel = escapeHtml(label);
  const resolved = resolveRepoPath(docPath, href);
  if (href.startsWith("http://") || href.startsWith("https://") || href.startsWith("mailto:")) {
    return `<a href="${escapeAttribute(href)}" target="_blank" rel="noreferrer">${cleanLabel}</a>`;
  }
  if (resolved) {
    const page = findPageByPath(resolved);
    if (page) {
      return `<a href="#/${state.route?.lang || "zh"}/page/${escapeAttribute(page.slug)}">${cleanLabel}</a>`;
    }
    const example = findExampleByPath(resolved);
    if (example) {
      return `<a href="#/${state.route?.lang || "zh"}/example/${escapeAttribute(example.name)}">${cleanLabel}</a>`;
    }
    const assetURL = buildRepoAssetURL(resolved);
    if (assetURL) {
      return `<a href="${escapeAttribute(assetURL)}" target="_blank" rel="noreferrer">${cleanLabel}</a>`;
    }
    return `<span class="dead-link" title="${escapeAttribute(resolved)}">${cleanLabel}</span>`;
  }
  return `<span class="dead-link">${cleanLabel}</span>`;
}

function renderRepoImage(alt, href, docPath) {
  const cleanAlt = escapeAttribute(alt || "");
  if (href.startsWith("http://") || href.startsWith("https://")) {
    return `<img class="doc-image" src="${escapeAttribute(href)}" alt="${cleanAlt}" loading="lazy" />`;
  }
  const resolved = resolveRepoPath(docPath, href);
  if (resolved) {
    const assetURL = buildRepoAssetURL(resolved);
    if (assetURL) {
      return `<img class="doc-image" src="${escapeAttribute(assetURL)}" alt="${cleanAlt}" loading="lazy" />`;
    }
  }
  return `<span class="dead-link">${escapeHtml(alt || href)}</span>`;
}

function renderAutoLink(href) {
  const cleanHref = escapeAttribute(href);
  const label = escapeHtml(href);
  return `<a href="${cleanHref}" target="_blank" rel="noreferrer">${label}</a>`;
}

function applyInlineStyles(html) {
  let value = html;
  value = value.replace(/\*\*([^\s*](?:[^*]*?[^\s*])?)\*\*/g, "<strong>$1</strong>");
  value = value.replace(/(^|[^A-Za-z0-9])__([^\s_](?:[^_]*?[^\s_])?)__(?=[^A-Za-z0-9]|$)/g, (_, lead, inner) => `${lead}<strong>${inner}</strong>`);
  value = value.replace(/\*([^\s*](?:[^*]*?[^\s*])?)\*/g, "<em>$1</em>");
  value = value.replace(/(^|[^A-Za-z0-9])_([^\s_](?:[^_]*?[^\s_])?)_(?=[^A-Za-z0-9]|$)/g, (_, lead, inner) => `${lead}<em>${inner}</em>`);
  return value;
}

function renderInline(value, docPath) {
  const tokens = [];
  const stash = (html) => {
    const key = `\u0000${tokens.length}\u0000`;
    tokens.push(html);
    return key;
  };

  let source = text(value);
  source = source.replace(/`([^`]+)`/g, (_, code) => stash(`<code>${escapeHtml(code)}</code>`));
  source = source.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, (_, alt, href) => stash(renderRepoImage(alt, href, docPath)));
  source = source.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_, label, href) => stash(renderRepoLink(label, href, docPath)));
  source = source.replace(/<((?:https?:\/\/|mailto:)[^>]+)>/g, (_, href) => stash(renderAutoLink(href)));

  let html = escapeHtml(source);
  html = applyInlineStyles(html);

  for (let i = 0; i < tokens.length; i += 1) {
    html = html.replaceAll(`\u0000${i}\u0000`, tokens[i]);
  }
  return html;
}

function isHorizontalRule(line) {
  return /^(-{3,}|_{3,}|\*{3,})$/.test(line.trim());
}

function parseTaskItem(line) {
  const match = line.trim().match(/^[-*]\s+\[([ xX])\]\s+(.*)$/);
  if (!match) {
    return null;
  }
  return {
    checked: match[1].toLowerCase() === "x",
    value: match[2],
  };
}

function isOrderedItem(line) {
  return /^\d+\.\s+/.test(line.trim());
}

function isHeading(line) {
  return /^#{1,6}\s+/.test(line.trim());
}

function getFenceInfo(line) {
  const match = line.trim().match(/^(```+|~~~+)(.*)$/);
  if (!match) {
    return null;
  }
  return {
    marker: match[1],
    language: match[2].trim(),
  };
}

function isTableLine(line) {
  return line.trim().startsWith("|");
}

function isTableDivider(line) {
  return /^\|?[\s:-|]+\|?$/.test(line.trim());
}

function parseTableCells(line) {
  const trimmed = line.trim().replace(/^\|/, "").replace(/\|$/, "");
  return trimmed.split("|").map((cell) => cell.trim());
}

function countIndent(line) {
  const match = text(line).match(/^\s*/);
  return match ? match[0].length : 0;
}

function getListInfo(line) {
  const raw = text(line);
  const trimmed = raw.trimStart();
  const indent = raw.length - trimmed.length;
  const task = parseTaskItem(trimmed);
  if (task) {
    return {
      ordered: false,
      indent,
      value: task.value,
      task,
    };
  }

  let match = trimmed.match(/^[-*]\s+(.*)$/);
  if (match) {
    return {
      ordered: false,
      indent,
      value: match[1],
      task: null,
    };
  }

  match = trimmed.match(/^\d+\.\s+(.*)$/);
  if (match) {
    return {
      ordered: true,
      indent,
      value: match[1],
      task: null,
    };
  }

  return null;
}

function appendParagraph(container, lines, docPath) {
  const paragraph = document.createElement("p");
  paragraph.innerHTML = renderInline(lines.join(" "), docPath);
  container.appendChild(paragraph);
}

function appendCodeBlock(container, lines, language) {
  const pre = document.createElement("pre");
  pre.className = "code-block";
  const code = document.createElement("code");
  if (language) {
    code.dataset.lang = language;
  }
  code.textContent = lines.join("\n");
  pre.appendChild(code);
  container.appendChild(pre);
}

function appendListContinuation(item, lines, docPath) {
  const paragraph = document.createElement("p");
  paragraph.className = "list-continuation";
  paragraph.innerHTML = renderInline(lines.join(" "), docPath);
  item.appendChild(paragraph);
}

function buildListItem(info, docPath) {
  const item = document.createElement("li");
  const body = document.createElement("div");
  body.className = "list-item-body";
  body.innerHTML = renderInline(info.value, docPath);

  if (info.task) {
    item.className = "task-item";
    const row = document.createElement("div");
    row.className = "task-main";

    const marker = document.createElement("span");
    marker.className = `task-mark ${info.task.checked ? "done" : "todo"}`;
    marker.textContent = info.task.checked ? "✓" : "";

    body.classList.add("task-body");
    row.append(marker, body);
    item.appendChild(row);
    return { item, isTask: true };
  }

  item.appendChild(body);
  return { item, isTask: false };
}

function parseList(lines, startIndex, docPath, baseIndent = null) {
  const first = getListInfo(lines[startIndex]);
  if (!first) {
    return null;
  }

  const indent = baseIndent == null ? first.indent : baseIndent;
  const ordered = first.ordered;
  const list = document.createElement(ordered ? "ol" : "ul");
  list.className = "bullet-list";
  let hasTaskItems = false;
  let index = startIndex;

  while (index < lines.length) {
    const current = getListInfo(lines[index]);
    if (!current || current.indent < indent || current.indent > indent || current.ordered !== ordered) {
      break;
    }

    const built = buildListItem(current, docPath);
    hasTaskItems = hasTaskItems || built.isTask;
    index += 1;

    const continuation = [];
    while (index < lines.length) {
      const raw = lines[index];
      const trimmed = raw.trim();
      if (!trimmed) {
        index += 1;
        break;
      }

      const next = getListInfo(raw);
      if (next) {
        if (next.indent < indent) {
          break;
        }
        if (next.indent === indent && next.ordered === ordered) {
          break;
        }
        if (next.indent > indent) {
          if (continuation.length) {
            appendListContinuation(built.item, continuation, docPath);
            continuation.length = 0;
          }
          const nested = parseList(lines, index, docPath, next.indent);
          if (nested) {
            built.item.appendChild(nested.node);
            index = nested.nextIndex;
            continue;
          }
        }
      }

      if (countIndent(raw) > indent) {
        continuation.push(trimmed);
        index += 1;
        continue;
      }
      break;
    }

    if (continuation.length) {
      appendListContinuation(built.item, continuation, docPath);
    }
    list.appendChild(built.item);
  }

  if (hasTaskItems) {
    list.classList.add("task-list");
  }

  return { node: list, nextIndex: index };
}

function appendTable(container, lines, docPath) {
  if (lines.length < 2 || !isTableDivider(lines[1])) {
    appendParagraph(container, [lines.join(" ")], docPath);
    return;
  }

  const table = document.createElement("table");
  table.className = "doc-table";

  const thead = document.createElement("thead");
  const headRow = document.createElement("tr");
  for (const cell of parseTableCells(lines[0])) {
    const th = document.createElement("th");
    th.innerHTML = renderInline(cell, docPath);
    headRow.appendChild(th);
  }
  thead.appendChild(headRow);
  table.appendChild(thead);

  const tbody = document.createElement("tbody");
  for (const line of lines.slice(2)) {
    const row = document.createElement("tr");
    for (const cell of parseTableCells(line)) {
      const td = document.createElement("td");
      td.innerHTML = renderInline(cell, docPath);
      row.appendChild(td);
    }
    tbody.appendChild(row);
  }
  table.appendChild(tbody);
  container.appendChild(table);
}

function appendBlockquote(container, lines, docPath) {
  const blockquote = document.createElement("blockquote");
  const inner = lines.map((line) => line.replace(/^\s*>\s?/, "")).join("\n");
  renderMarkdown(blockquote, inner, docPath);
  container.appendChild(blockquote);
}

function renderMarkdown(container, markdown, docPath) {
  const body = document.createElement("div");
  body.className = "markdown-body";
  const lines = text(markdown).replace(/\r\n/g, "\n").replace(/\r/g, "\n").split("\n");
  let index = 0;

  while (index < lines.length) {
    const line = lines[index];
    const trimmed = line.trim();

    if (!trimmed) {
      index += 1;
      continue;
    }

    const fence = getFenceInfo(line);
    if (fence) {
      const block = [];
      index += 1;
      while (index < lines.length) {
        const closingFence = getFenceInfo(lines[index]);
        if (closingFence && closingFence.marker[0] === fence.marker[0]) {
          break;
        }
        block.push(lines[index]);
        index += 1;
      }
      if (index < lines.length) {
        index += 1;
      }
      appendCodeBlock(body, block, fence.language);
      continue;
    }

    if (isHeading(line)) {
      const level = Math.min(6, line.trim().match(/^#+/)[0].length);
      const heading = document.createElement(`h${level}`);
      heading.innerHTML = renderInline(line.trim().replace(/^#{1,6}\s+/, ""), docPath);
      body.appendChild(heading);
      index += 1;
      continue;
    }

    if (isHorizontalRule(line)) {
      body.appendChild(document.createElement("hr"));
      index += 1;
      continue;
    }

    if (/^>\s?/.test(trimmed)) {
      const quoteLines = [];
      while (index < lines.length && /^>\s?/.test(lines[index].trim())) {
        quoteLines.push(lines[index]);
        index += 1;
      }
      appendBlockquote(body, quoteLines, docPath);
      continue;
    }

    if (isTableLine(line)) {
      const tableLines = [];
      while (index < lines.length && isTableLine(lines[index])) {
        tableLines.push(lines[index]);
        index += 1;
      }
      appendTable(body, tableLines, docPath);
      continue;
    }

    const listInfo = getListInfo(line);
    if (listInfo) {
      const parsed = parseList(lines, index, docPath, listInfo.indent);
      if (parsed) {
        body.appendChild(parsed.node);
        index = parsed.nextIndex;
        continue;
      }
      continue;
    }

    const paragraphLines = [];
    while (index < lines.length) {
      const current = lines[index].trim();
      if (
        !current ||
        getFenceInfo(lines[index]) ||
        isHeading(lines[index]) ||
        isHorizontalRule(lines[index]) ||
        /^>\s?/.test(current) ||
        isTableLine(lines[index]) ||
        getListInfo(lines[index])
      ) {
        break;
      }
      paragraphLines.push(current);
      index += 1;
    }
    appendParagraph(body, paragraphLines, docPath);
  }

  container.appendChild(body);
}

function renderPage(payload, route) {
  byId("articleMeta").textContent = state.site?.page_meta || "Repo Doc";
  byId("articleTitle").textContent = payload.title;
  byId("articleSummary").innerHTML = renderInline(payload.summary, payload.path);

  const body = byId("articleBody");
  clear(body);

  const note = document.createElement("section");
  note.className = "content-block content-note";
  const row = document.createElement("div");
  row.className = "meta-row";
  row.appendChild(makeStaticChip(`${state.site?.path_meta || "Repo Path"}: ${payload.path}`));
  note.appendChild(row);
  const summary = document.createElement("p");
  summary.textContent = state.site?.rendered_note || "";
  note.appendChild(summary);
  body.appendChild(note);

  renderMarkdown(body, payload.content, payload.path);
  byId("sourcePanel").classList.add("hidden");
  scrollPrimaryPanel("articlePanel");
}

function renderExample(payload, route) {
  byId("articleMeta").textContent = state.site?.example_meta || "Example Source";
  byId("articleTitle").textContent = payload.title;
  byId("articleSummary").innerHTML = renderInline(payload.summary, payload.path);

  const body = byId("articleBody");
  clear(body);

  const block = document.createElement("section");
  block.className = "content-block content-note";
  const row = document.createElement("div");
  row.className = "meta-row";
  row.appendChild(makeStaticChip(`${state.site?.path_meta || "Repo Path"}: ${payload.path}`));
  block.appendChild(row);

  const summary = document.createElement("p");
  summary.textContent =
    route.lang === "zh"
      ? "这里展示的是仓库中的真实 Gwen 源文件，而不是复制出来的教学版本。"
      : "This view shows the real Gwen source file from the repository, not a copied teaching variant.";
  block.appendChild(summary);
  body.appendChild(block);

  byId("sourcePanel").classList.remove("hidden");
  byId("sourceTitle").textContent = payload.title;
  byId("sourceSummary").innerHTML = renderInline(payload.summary, payload.path);
  byId("sourcePath").textContent = payload.path;
  byId("sourceCode").textContent = payload.source;
  scrollPrimaryPanel("sourcePanel");
}

async function renderRoute(route = parseHash()) {
  const site = await loadSite(route.lang);
  const nextRoute = normalizeRoute(site, route);
  if (nextRoute.slug !== route.slug || nextRoute.kind !== route.kind) {
    setRoute(nextRoute);
    return;
  }

  state.route = nextRoute;
  renderShell(site, nextRoute);

  if (nextRoute.kind === "example") {
    const payload = await fetchJson(`/api/example/${nextRoute.lang}/${nextRoute.slug}`);
    renderExample(payload, nextRoute);
    return;
  }

  const payload = await fetchJson(`/api/page/${nextRoute.lang}/${nextRoute.slug}`);
  renderPage(payload, nextRoute);
}

function buildSearchSnippet(item, query) {
  const source = `${item.summary || ""}\n${item.content || ""}`.replace(/\s+/g, " ").trim();
  if (!source) {
    return item.path || "";
  }
  const lower = source.toLowerCase();
  const index = lower.indexOf(query);
  if (index === -1) {
    return source.length > 140 ? `${source.slice(0, 140)}…` : source;
  }
  const start = Math.max(0, index - 42);
  const end = Math.min(source.length, index + query.length + 84);
  let snippet = source.slice(start, end);
  if (start > 0) {
    snippet = `…${snippet}`;
  }
  if (end < source.length) {
    snippet = `${snippet}…`;
  }
  return snippet;
}

async function runSearch() {
  const route = state.route || parseHash();
  const input = byId("searchInput");
  const resultsNode = byId("searchResults");
  const query = input.value.trim().toLowerCase();
  const token = ++state.searchToken;

  clear(resultsNode);
  if (!query) {
    return;
  }

  const [site, index] = await Promise.all([loadSite(route.lang), loadSearchIndex(route.lang)]);
  if (token !== state.searchToken || input.value.trim().toLowerCase() !== query) {
    return;
  }

  const seen = new Set();
  const groups = {
    page: [],
    example: [],
  };

  for (const item of index) {
    const searchable = `${item.title || ""}\n${item.summary || ""}\n${item.path || ""}\n${item.content || ""}`.toLowerCase();
    if (!searchable.includes(query)) {
      continue;
    }
    const key = item.kind === "example" ? `example:${item.name}` : `page:${item.slug}`;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    groups[item.kind].push(item);
  }

  if (!groups.page.length && !groups.example.length) {
    const empty = document.createElement("div");
    empty.className = "result-item";
    empty.innerHTML = `<strong>${escapeHtml(site.empty_search)}</strong>`;
    resultsNode.appendChild(empty);
    return;
  }

  for (const kind of ["page", "example"]) {
    if (!groups[kind].length) {
      continue;
    }

    const section = document.createElement("section");
    section.className = "result-group";

    const heading = document.createElement("p");
    heading.className = "result-group-title";
    heading.textContent = kind === "page" ? site.page_meta : site.example_meta;
    section.appendChild(heading);

    for (const item of groups[kind]) {
      const snippet = buildSearchSnippet(item, query);
      const button = makeSearchButton(item, route, query, snippet, () => {
        if (item.kind === "example") {
          setRoute({ lang: route.lang, kind: "example", slug: item.name });
        } else {
          setRoute({ lang: route.lang, kind: "page", slug: item.slug });
        }
        byId("searchInput").value = "";
        state.searchToken += 1;
        clear(resultsNode);
      });
      section.appendChild(button);
    }

    resultsNode.appendChild(section);
  }
}

function bindEvents() {
  window.addEventListener("hashchange", () => {
    void renderRoute(parseHash());
  });

  byId("searchInput").addEventListener("input", () => {
    void runSearch();
  });

  for (const button of document.querySelectorAll(".lang-chip")) {
    button.addEventListener("click", () => {
      const route = state.route || parseHash();
      setRoute({ lang: button.dataset.lang, kind: route.kind, slug: route.slug });
    });
  }
}

bindEvents();
void renderRoute(parseHash());
