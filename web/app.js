// Playbook SPA — Codecademy-style split layout:
// left = lesson / exercise instructions, right = always-available playground.
// Branding, locales, and toolchains all come from content/course.json.

const $ = (sel, el = document) => el.querySelector(sel);
const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

const state = {
  course: null,
  progress: {},   // "<ch>/<sectionId>" -> "done"
  locale: "vi",
  toolchains: {}, // toolchain name -> installed?
};

// ---------- toolchains (playground languages) ----------

const TOOLCHAINS = {
  c:      { scratchFile: "main.c",   cmMode: "text/x-csrc",
            scratch: '#include <stdio.h>\n\nint main(void)\n{\n    printf("hello, playground!\\n");\n    return 0;\n}\n' },
  cpp:    { scratchFile: "main.cpp", cmMode: "text/x-c++src",
            scratch: '#include <iostream>\n\nint main()\n{\n    std::cout << "hello, playground!\\n";\n    return 0;\n}\n' },
  python: { scratchFile: "main.py",  cmMode: "python",
            scratch: 'print("hello, playground!")\n' },
  node:   { scratchFile: "main.js",  cmMode: "javascript",
            scratch: 'console.log("hello, playground!");\n' },
  kotlin: { scratchFile: "main.kt",  cmMode: "text/x-kotlin",
            scratch: 'fun main() {\n    println("hello, playground!")\n}\n' },
  java:   { scratchFile: "Main.java", cmMode: "text/x-java",
            scratch: 'public class Main {\n    public static void main(String[] args) {\n        System.out.println("hello, playground!");\n    }\n}\n' },
};

const EXT_META = {
  c: "text/x-csrc", h: "text/x-csrc", py: "python",
  cpp: "text/x-c++src", cc: "text/x-c++src", cxx: "text/x-c++src", hpp: "text/x-c++src",
  js: "javascript", mjs: "javascript", cjs: "javascript", json: "application/json",
  kt: "text/x-kotlin", kts: "text/x-kotlin", java: "text/x-java",
};
const cmModeFor = (file) => EXT_META[(file || "").split(".").pop()] || "text/plain";
const hlClassFor = (file) => {
  const e = (file || "").split(".").pop();
  return { c: "c", h: "c", py: "python", js: "javascript", mjs: "javascript",
           cpp: "cpp", cc: "cpp", cxx: "cpp", hpp: "cpp",
           kt: "kotlin", kts: "kotlin", java: "java" }[e] || "plaintext";
};
const defaultToolchain = () => state.course?.default_toolchain || "c";

const LANG_META = { vi: "🇻🇳 Tiếng Việt", en: "🇺🇸 English" };

const T = {
  vi: {
    summary: "Tóm tắt & Thuật ngữ", quiz: "Trắc nghiệm",
    markDone: "Đánh dấu đã học ✓", marked: "Đã học ✓", prev: "← Trước", next: "Tiếp →",
    nextCh: "Chương tiếp theo", planned: "sắp có", submit: "Nộp bài", retry: "Làm lại",
    history: "Lịch sử làm bài", run: "▶ Run", stop: "■ Stop",
    compiling: "đang biên dịch…", running: "đang chạy", exited: "đã thoát", idle: "sẵn sàng",
    args: "args:", stdinPh: "gõ input rồi Enter…", glossary: "Bảng thuật ngữ",
    score: "Điểm của bạn", tasks: "Nhiệm vụ", checksTitle: "Tiêu chí hoàn thành (chấm tự động)",
    passed: "✅ Hoàn thành!", exercise: "🧩 Bài tập", scratchChip: "🧪 Scratch",
    gateReading: "Đánh dấu đã học để mở khóa bài tiếp theo",
    gateExercise: "Chạy code đạt đủ tiêu chí bên dưới để mở khóa bài tiếp theo",
    gateQuiz: "Trả lời đúng tất cả câu hỏi để mở khóa bài tiếp theo",
    quizPerfect: "✅ Chính xác tuyệt đối!",
    scratchBrief: "Playground tự do — hai pane độc lập, chạy code thật trên máy bạn. Muốn thử gì vừa đọc thì gõ vào đây, bấm Run.",
    homeTitle: "Học theo cách tương tác",
    homeSub: "Đọc, làm quiz, và chạy code thật trên chính máy của bạn.",
    sections: "mục", correctIs: "Đáp án đúng:", progressLbl: "Tiến độ",
    chapter: "Chương", loading: "Đang tải…", question: "Câu",
    playToggle: "Ẩn/hiện Playground", wsFail: "Không kết nối được server",
    solutionTitle: "📖 Đáp án mẫu", applyToPane: "⤵ Chép vào editor",
    useSolution: "Dùng đáp án & đánh dấu hoàn thành ✓",
    applyConfirm: "Thao tác này sẽ thay code hiện tại trong editor bằng đáp án mẫu. Tiếp tục?",
  },
  en: {
    summary: "Summary & Glossary", quiz: "Quiz",
    markDone: "Mark as done ✓", marked: "Done ✓", prev: "← Prev", next: "Next →",
    nextCh: "Next chapter", planned: "soon", submit: "Submit", retry: "Retry",
    history: "Attempt history", run: "▶ Run", stop: "■ Stop",
    compiling: "compiling…", running: "running", exited: "exited", idle: "ready",
    args: "args:", stdinPh: "type input, press Enter…", glossary: "Glossary",
    score: "Your score", tasks: "Tasks", checksTitle: "Completion criteria (auto-graded)",
    passed: "✅ Passed!", exercise: "🧩 Exercise", scratchChip: "🧪 Scratch",
    gateReading: "Mark this section as done to unlock the next one",
    gateExercise: "Run your code until every criterion below passes to unlock the next section",
    gateQuiz: "Answer every question correctly to unlock the next section",
    quizPerfect: "✅ Perfect score!",
    scratchBrief: "Free playground — two independent panes running real code on your machine.",
    homeTitle: "Learn interactively",
    homeSub: "Read, take quizzes, and run real code on your own machine.",
    sections: "sections", correctIs: "Correct answer:", progressLbl: "Progress",
    chapter: "Chapter", loading: "Loading…", question: "Q",
    playToggle: "Show/hide playground", wsFail: "Could not connect to the server",
    solutionTitle: "📖 Sample solution", applyToPane: "⤵ Copy into editor",
    useSolution: "Use solution & mark as done ✓",
    applyConfirm: "This will replace the current code in the editor with the sample solution. Continue?",
  },
};
const t = (k) => (T[state.locale] || T.vi)[k] ?? T.vi[k] ?? k;
const loc = (m) => (m && (m[state.locale] || m.vi || m.en || Object.values(m)[0])) || "";
const brand = () => loc(state.course?.brand) || "Playbook";

