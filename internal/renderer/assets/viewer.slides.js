/* Slides: one slice per slide, for readers who do not know Event Modeling.
   Order of attention: the screen, then the examples (given / when / then), then the
   event model steps behind the screen. */
(function(){
  const host = document.getElementById("board-slides");
  if(!host || !window.EMC) return;
  const {esc, PAT_SVG, STATUS_LABEL, statusVar, titleize} = EMC;

  // What each slice pattern means, in words for a reader who does not know the method.
  const PATTERN_MEANING = {
    state_change: "A person changes something in the system.",
    state_view: "A person looks at information from the system.",
    automation: "The system does this step on its own.",
    translation: "The system takes in information from another system.",
  };
  const NO_SCREEN = {
    automation: "This step has no screen. The system runs it on its own.",
    translation: "This step has no screen. Information comes in from another system.",
  };
  const KIND_GROUPS = [
    {kinds:["command"], title:"Commands", meaning:"Requests to change something."},
    {kinds:["event"], title:"Events", meaning:"Facts that the system records."},
    {kinds:["readmodel"], title:"Read models", meaning:"Information that the system prepares for a screen or a process."},
    {kinds:["table"], title:"Tables", meaning:"Records that the system stores."},
    {kinds:["processor"], title:"Processors", meaning:"Work that the system does on its own."},
  ];

  function formatValue(value){
    if(value === null || value === undefined) return "—";
    if(typeof value === "string") return value;
    if(typeof value === "object") return JSON.stringify(value);
    return String(value);
  }
  // Example data reads as "name: value" rows instead of raw JSON.
  function examplesHTML(examples){
    if(examples === undefined || examples === null) return "";
    const rows = (Array.isArray(examples) ? examples : [examples]).map(item=>{
      if(item && typeof item === "object" && !Array.isArray(item)){
        return `<dl class="sl-data">${Object.entries(item).map(([key,value])=>
          `<dt>${esc(key)}</dt><dd>${esc(formatValue(value))}</dd>`).join("")}</dl>`;
      }
      return `<div class="sl-data-value">${esc(formatValue(item))}</div>`;
    });
    return rows.join("");
  }

  function stepHTML(step){
    const parts = [`<div class="sl-step-title">${esc(step.title)}</div>`];
    parts.push(examplesHTML(step.examples));
    if(step.fields && step.fields.length) parts.push(`<div class="sl-step-note">With ${step.fields.map(esc).join(", ")}</div>`);
    if(step.error) parts.push(`<div class="sl-step-error">Fails with: ${esc(step.error)}</div>`);
    if(step.emptyList) parts.push(`<div class="sl-step-note">Expects an empty list</div>`);
    return parts.join("");
  }

  function scenarioHTML(scenario){
    const rows = [];
    const add = (key, step, failed) => rows.push(
      `<div class="sl-gwt sl-${key.toLowerCase()}${failed?" failed":""}"><span class="sl-gwt-key">${key}</span><div class="sl-gwt-body">${stepHTML(step)}</div></div>`);
    (scenario.given || []).forEach(step=>add("Given", step));
    if(scenario.when) add("When", scenario.when);
    (scenario.then || []).forEach(step=>add("Then", step, !!step.error));
    const comments = (scenario.comments || []).map(comment=>`<p class="sl-comment">${esc(comment)}</p>`).join("");
    return `<article class="sl-scenario"><h4>${esc(scenario.title)}</h4>${rows.join("")}${comments}</article>`;
  }

  function fieldsHTML(fields){
    if(!fields || !fields.length) return "";
    return `<dl class="sl-fields">${fields.map(field=>
      `<dt>${esc(field.name)}${field.id?' <i class="id">id</i>':""}${field.pii?' <i class="pii">pii</i>':""}</dt><dd>${esc(field.type)}</dd>`).join("")}</dl>`;
  }

  function actorTitle(id){
    return id ? ((EMC.MODEL.actors[id] && EMC.MODEL.actors[id].title) || titleize(id)) : "";
  }

  function captionHTML(title, actorID){
    const actor = actorTitle(actorID);
    if(!title && !actor) return "";
    return `<figcaption>${title ? `<span class="sl-screen-title">${esc(title)}</span>` : ""}`+
      (actor ? `<span class="sl-screen-actor">Used by ${esc(actor)}</span>` : "")+`</figcaption>`;
  }

  // A screenshot shows the screen at the same stage; its fields are listed under the image.
  function shotHTML(image, screen, hero){
    const fields = screen && screen.fields && screen.fields.length
      ? `<p class="sl-shot-fields">Fields: ${screen.fields.map(field=>`<span class="mono">${esc(field.name)}</span>`).join(", ")}</p>` : "";
    return `<figure class="sl-screen${hero?" hero":""}"><div class="sl-shot">`+
      `<img src="${esc(image.imageUrl)}" alt="Screenshot: ${esc(image.title)}" referrerpolicy="no-referrer">`+
      `<div class="sl-shot-failed">The screenshot did not load.<span class="mono">${esc(image.imageUrl)}</span></div>`+
      `</div>${captionHTML(screen ? screen.title : image.title, (screen || image).actor)}${fields}</figure>`;
  }

  // Without a screenshot, an honest wireframe built from the fields of the screen.
  function wireHTML(screen, hero){
    const inputs = (screen.fields || []).map(field=>`<div class="sl-input"><span>${esc(field.name)}</span><span class="sl-input-box">${esc(field.type)}</span></div>`).join("");
    return `<figure class="sl-screen sl-wire${hero?" hero":""}"><div class="sl-wireframe">`+
      `<div class="sl-wire-body"><div class="sl-wire-title">${esc(screen.title)}</div>${inputs || '<div class="sl-wire-empty">No fields in the model</div>'}</div>`+
      `<div class="sl-wire-note">Wireframe. The model has no screenshot for this screen.</div>`+
      `</div>${captionHTML(null, screen.actor)}</figure>`;
  }

  function heroHTML(slice){
    const screens = slice.elements.filter(element=>element.kind === "screen");
    const images = slice.elements.filter(element=>element.kind === "screen_image" && element.imageUrl);
    if(!screens.length && !images.length){
      const processor = slice.elements.find(element=>element.kind === "processor");
      return `<div class="sl-noscreen"><span class="sl-noscreen-icon" aria-hidden="true">${PAT_SVG[slice.type] || ""}</span>`+
        `<p>${esc(NO_SCREEN[slice.type] || "This step has no screen.")}</p>`+
        (processor ? `<p class="sl-noscreen-proc">${esc(processor.title)}</p>` : "")+`</div>`;
    }
    const paired = new Set();
    const items = images.map(image=>{
      const screen = screens.find(candidate=>!paired.has(candidate) && candidate.stage === image.stage);
      if(screen) paired.add(screen);
      return hero=>shotHTML(image, screen, hero);
    });
    screens.filter(screen=>!paired.has(screen)).forEach(screen=>items.push(hero=>wireHTML(screen, hero)));
    const [first, ...others] = items;
    return first(true)+(others.length ? `<div class="sl-other-screens">${others.map(item=>item(false)).join("")}</div>` : "");
  }

  function questionsHTML(slice){
    const ids = new Set(["slice__" + slice.id, ...slice.elements.map(element=>element.id)]);
    const open = (EMC.MODEL.hotspots || []).filter(hotspot=>ids.has(hotspot.onId));
    if(!open.length) return "";
    return `<section class="sl-questions"><h3>Open questions</h3><ul>${open.map(hotspot=>
      `<li>${esc(hotspot.question)}<span class="sl-question-status">${esc(titleize(hotspot.status || "open"))}</span></li>`).join("")}</ul></section>`;
  }

  function detailsHTML(slice){
    return KIND_GROUPS.map(group=>{
      const elements = slice.elements.filter(element=>group.kinds.includes(element.kind));
      if(!elements.length) return "";
      return `<section class="sl-kind"><h4>${group.title}</h4><p class="sl-kind-meaning">${group.meaning}</p>`+
        elements.map(element=>`<div class="sl-element sl-${element.kind}${element.external?" external":""}">`+
          `<div class="sl-element-title">${esc(element.title)}</div>`+
          (element.question ? `<div class="sl-element-note">Answers: “${esc(element.question)}”</div>` : "")+
          (element.api ? `<div class="sl-element-note mono">${esc(element.api)}</div>` : "")+
          (element.external ? `<div class="sl-element-note">From another system</div>` : "")+
          fieldsHTML(element.fields)+`</div>`).join("")+`</section>`;
    }).join("");
  }

  function navLink(className, target, label, title){
    if(!target) return `<span class="sl-nav-btn ${className}" aria-disabled="true"><span class="sl-nav-label">${label}</span><span class="sl-nav-title">${esc(title)}</span></span>`;
    return `<a class="sl-nav-btn ${className}" href="${esc(target)}"><span class="sl-nav-label">${label}</span><span class="sl-nav-title">${esc(title)}</span></a>`;
  }

  let current = null; // {index, prevHref, nextHref, go}
  window.renderSlides = function(index, nav){
    const slices = EMC.MODEL.slices;
    if(!slices.length){
      host.innerHTML = `<p class="sl-empty">This chapter has no slices.</p>`;
      current = null;
      return;
    }
    const position = Math.min(Math.max(1, index), slices.length);
    const slice = slices[position-1];
    const status = slice.status || "created";
    const prev = position > 1
      ? {href:nav.slideHref(position-1), label:"‹ Previous", title:slices[position-2].title}
      : nav.prevChapter ? {href:nav.prevChapter.href, label:"‹ Previous chapter", title:nav.prevChapter.title}
      : {href:null, label:"‹ Previous", title:"This is the first slide"};
    const next = position < slices.length
      ? {href:nav.slideHref(position+1), label:"Next ›", title:slices[position].title}
      : nav.nextChapter ? {href:nav.nextChapter.href, label:"Next chapter ›", title:nav.nextChapter.title}
      : {href:null, label:"Next ›", title:"This is the last slide"};
    const scenarios = slice.scenarios || [];
    const menu = slices.map((item, i)=>`<a href="${esc(nav.slideHref(i+1))}"${i+1===position?' aria-current="page"':""}>`+
      `<span class="n">${String(i+1).padStart(2,"0")}</span><span class="t">${esc(item.title)}</span></a>`).join("");

    host.innerHTML =
      `<div class="sl-progress" role="progressbar" aria-label="Slide progress" aria-valuemin="1" aria-valuemax="${slices.length}" aria-valuenow="${position}"><span style="width:${position/slices.length*100}%"></span></div>`+
      `<div class="sl-stage" tabindex="-1">`+
        `<article class="sl-slide" aria-roledescription="slide" aria-label="Slide ${position} of ${slices.length}: ${esc(slice.title)}">`+
          `<header class="sl-head">`+
            `<div class="sl-head-row">`+
              `<details class="popover sl-picker"><summary class="toolbtn">Slide ${position} of ${slices.length}<span class="cv" aria-hidden="true">▾</span></summary>`+
                `<div class="popover-panel chapter-menu sl-menu">${menu}</div></details>`+
              (document.fullscreenEnabled ? `<button type="button" class="toolbtn sl-fullscreen">${document.fullscreenElement ? "Exit full screen" : "Full screen"}</button>` : "")+
            `</div>`+
            `<h2>${esc(slice.title)}</h2>`+
            `<p class="sl-pattern"><span class="sl-pattern-icon" aria-hidden="true">${PAT_SVG[slice.type] || ""}</span>${esc(PATTERN_MEANING[slice.type] || "")}`+
              `<span class="status" style="--sc:${statusVar(status)}"><span class="sd"></span>${esc(STATUS_LABEL[status] || titleize(status))}</span></p>`+
            (slice.description ? `<p class="sl-desc">${esc(slice.description)}</p>` : "")+
          `</header>`+
          `<div class="sl-main${scenarios.length ? "" : " no-examples"}">`+
            `<section class="sl-screens" aria-label="Screens">${heroHTML(slice)}</section>`+
            `<section class="sl-examples"><h3>Examples</h3>`+
              (scenarios.length
                ? `<p class="sl-hint">Given: what is already true. When: what happens. Then: the expected result.</p>${scenarios.map(scenarioHTML).join("")}`
                : `<p class="sl-empty">The model has no examples for this slice.</p>`)+
            `</section>`+
          `</div>`+
          questionsHTML(slice)+
          `<section class="sl-behind"><h3>Steps in the system</h3>`+
            `<p class="sl-hint">The event model of this slice. Time runs from left to right.</p>`+
            `<div class="sl-diagram"></div>`+
            `<div class="sl-details">${detailsHTML(slice)}</div>`+
          `</section>`+
        `</article>`+
      `</div>`+
      `<nav class="sl-nav" aria-label="Slides">${navLink("prev", prev.href, prev.label, prev.title)}`+
        `${navLink("next", next.href, next.label, next.title)}</nav>`;

    host.querySelector(".sl-diagram").appendChild(EMC.sliceTimeline(slice));
    host.querySelectorAll(".sl-shot img").forEach(image=>{
      const fail = ()=>image.closest(".sl-shot").classList.add("failed");
      if(image.complete && image.naturalWidth === 0) fail();
      image.addEventListener("error", fail);
    });
    const fullscreen = host.querySelector(".sl-fullscreen");
    if(fullscreen) fullscreen.addEventListener("click", ()=>{
      if(document.fullscreenElement) document.exitFullscreen();
      else document.documentElement.requestFullscreen().catch(()=>{});
    });
    host.querySelector(".sl-stage").scrollTop = 0;
    current = {prevHref:prev.href, nextHref:next.href, firstHref:nav.slideHref(1), lastHref:nav.slideHref(slices.length), go:nav.go};
  };

  // Slide links replace the current history entry, so browser Back leaves the slides.
  host.addEventListener("click", e=>{
    const link = e.target.closest('a[href^="#"]');
    if(!link || !current || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    current.go(link.getAttribute("href"));
  });

  document.addEventListener("keydown", e=>{
    if(!current || document.body.dataset.view !== "slides") return;
    if(e.ctrlKey || e.metaKey || e.altKey) return;
    const target = e.target;
    if(target && (/^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName) || target.isContentEditable)) return;
    if(document.querySelector("details.popover[open]")) return;
    const href = {ArrowRight:current.nextHref, PageDown:current.nextHref, ArrowLeft:current.prevHref, PageUp:current.prevHref,
      Home:current.firstHref, End:current.lastHref}[e.key];
    if(!href) return;
    e.preventDefault();
    current.go(href);
  });

  // Swipe left or right on touch screens.
  let start = null;
  host.addEventListener("pointerdown", e=>{ if(e.pointerType === "touch") start = {x:e.clientX, y:e.clientY}; });
  host.addEventListener("pointerup", e=>{
    if(!start || !current) return;
    const dx = e.clientX - start.x, dy = e.clientY - start.y;
    start = null;
    if(Math.abs(dx) < 60 || Math.abs(dx) < Math.abs(dy) * 2) return;
    const href = dx < 0 ? current.nextHref : current.prevHref;
    if(href) current.go(href);
  });

  document.addEventListener("fullscreenchange", ()=>{
    const button = host.querySelector(".sl-fullscreen");
    if(button) button.textContent = document.fullscreenElement ? "Exit full screen" : "Full screen";
  });
})();
