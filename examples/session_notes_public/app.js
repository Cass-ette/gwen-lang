const state = {
  me: null,
  notes: [],
};

const sessionName = document.querySelector("#session-name");
const sessionChip = document.querySelector("#session-chip");
const loginCount = document.querySelector("#login-count");
const noteCount = document.querySelector("#note-count");
const composeState = document.querySelector("#compose-state");
const formHint = document.querySelector("#form-hint");
const flash = document.querySelector("#flash");
const sessionBanner = document.querySelector("#session-banner");
const loginForm = document.querySelector("#login-form");
const loginName = document.querySelector("#login-name");
const logoutButton = document.querySelector("#logout-button");
const noteForm = document.querySelector("#note-form");
const noteInput = document.querySelector("#note-input");
const noteSubmit = document.querySelector("#note-submit");
const notesList = document.querySelector("#notes-list");
const apiSnippet = document.querySelector("#api-snippet");
const quickLoginButtons = Array.from(document.querySelectorAll(".quick-login"));
const pendingActionKey = "gwen-session-notes-pending";

function text(value) {
  return value == null ? "" : String(value);
}

function escapeHtml(value) {
  return text(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

async function requestJSON(url, options = {}) {
  const response = await fetch(url, options);
  if (!response.ok) {
    const body = await response.text();
    throw new Error(body || `${response.status} ${response.statusText}`);
  }
  return response.json();
}

function setFlash(message, kind = "") {
  flash.textContent = message;
  flash.className = `flash${kind ? ` ${kind}` : ""}`;
}

function rememberPendingAction(payload) {
  try {
    sessionStorage.setItem(pendingActionKey, JSON.stringify(payload));
  } catch {
    // Ignore storage failures; UI still works without post-redirect flash.
  }
}

function consumePendingAction() {
  try {
    const raw = sessionStorage.getItem(pendingActionKey);
    if (!raw) {
      return null;
    }
    sessionStorage.removeItem(pendingActionKey);
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

function currentSession() {
  return text(state.me?.session || "guest");
}

function renderNotes() {
  if (!Array.isArray(state.notes) || state.notes.length === 0) {
    notesList.innerHTML = `<div class="note-empty">${
      currentSession() === "guest"
        ? "Login to start a session-bound note feed."
        : "No notes yet. Add the first one from the form above."
    }</div>`;
    return;
  }

  notesList.innerHTML = state.notes
    .map(
      (note, index) => `
        <article class="note-card">
          <small>Note ${index + 1}</small>
          <p>${escapeHtml(note)}</p>
        </article>
      `
    )
    .join("");
}

function renderSnippet() {
  const session = currentSession();
  const payload = `{"text":"ship the session demo"}`;
  apiSnippet.textContent =
    session === "guest"
      ? [
          "GET /api/me",
          "GET /api/notes",
          "",
          "Login first via /login/<name>.",
        ].join("\n")
      : [
          `GET /api/me`,
          `GET /api/notes`,
          `POST /api/notes`,
          `Cookie: session=${session}`,
          `Content-Type: application/json`,
          "",
          payload,
        ].join("\n");
}

function renderIdentity() {
  const session = currentSession();
  const guest = session === "guest";

  sessionName.textContent = session;
  sessionChip.textContent = guest ? "Guest" : "Active Session";
  sessionChip.className = `chip ${guest ? "guest" : "member"}`;
  loginCount.textContent = text(state.me?.login_count || 0);
  noteCount.textContent = text(state.me?.note_count || 0);

  composeState.textContent = guest ? "Login required" : "Ready to write";
  composeState.className = `status-pill${guest ? "" : " ready"}`;
  formHint.textContent = guest
    ? "Login first. The note list is keyed by the current session cookie."
    : `Notes are currently attached to the "${session}" cookie.`;
  sessionBanner.className = `session-banner ${guest ? "guest" : "member"}`;
  sessionBanner.innerHTML = guest
    ? "<strong>Guest Mode</strong><span>Login to bind notes to a real session cookie.</span>"
    : `<strong>Session Ready</strong><span>Notes and counts now persist for "${escapeHtml(session)}".</span>`;

  noteInput.disabled = guest;
  noteSubmit.disabled = guest;
  logoutButton.disabled = guest;
  noteInput.placeholder = guest
    ? "Login first to attach a note to a session."
    : `Write a note for ${session}.`;

  quickLoginButtons.forEach((button) => {
    const active = !guest && text(button.dataset.name) === session;
    button.classList.toggle("active", active);
  });
}

function render() {
  renderIdentity();
  renderNotes();
  renderSnippet();
}

async function refresh() {
  state.me = await requestJSON("/api/me");
  if (currentSession() === "guest") {
    state.notes = [];
  } else {
    const payload = await requestJSON("/api/notes");
    state.notes = Array.isArray(payload.notes) ? payload.notes : [];
  }
  render();

  const pending = consumePendingAction();
  if (pending?.action === "login" && pending.name === currentSession()) {
    setFlash(`Logged in as ${currentSession()}.`, "success");
  } else if (pending?.action === "logout" && currentSession() === "guest") {
    setFlash("Session cleared.", "success");
  }
}

function loginAs(name) {
  const value = text(name).trim();
  if (!value) {
    setFlash("Session name cannot be empty.", "error");
    return;
  }
  rememberPendingAction({ action: "login", name: value });
  window.location.assign(`/login/${encodeURIComponent(value)}`);
}

document.querySelectorAll(".quick-login").forEach((button) => {
  button.addEventListener("click", () => {
    loginAs(button.dataset.name);
  });
});

loginForm.addEventListener("submit", (event) => {
  event.preventDefault();
  loginAs(loginName.value);
});

logoutButton.addEventListener("click", () => {
  rememberPendingAction({ action: "logout" });
  window.location.assign("/logout");
});

noteForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const textValue = noteInput.value.trim();
  if (!textValue) {
    setFlash("Note text cannot be empty.", "error");
    return;
  }

  noteSubmit.disabled = true;
  setFlash("Saving note...", "");
  try {
    await requestJSON("/api/notes", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ text: textValue }),
    });
    noteInput.value = "";
    await refresh();
    setFlash("Note saved.", "success");
  } catch (error) {
    setFlash(text(error.message || error), "error");
  } finally {
    noteSubmit.disabled = currentSession() === "guest";
  }
});

refresh().catch((error) => {
  setFlash(text(error.message || error), "error");
});