async function api(path, opts) {
  const res = await fetch(path, opts);
  if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || res.statusText);
  return res.json();
}

async function setProgress(id, status) {
  await api("/api/progress", {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ section_id: id, status }),
  });
  if (status) state.progress[id] = status; else delete state.progress[id];
  renderSidebar();
}

// ---------- boot ----------

async function boot() {
  const data = await api("/api/course");
  state.course = data.course;
  state.progress = data.progress;
  state.locale = data.locale || state.course.default_locale || "vi";
  state.toolchains = data.toolchains || {};
  document.title = brand();
  $("#brand").innerHTML = `⚡ <b>${esc(brand())}</b>`;
  initTopbar();
  initPlayControls();
  renderSidebar();
  route();
}
window.addEventListener("hashchange", route);

function initTopbar() {
  $("#toc-toggle").onclick = () => $("#body").classList.toggle("drawer-open");
  $("#drawer-scrim").onclick = closeDrawer;
  document.addEventListener("keydown", (e) => { if (e.key === "Escape") closeDrawer(); });

  const sel = $("#lang-sel");
  sel.innerHTML = state.course.locales.map((l) =>
    `<option value="${l}" ${l === state.locale ? "selected" : ""}>${LANG_META[l] || l.toUpperCase()}</option>`).join("");
  sel.onchange = async () => {
    state.locale = sel.value;
    await api("/api/settings", {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ locale: sel.value }),
    });
    renderSidebar();
    route();
  };
}

function closeDrawer() { $("#body").classList.remove("drawer-open"); }

// ---------- sidebar ----------

function chapterProgress(ch) {
  if (!ch.sections?.length) return { done: 0, total: 0 };
  const done = ch.sections.filter((s) => state.progress[`${ch.id}/${s.id}`] === "done").length;
  return { done, total: ch.sections.length };
}

function renderSidebar() {
  const toc = $("#toc");
  toc.innerHTML = "";
  const current = location.hash;
  let totalDone = 0, totalAll = 0;

  for (const ch of state.course.chapters) {
    const wrap = document.createElement("div");
    const ready = ch.status === "ready";
    wrap.className = "toc-chapter" + (ready ? "" : " planned");
    const { done, total } = chapterProgress(ch);
    totalDone += done; totalAll += total;

    const head = document.createElement("div");
    head.className = "toc-chapter-head";
    head.innerHTML = ready
      ? `<span class="chev">▶</span><span class="head-title">${esc(loc(ch.title))}</span><span class="ch-progress">${done}/${total}</span>`
      : `<span class="chev" style="visibility:hidden">▶</span><span class="head-title">${esc(loc(ch.title))}</span><span class="badge">${t("planned")}</span>`;
    wrap.appendChild(head);

    if (ready) {
      head.onclick = () => wrap.classList.toggle("open");
      const items = document.createElement("div");
      items.className = "toc-items";
      for (const s of ch.sections) {
        const sid = `${ch.id}/${s.id}`;
        const a = document.createElement("a");
        a.className = "toc-item" + (state.progress[sid] === "done" ? " done" : "");
        a.href = `#/${ch.id}/s/${s.id}`;
        a.onclick = closeDrawer;
        a.innerHTML = `<span class="dot"></span><span>${esc(loc(s.title))}</span>`;
        if (current === a.getAttribute("href")) a.classList.add("active");
        items.appendChild(a);
      }
      const specials = [];
      if (ch.has_summary) specials.push([`#/${ch.id}/summary`, "📋", t("summary")]);
      if (ch.has_quiz) specials.push([`#/${ch.id}/quiz`, "✏️", t("quiz")]);
      for (const [href, ico, label] of specials) {
        const a = document.createElement("a");
        a.className = "toc-item special";
        a.href = href;
        a.onclick = closeDrawer;
        a.innerHTML = `<span class="ico">${ico}</span><span>${esc(label)}</span>`;
        if (current === href) a.classList.add("active");
        items.appendChild(a);
      }
      wrap.appendChild(items);
      if (current.startsWith(`#/${ch.id}/`)) wrap.classList.add("open");
    }
    toc.appendChild(wrap);
  }
  $("#overall-progress").textContent =
    totalAll ? `${t("progressLbl")}: ${totalDone}/${totalAll} ${t("sections")}` : "";
}

// ---------- routing ----------

function route() {
  renderSidebar();
  const h = location.hash.replace(/^#\/?/, "");
  const main = $("#main");
  main.scrollTop = 0;
  if (!h) return renderHome(main);
  const parts = h.split("/");
  const ch = state.course.chapters.find((c) => c.id === parts[0]);
  if (!ch) return renderHome(main);
  if (parts[1] === "s" && parts[2]) return renderSection(main, ch, parts[2]);
  if (parts[1] === "summary") return renderSummary(main, ch);
  if (parts[1] === "quiz") return renderQuiz(main, ch);
  renderHome(main);
}

function nextChapter(ch) {
  const idx = state.course.chapters.findIndex((c) => c.id === ch.id);
  for (let i = idx + 1; i < state.course.chapters.length; i++) {
    const c = state.course.chapters[i];
    if (c.status === "ready" && c.sections.length) return c;
  }
  return null;
}
const nextChapterBtn = (ch) => {
  const nc = nextChapter(ch);
  return nc ? `<a class="btn primary" href="#/${nc.id}/s/${nc.sections[0].id}">${t("nextCh")}: ${esc(loc(nc.title))} →</a>` : "";
};

// ---------- home ----------

function renderHome(main) {
  hidePlayHost();
  const cards = state.course.chapters.map((ch, i) => {
    const ready = ch.status === "ready";
    const { done, total } = chapterProgress(ch);
    const pct = total ? Math.round((done / total) * 100) : 0;
    return `<div class="chapter-card ${ready ? "" : "planned"}" ${ready ? `data-href="#/${ch.id}/s/${ch.sections[0].id}"` : ""}>
      <div class="num">${t("chapter")} ${i + 1}</div>
      <div class="name">${esc(loc(ch.title))}</div>
      <div class="meta">${ready ? `${total} ${t("sections")}${ch.has_quiz ? " · quiz" : ""}` : t("planned")}</div>
      ${ready ? `<div class="progress-bar"><div style="width:${pct}%"></div></div>` : ""}
    </div>`;
  }).join("");
  main.innerHTML = `<div class="page">
    <div class="home-hero"><h1>${esc(loc(state.course.tagline) || t("homeTitle"))}</h1><p>${esc(loc(state.course.subtitle) || t("homeSub"))}</p></div>
    <div class="chapter-cards">${cards}</div>
  </div>`;
  main.querySelectorAll(".chapter-card[data-href]").forEach((el) => {
    el.onclick = () => (location.hash = el.dataset.href);
  });
}

// ---------- section nav helpers ----------

// Returns {prevHtml, nextHtml, gateApplies}. The next link carries id="next-link"
// so gating can lock/unlock it.
function sectionNavLinks(ch, idx) {
  const prev = ch.sections[idx - 1], next = ch.sections[idx + 1];
  let nextHtml;
  if (next) nextHtml = `<a class="btn primary" id="next-link" href="#/${ch.id}/s/${next.id}">${t("next")}</a>`;
  else if (ch.has_summary) nextHtml = `<a class="btn primary" id="next-link" href="#/${ch.id}/summary">${t("summary")} →</a>`;
  else if (ch.has_quiz) nextHtml = `<a class="btn primary" id="next-link" href="#/${ch.id}/quiz">${t("quiz")} →</a>`;
  else nextHtml = nextChapterBtn(ch).replace('class="btn primary"', 'class="btn primary" id="next-link"');
  return {
    prevHtml: prev ? `<a class="btn" href="#/${ch.id}/s/${prev.id}">${t("prev")}</a>` : "",
    nextHtml,
  };
}

function setNextLocked(locked) {
  $("#next-link")?.classList.toggle("locked", locked);
}

// ---------- reading section ----------

async function renderSection(main, ch, sid) {
  const idx = ch.sections.findIndex((s) => s.id === sid);
  const sec = ch.sections[idx];
  if (!sec) return renderHome(main);
  if (sec.type === "exercise") return renderExerciseSection(main, ch, sec, idx);
  if (sec.type === "quiz") return renderQuizSection(main, ch, sec, idx);

  showScratch();
  main.innerHTML = `<div class="page"><div class="crumb"><a href="#/">${esc(brand())}</a> / ${esc(loc(ch.title))}</div><div class="md">${t("loading")}</div></div>`;
  const { html } = await api(`/api/section/${ch.id}/${sid}?lang=${state.locale}`);

  const fullId = `${ch.id}/${sid}`;
  const isDone = state.progress[fullId] === "done";
  const { prevHtml, nextHtml } = sectionNavLinks(ch, idx);

  main.innerHTML = `<div class="page">
    <div class="crumb"><a href="#/">${esc(brand())}</a> / ${esc(loc(ch.title))}</div>
    <div class="md">${html}</div>
    <div class="section-nav">
      ${prevHtml}
      <span class="spacer"></span>
      <span class="gate-hint" id="gate-hint" ${isDone ? "hidden" : ""}>${t("gateReading")}</span>
      <button class="done-btn ${isDone ? "marked" : ""}">${isDone ? t("marked") : t("markDone")}</button>
      ${nextHtml}
    </div>
  </div>`;
  main.querySelectorAll("pre code").forEach((el) => window.hljs?.highlightElement(el));
  main.querySelectorAll(".md a[href^='http']").forEach((a) => { a.target = "_blank"; a.rel = "noopener"; });
  setNextLocked(!isDone);

  $(".done-btn", main).onclick = async (e) => {
    const nowDone = state.progress[fullId] !== "done";
    await setProgress(fullId, nowDone ? "done" : "");
    // the page may have been swapped out while setProgress was in flight
    const hint = $("#gate-hint", main);
    if (!hint) return;
    e.target.classList.toggle("marked", nowDone);
    e.target.textContent = nowDone ? t("marked") : t("markDone");
    hint.hidden = nowDone;
    setNextLocked(!nowDone);
  };
}

// ---------- exercise section ----------

async function renderExerciseSection(main, ch, sec, idx) {
  main.innerHTML = `<div class="page"><div class="crumb"><a href="#/">${esc(brand())}</a> / ${esc(loc(ch.title))}</div><p>${t("loading")}</p></div>`;
  const { exercises, files, solution_files: solutionFiles = {} } = await exerciseData(ch.id);
  const ex = exercises.find((e) => e.id === sec.exercise);
  if (!ex) { main.innerHTML = `<div class="page"><p>exercise not found</p></div>`; return; }

  const fullId = `${ch.id}/${sec.id}`;
  const isDone = state.progress[fullId] === "done";
  const { prevHtml, nextHtml } = sectionNavLinks(ch, idx);

  const tasks = loc(ex.tasks).split("\n").filter(Boolean)
    .map((l) => `<li>${esc(l.replace(/^[-*]\s*/, ""))}</li>`).join("");
  const checks = (ex.checks || []).map((c, i) =>
    `<li data-check="${i}">${esc(loc(c.label))}</li>`).join("");

  const sol = ex.solution;
  let solHtml = "";
  if (sol) {
    const solFilesHtml = (sol.files || []).map((f) => `
      <div class="sol-file">
        <div class="sol-file-head"><code>${esc(f)}</code>
          <button class="sol-apply" data-f="${esc(f)}">${t("applyToPane")}</button></div>
        <pre><code class="language-${hlClassFor(f)}">${esc(solutionFiles[f] || "")}</code></pre>
      </div>`).join("");
    solHtml = `<details class="ex-solution">
      <summary>${t("solutionTitle")}</summary>
      <p class="sol-notes">${esc(loc(sol.notes))}</p>
      ${solFilesHtml}
      <button class="sol-use">${t("useSolution")}</button>
    </details>`;
  }

  main.innerHTML = `<div class="page">
    <div class="crumb"><a href="#/">${esc(brand())}</a> / ${esc(loc(ch.title))}</div>
    <div class="md">
      <h1>${esc(loc(ex.title))}</h1>
      <p>${esc(loc(ex.brief))}</p>
      <div class="ex-checks-title">${t("tasks")}</div>
      <ul class="ex-tasks">${tasks}</ul>
      ${checks ? `<div class="ex-checks-title">${t("checksTitle")}</div><ul class="ex-checks" id="ex-checks">${checks}</ul>` : ""}
      ${solHtml}
    </div>
    <div class="section-nav">
      ${prevHtml}
      <span class="spacer"></span>
      <span class="gate-hint" id="gate-hint" ${isDone ? "hidden" : ""}>${t("gateExercise")}</span>
      ${nextHtml}
    </div>
  </div>`;
  main.querySelectorAll("pre code").forEach((el) => window.hljs?.highlightElement(el));
  setNextLocked(!isDone);

  showExercise(ch, sec, ex, files);

  const applyFile = (f) => {
    const paneIdx = ex.panes.findIndex((p) => p.files.includes(f));
    const pane = host.slots[`${ch.id}/${ex.id}`]?.panes[paneIdx];
    if (!pane || !(f in solutionFiles)) return;
    pane.files[f] = solutionFiles[f];
    if (pane.current === f) pane.cm.setValue(solutionFiles[f]);
  };
  main.querySelectorAll(".sol-apply").forEach((btn) => {
    btn.onclick = () => { if (confirm(t("applyConfirm"))) applyFile(btn.dataset.f); };
  });
  const useBtn = $(".sol-use", main);
  if (useBtn) useBtn.onclick = async () => {
    if (!confirm(t("applyConfirm"))) return;
    (sol.files || []).forEach(applyFile);
    await setProgress(fullId, "done").catch(() => {});
    $("#gate-hint", main)?.setAttribute("hidden", "");
    setNextLocked(false);
    updateChecks();
  };
}

// ---------- playground host (right pane) ----------

const scratchDef = () => {
  const tc = TOOLCHAINS[defaultToolchain()] || TOOLCHAINS.c;
  return { toolchain: defaultToolchain(), files: { [tc.scratchFile]: tc.scratch } };
};

const host = {
  built: false,
  exData: {},     // chId -> {exercises, files}
  slots: {},      // key ("scratch" | "<ch>/<exId>") -> {el, panes}
  visibleKey: null,
  context: null,  // {ch, sec, ex} when on an exercise section
  mode: "scratch",
};

async function exerciseData(chId) {
  if (!host.exData[chId]) host.exData[chId] = await api(`/api/exercises/${chId}`);
  return host.exData[chId];
}

function ensureHost() {
  if (host.built) return;
  const el = $("#play-host");
  el.innerHTML = `
    <div class="play-host-bar">
      <span class="chips" id="play-chips" hidden>
        <button class="chip" data-m="exercise" id="chip-ex"></button>
        <button class="chip" data-m="scratch" id="chip-scratch"></button>
      </span>
      <span class="grow"></span>
      <span class="check-badge" id="check-badge"></span>
    </div>
    <div id="slots"></div>`;
  $("#chip-ex").onclick = () => setMode("exercise");
  $("#chip-scratch").onclick = () => setMode("scratch");
  host.built = true;
}

function makeSlot(key, paneDefs) {
  if (host.slots[key]) return host.slots[key];
  const slotEl = document.createElement("div");
  slotEl.className = "play-panes";
  slotEl.hidden = true;
  $("#slots").appendChild(slotEl);
  const panes = paneDefs.map((d, i) => {
    if (i > 0) slotEl.appendChild(makePaneDrag(slotEl));
    const pd = document.createElement("div");
    pd.className = "pane";
    slotEl.appendChild(pd);
    return new Pane(pd, d.name, d.files, d.args, d.toolchain);
  });
  host.slots[key] = { el: slotEl, panes };
  return host.slots[key];
}

// Divider between two playground panes: resizes the pane before it,
// horizontally or vertically depending on the current flex direction.
function makePaneDrag(slotEl) {
  const dv = document.createElement("div");
  dv.className = "pane-drag";
  dv.onmousedown = (e) => {
    e.preventDefault();
    const prev = dv.previousElementSibling;
    const row = getComputedStyle(slotEl).flexDirection === "row";
    const move = (ev) => {
      const r = slotEl.getBoundingClientRect();
      const pct = row
        ? ((ev.clientX - r.left) / r.width) * 100
        : ((ev.clientY - r.top) / r.height) * 100;
      prev.style.flex = `0 0 calc(${Math.max(15, Math.min(85, pct))}% - 3px)`;
    };
    const up = () => {
      document.removeEventListener("mousemove", move);
      document.removeEventListener("mouseup", up);
      host.slots[host.visibleKey]?.panes.forEach((p) => p.cm.refresh());
    };
    document.addEventListener("mousemove", move);
    document.addEventListener("mouseup", up);
  };
  return dv;
}

function showSlot(key) {
  for (const [k, s] of Object.entries(host.slots)) s.el.hidden = k !== key;
  host.visibleKey = key;
  requestAnimationFrame(() => host.slots[key]?.panes.forEach((p) => p.cm.refresh()));
}

function scratchSlot() {
  const s = scratchDef();
  return makeSlot("scratch", [
    { name: "Pane A", files: { ...s.files }, args: "", toolchain: s.toolchain },
    { name: "Pane B", files: { ...s.files }, args: "", toolchain: s.toolchain },
  ]);
}

// User preference: playground hidden while reading (persisted). Exercise
// sections force it visible on entry, but the toggle always wins afterwards.
const playPref = {
  get hidden() { try { return localStorage.getItem("playHidden") === "1"; } catch { return false; } },
  set hidden(v) { try { localStorage.setItem("playHidden", v ? "1" : ""); } catch {} },
};
const playCtx = { wants: false, force: false }; // what the current page asks for

function playShown() {
  return playCtx.wants && (playCtx.force || !playPref.hidden);
}

function applyPlayVisibility() {
  const show = playShown();
  $("#play-host").hidden = !show;
  $("#split-drag").hidden = !show;
  $("#content-split").classList.toggle("with-play", show);
  const btn = $("#play-toggle");
  btn.hidden = !playCtx.wants;
  btn.classList.toggle("active", show);
  btn.title = t("playToggle");
  if (show) requestAnimationFrame(() => host.slots[host.visibleKey]?.panes.forEach((p) => p.cm.refresh()));
}

function hidePlayHost() {
  playCtx.wants = false;
  playCtx.force = false;
  applyPlayVisibility();
}

function showPlayHost(force) {
  ensureHost();
  playCtx.wants = true;
  playCtx.force = !!force;
  applyPlayVisibility();
}

function initPlayControls() {
  $("#play-toggle").onclick = () => {
    playPref.hidden = playShown(); // hide if shown, show if hidden
    playCtx.force = false;
    applyPlayVisibility();
  };

  const drag = $("#split-drag");
  try {
    const pct = localStorage.getItem("playPct");
    if (pct) $("#play-host").style.flex = `0 0 ${pct}%`;
  } catch {}
  drag.onmousedown = (e) => {
    e.preventDefault();
    const split = $("#content-split");
    let pct = null;
    const move = (ev) => {
      const r = split.getBoundingClientRect();
      pct = Math.max(24, Math.min(72, ((r.right - ev.clientX) / r.width) * 100));
      $("#play-host").style.flex = `0 0 ${pct}%`;
    };
    const up = () => {
      document.removeEventListener("mousemove", move);
      document.removeEventListener("mouseup", up);
      if (pct != null) { try { localStorage.setItem("playPct", pct.toFixed(1)); } catch {} }
      host.slots[host.visibleKey]?.panes.forEach((p) => p.cm.refresh());
    };
    document.addEventListener("mousemove", move);
    document.addEventListener("mouseup", up);
  };
}

function showScratch() {
  showPlayHost(false);
  host.context = null;
  host.mode = "scratch";
  $("#play-chips").hidden = true;
  $("#check-badge").textContent = "";
  scratchSlot();
  showSlot("scratch");
}

function showExercise(ch, sec, ex, files) {
  showPlayHost(true);
  host.context = { ch, sec, ex };
  host.mode = "exercise";
  const key = `${ch.id}/${ex.id}`;
  const slot = makeSlot(key, ex.panes.map((p) => {
    const paneFiles = {};
    for (const f of p.files) paneFiles[f] = files[f] || "";
    return { name: loc(p.name), files: paneFiles, args: p.default_args,
             toolchain: p.toolchain || defaultToolchain() };
  }));
  slot.panes.forEach((p) => (p.onOutput = updateChecks));
  scratchSlot(); // make sure scratch exists for the chip
  $("#play-chips").hidden = false;
  $("#chip-ex").textContent = t("exercise");
  $("#chip-scratch").textContent = t("scratchChip");
  setMode("exercise");
  updateChecks();
}

function setMode(mode) {
  host.mode = mode;
  $("#chip-ex").classList.toggle("active", mode === "exercise");
  $("#chip-scratch").classList.toggle("active", mode === "scratch");
  if (mode === "scratch" || !host.context) showSlot("scratch");
  else showSlot(`${host.context.ch.id}/${host.context.ex.id}`);
}

function updateChecks() {
  const ctx = host.context;
  if (!ctx) return;
  const slot = host.slots[`${ctx.ch.id}/${ctx.ex.id}`];
  const checks = ctx.ex.checks || [];
  const badge = $("#check-badge");
  const fullId = `${ctx.ch.id}/${ctx.sec.id}`;
  const already = state.progress[fullId] === "done";
  if (!checks.length) { badge.textContent = ""; return; }
  let allPass = true;
  checks.forEach((c, i) => {
    const pass = !!slot.panes[c.pane] && new RegExp(c.pattern).test(slot.panes[c.pane].outBuf);
    if (!pass) allPass = false;
    document.querySelector(`#ex-checks [data-check="${i}"]`)?.classList.toggle("pass", pass || already);
  });
  badge.textContent = allPass || already ? t("passed") : "";
  badge.classList.toggle("pass", allPass || already);
  if ((allPass || already)) {
    $("#gate-hint")?.setAttribute("hidden", "");
    setNextLocked(false);
  }
  if (allPass && !already) setProgress(fullId, "done").catch(() => {});
}

// ---------- summary + glossary ----------

async function renderSummary(main, ch) {
  showScratch();
  main.innerHTML = `<div class="page"><div class="crumb"><a href="#/">${esc(brand())}</a> / ${esc(loc(ch.title))}</div><div class="md">${t("loading")}</div></div>`;
  const [{ html }, glossary] = await Promise.all([
    api(`/api/chapter/${ch.id}/summary?lang=${state.locale}`),
    api(`/api/chapter/${ch.id}/glossary?lang=${state.locale}`).catch(() => null),
  ]);
  let gloss = "";
  if (glossary?.length) {
    gloss = `<h2>${t("glossary")}</h2><table class="glossary-table">` +
      glossary.map((g) => `<tr><td>${esc(g.term)}</td><td>${esc(g.def)}</td></tr>`).join("") +
      `</table>`;
  }
  main.innerHTML = `<div class="page">
    <div class="crumb"><a href="#/">${esc(brand())}</a> / ${esc(loc(ch.title))}</div>
    <div class="md">${html}${gloss}</div>
    <div class="section-nav">
      <span class="spacer"></span>
      ${ch.has_quiz ? `<a class="btn primary" href="#/${ch.id}/quiz">${t("quiz")} →</a>` : nextChapterBtn(ch)}
    </div>
  </div>`;
  main.querySelectorAll("pre code").forEach((el) => window.hljs?.highlightElement(el));
}

// ---------- quiz ----------

function quizQuestionsHTML(quiz) {
  return quiz.questions.map((q, i) => `
    <div class="quiz-q" data-qid="${q.id}">
      <div class="qnum">${t("question")} ${i + 1}/${quiz.questions.length}</div>
      <div class="prompt">${esc(q.prompt)}</div>
      ${q.choices.map((c, j) => `
        <label class="quiz-choice" data-idx="${j}">
          <input type="radio" name="${q.id}" value="${j}"><span>${esc(c)}</span>
        </label>`).join("")}
      <div class="explanation" style="display:none"></div>
    </div>`).join("");
}

function collectQuizAnswers(scope, quiz) {
  const answers = {};
  for (const q of quiz.questions) {
    const sel = scope.querySelector(`input[name="${q.id}"]:checked`);
    if (sel) answers[q.id] = parseInt(sel.value, 10);
  }
  return answers;
}

function applyQuizResults(scope, results) {
  for (const r of results) {
    const box = scope.querySelector(`.quiz-q[data-qid="${r.id}"]`);
    if (!box) continue;
    box.querySelectorAll("input").forEach((inp) => (inp.disabled = true));
    const choices = box.querySelectorAll(".quiz-choice");
    choices[r.correct]?.classList.add("correct");
    if (r.chosen >= 0 && !r.is_correct) choices[r.chosen]?.classList.add("wrong");
    const ex = box.querySelector(".explanation");
    ex.style.display = "";
    ex.textContent = (r.is_correct ? "✓ " : `✗ ${t("correctIs")} ${String.fromCharCode(65 + r.correct)}. `) + r.explanation;
  }
}

async function renderQuiz(main, ch) {
  showScratch();
  main.innerHTML = `<div class="page"><div class="crumb"><a href="#/">${esc(brand())}</a> / ${esc(loc(ch.title))}</div><p>${t("loading")}</p></div>`;
  const [quiz, attempts] = await Promise.all([
    api(`/api/chapter/${ch.id}/quiz?lang=${state.locale}`),
    api(`/api/quiz/${ch.id}/attempts`).catch(() => []),
  ]);

  const qHtml = quizQuestionsHTML(quiz);

  const hist = attempts.length
    ? `<div class="attempts-list">${t("history")}:<ul>` +
      attempts.map((a) => `<li>${new Date(a.created_at * 1000).toLocaleString()} — ${a.score}/${a.total}</li>`).join("") +
      `</ul></div>`
    : "";

  main.innerHTML = `<div class="page">
    <div class="crumb"><a href="#/">${esc(brand())}</a> / ${esc(loc(ch.title))}</div>
    <h1>${t("quiz")} — ${esc(loc(ch.title))}</h1>
    <div id="quiz-score"></div>
    <form id="quiz-form">${qHtml}</form>
    <div class="section-nav">
      <span class="spacer"></span>
      <button class="primary" id="quiz-submit">${t("submit")}</button>
    </div>
    ${hist}
  </div>`;

  $("#quiz-submit", main).onclick = async (e) => {
    e.preventDefault();
    const answers = collectQuizAnswers(main, quiz);
    const res = await api(`/api/quiz/${ch.id}/attempt`, {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ answers }),
    });
    $("#quiz-score", main).innerHTML = `<div class="quiz-score">
      <div>${t("score")}</div><div class="big">${res.score}/${res.total}</div>
      <div class="score-actions">
        <button id="quiz-retry">${t("retry")}</button>
        ${nextChapterBtn(ch)}
      </div>
    </div>`;
    $("#quiz-retry", main).onclick = () => renderQuiz(main, ch);
    applyQuizResults(main, res.results);
    $("#quiz-submit", main).disabled = true;
    $("#quiz-score", main).scrollIntoView({ behavior: "smooth" });
  };
}

// ---------- inline quiz section (gated, placed anywhere in a chapter) ----------

async function renderQuizSection(main, ch, sec, idx) {
  showScratch();
  main.innerHTML = `<div class="page"><div class="crumb"><a href="#/">${esc(brand())}</a> / ${esc(loc(ch.title))}</div><p>${t("loading")}</p></div>`;
  const quiz = await api(`/api/chapter/${ch.id}/quiz/${sec.quiz}?lang=${state.locale}`);

  const fullId = `${ch.id}/${sec.id}`;
  const isDone = state.progress[fullId] === "done";
  const { prevHtml, nextHtml } = sectionNavLinks(ch, idx);

  main.innerHTML = `<div class="page">
    <div class="crumb"><a href="#/">${esc(brand())}</a> / ${esc(loc(ch.title))}</div>
    <div class="md"><h1>${esc(loc(sec.title))}</h1></div>
    <div id="quiz-score"></div>
    <form id="quiz-form">${quizQuestionsHTML(quiz)}</form>
    <div class="section-nav">
      ${prevHtml}
      <span class="spacer"></span>
      <span class="gate-hint" id="gate-hint" ${isDone ? "hidden" : ""}>${t("gateQuiz")}</span>
      <button class="primary" id="quiz-submit">${t("submit")}</button>
      ${nextHtml}
    </div>
  </div>`;
  setNextLocked(!isDone);

  $("#quiz-submit", main).onclick = async (e) => {
    e.preventDefault();
    const answers = collectQuizAnswers(main, quiz);
    const res = await api(`/api/quiz/${ch.id}/${sec.quiz}/attempt`, {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ answers }),
    });
    applyQuizResults(main, res.results);
    const perfect = res.score === res.total;
    const scoreEl = $("#quiz-score", main);
    scoreEl.innerHTML = `<div class="quiz-score compact">
      <div class="big">${res.score}/${res.total}</div>
      ${perfect ? `<div class="perfect">${t("quizPerfect")}</div>` : ""}
    </div>`;
    const btn = $("#quiz-submit", main);
    if (perfect || isDone) {
      btn.disabled = true;
      $("#gate-hint", main)?.setAttribute("hidden", "");
      setNextLocked(false);
      if (perfect && !isDone) await setProgress(fullId, "done").catch(() => {});
    } else {
      btn.textContent = t("retry");
      btn.onclick = (ev) => { ev.preventDefault(); renderQuizSection(main, ch, sec, idx); };
    }
    scoreEl.scrollIntoView({ behavior: "smooth" });
  };
}

// ---------- playground pane ----------

class Pane {
  constructor(root, name, files, defaultArgs, toolchain) {
    this.root = root;
    this.files = { ...files };
    this.fileNames = Object.keys(files);
    this.current = this.fileNames[0];
    this.toolchain = toolchain || defaultToolchain();
    this.ws = null;
    this.running = false;
    this.outBuf = "";
    this.onOutput = null;

    root.innerHTML = `
      <div class="pane-bar">
        <span class="pane-name">${esc(name)}</span>
        <span class="file-tabs">${this.fileNames.map((f, i) =>
          `<span class="file-tab ${i === 0 ? "active" : ""}" data-f="${esc(f)}">${esc(f)}</span>`).join("")}</span>
        <span class="grow"></span>
        <span class="status">${t("idle")}</span>
        <button class="run-btn primary">${t("run")}</button>
      </div>
      <div class="editor-wrap"></div>
      <div class="out-drag" title="⇕"></div>
      <div class="args-row"><span>${t("args")}</span><input class="args" value="${esc(defaultArgs || "")}"></div>
      <pre class="term"></pre>
      <div class="stdin-row"><input class="stdin" placeholder="${t("stdinPh")}"></div>`;

    this.term = $(".term", root);
    this.statusEl = $(".status", root);
    this.runBtn = $(".run-btn", root);
    this.cm = CodeMirror($(".editor-wrap", root), {
      value: this.files[this.current] || "",
      mode: cmModeFor(this.current),
      theme: "material-darker",
      lineNumbers: true,
      indentUnit: 4, tabSize: 4, indentWithTabs: false,
    });

    root.querySelectorAll(".file-tab").forEach((tab) => {
      tab.onclick = () => {
        this.files[this.current] = this.cm.getValue();
        this.current = tab.dataset.f;
        root.querySelectorAll(".file-tab").forEach((x) => x.classList.toggle("active", x === tab));
        this.cm.setValue(this.files[this.current] || "");
        this.cm.setOption("mode", cmModeFor(this.current));
      };
    });
    this.runBtn.onclick = () => (this.running ? this.kill() : this.run());

    // per-pane editor/output divider: drag sets the code area's height,
    // the output area takes the rest; double-click resets the split.
    const editorWrap = $(".editor-wrap", root);
    const outDrag = $(".out-drag", root);
    outDrag.onmousedown = (e) => {
      e.preventDefault();
      const move = (ev) => {
        const top = editorWrap.getBoundingClientRect().top;
        const max = root.clientHeight - 150; // keep the bars + some output visible
        const h = Math.max(60, Math.min(max, ev.clientY - top));
        editorWrap.style.flex = `0 0 ${h}px`;
      };
      const up = () => {
        document.removeEventListener("mousemove", move);
        document.removeEventListener("mouseup", up);
        this.cm.refresh();
      };
      document.addEventListener("mousemove", move);
      document.addEventListener("mouseup", up);
    };
    outDrag.ondblclick = () => { editorWrap.style.flex = ""; this.cm.refresh(); };
    $(".stdin", root).onkeydown = (e) => {
      if (e.key === "Enter") {
        this.send({ op: "stdin", data: e.target.value + "\n" });
        e.target.value = "";
      }
    };
  }

  print(text, cls) {
    const span = document.createElement("span");
    if (cls) span.className = cls;
    span.textContent = text;
    this.term.appendChild(span);
    this.term.scrollTop = this.term.scrollHeight;
  }

  setStatus(text, running) {
    this.statusEl.textContent = text;
    this.statusEl.classList.toggle("running", !!running);
    this.running = !!running;
    this.runBtn.textContent = running ? t("stop") : t("run");
    this.runBtn.classList.toggle("primary", !running);
  }

  connect() {
    return new Promise((resolve, reject) => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) return resolve();
      const ws = new WebSocket(`ws://${location.host}/ws/run`);
      ws.onopen = () => resolve();
      ws.onerror = reject;
      ws.onclose = () => { if (this.running) this.setStatus(t("exited"), false); this.ws = null; };
      ws.onmessage = (ev) => {
        const m = JSON.parse(ev.data);
        if (m.type === "out") {
          this.print(m.data);
          this.outBuf += m.data;
          this.onOutput?.();
        } else if (m.type === "status") {
          this.setStatus(m.data === "compiling" ? t("compiling") : t("running"), m.data === "running");
        } else if (m.type === "exit") {
          this.print(`\n[exit ${m.code}]\n`, "sys");
          this.setStatus(t("exited"), false);
          this.onOutput?.();
        } else if (m.type === "error") {
          this.print(m.data + "\n", "err");
          this.setStatus(t("idle"), false);
        }
      };
      this.ws = ws;
    });
  }

  send(obj) { if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify(obj)); }

  async run() {
    this.files[this.current] = this.cm.getValue();
    this.term.textContent = "";
    this.outBuf = "";
    try { await this.connect(); } catch { this.print(t("wsFail") + "\n", "err"); return; }
    const args = $(".args", this.root).value.trim();
    this.send({
      op: "run",
      toolchain: this.toolchain,
      files: Object.entries(this.files).map(([name, content]) => ({ name, content })),
      args: args ? args.split(/\s+/) : [],
    });
  }

  kill() { this.send({ op: "kill" }); }
}

boot().catch((e) => {
  $("#main").innerHTML = `<div class="page"><p style="color:var(--red)">Lỗi khởi động: ${esc(e.message)}</p></div>`;
});
