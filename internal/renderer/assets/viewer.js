/* =====================================================================
   MODEL — adapted from the validated Event Modeling HCL Specification __SPEC_VERSION__
   typed IR and injected here as
   JSON. The browser only handles layout and interaction.
   ===================================================================== */
const MODEL = __MODEL_JSON__;

/* --------------------------- constants --------------------------- */
const BAND = {screen:"screens", screen_image:"screens", command:"domain", readmodel:"domain", processor:"processors", table:"domain", event:"events"};
const KIND_LABEL = {screen:"Screen", command:"Command", readmodel:"Read Model",
  processor:"Processor", screen_image:"Screen Image", table:"Table", event:"Event"};
const PATTERN_LABEL = {state_change:"state_change", state_view:"state_view",
  automation:"automation", translation:"translation"};
const STATUS_LABEL = {created:"Created", done:"Done", assigned:"Assigned", in_progress:"In progress",
  review:"Review", blocked:"Blocked", planned:"Planned", informational:"Informational"};

const PAT_SVG = {
  state_change:'<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="5" y="2.5" width="14" height="6.5" rx="1.6"/><path d="M12 9.2V15"/><path d="M9 12.5l3 3 3-3"/><rect x="5" y="15.5" width="14" height="6" rx="1.6"/></svg>',
  state_view:'<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M2 12s3.6-6 10-6 10 6 10 6-3.6 6-10 6-10-6-10-6Z"/><circle cx="12" cy="12" r="2.6"/></svg>',
  automation:'<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><circle cx="12" cy="12" r="3"/><path d="M12 2.5v3M12 18.5v3M2.5 12h3M18.5 12h3M5 5l2.1 2.1M16.9 16.9 19 19M19 5l-2.1 2.1M7.1 16.9 5 19"/></svg>',
  translation:'<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path d="M3 7h11M9.5 3.5 13 7l-3.5 3.5"/><path d="M21 17H10M14.5 13.5 11 17l3.5 3.5"/></svg>',
};
const GEAR_SVG = '<svg class="gear" viewBox="0 0 24 24" fill="none" stroke="var(--proc-ink)" stroke-width="1.6"><circle cx="12" cy="12" r="3.1"/><path d="M12 2.2v3.2M12 18.6v3.2M2.2 12h3.2M18.6 12h3.2M4.9 4.9l2.3 2.3M16.8 16.8l2.3 2.3M19.1 4.9l-2.3 2.3M7.2 16.8l-2.3 2.3"/></svg>';
const ACTOR_SVG = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 21a8 8 0 0 0-16 0"/><circle cx="12" cy="7" r="4"/></svg>';
const LOCK_SVG = '<svg class="lock" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect width="18" height="11" x="3" y="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>';

/* --------------------------- helpers --------------------------- */
const $ = (s, r=document) => r.querySelector(s);
const el = (tag, cls, html) => { const n=document.createElement(tag); if(cls)n.className=cls; if(html!=null)n.innerHTML=html; return n; };
const esc = s => String(s).replace(/[&<>"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]));
const fmtEx = v => typeof v === "string" ? '"'+v+'"' : (typeof v === "object" ? JSON.stringify(v).replace(/"/g,'') : String(v));
function statusVar(st){ return "var(--st-"+st+")"; }

// flat index of every element across slices, + which slice it lives in
const ELEMENTS = {};
const SLICE_OF = {};
MODEL.slices.forEach((s, si) => s.elements.forEach(e => { ELEMENTS[e.id]=e; SLICE_OF[e.id]=si; }));

/* ---- bounded-context / aggregate index for the event lanes ---- */
const titleize = s => String(s).replace(/[_-]+/g, " ").replace(/\b\w/g, c => c.toUpperCase());
const CTX_AGGS = {};                       // context id -> [aggregate id], first-seen order
(function(){
  const seen = new Set();
  MODEL.slices.forEach(s => s.elements.forEach(e => {
    if (e.kind !== "event") return;
    const ctx = (e.ctx && MODEL.contexts[e.ctx]) ? e.ctx : "__unmapped";
    const agg = e.agg || "__none";
    const key = ctx + " " + agg;
    if (seen.has(key)) return;
    seen.add(key);
    (CTX_AGGS[ctx] = CTX_AGGS[ctx] || []).push(agg);
  }));
})();
const CTX_ORDER = [
  ...Object.keys(MODEL.contexts).filter(c => CTX_AGGS[c]),
  ...(CTX_AGGS["__unmapped"] ? ["__unmapped"] : []),
];
const ctxTitle = c => c === "__unmapped" ? "Unmapped" : ((MODEL.contexts[c] && MODEL.contexts[c].title) || titleize(c));
const ctxExternal = c => !!(MODEL.contexts[c] && MODEL.contexts[c].external);
const aggTitle = a => a === "__none" ? "No aggregate" : titleize(a);
const eventCtx = e => (e.ctx && MODEL.contexts[e.ctx]) ? e.ctx : "__unmapped";

// directed edges from the canonical typed IR, de-duplicated defensively
const EDGES = [];
(function(){
  const seen = new Set();
  const add = (a,b) => { if(a&&b&&ELEMENTS[a]&&ELEMENTS[b]){ const k=a+">"+b; if(!seen.has(k)){seen.add(k); EDGES.push([a,b]);} } };
  (MODEL.edges||[]).forEach(edge => add(edge.from, edge.to));
})();
const NEIGHBORS = {};
EDGES.forEach(([a,b]) => { (NEIGHBORS[a]=NEIGHBORS[a]||new Set()).add(b); (NEIGHBORS[b]=NEIGHBORS[b]||new Set()).add(a); });

/* --------------------------- shared drawer (used by every view) --------------------------- */
const drawer = $("#drawer"), scrim = $("#scrim");
function refChip(rk, name){
  const cl = rk==="event"?"var(--event-line)":rk==="command"?"var(--command-line)":rk==="readmodel"?"var(--read-line)":"var(--ink-faint)";
  return `<span class="ref"><span class="kd" style="--rc:${cl}"></span>${esc(name)}</span>`;
}
function openSlice(i){
  const s = MODEL.slices[i];
  const secs = [];

  // scenarios
  if(s.scenarios && s.scenarios.length){
    const sc = s.scenarios.map(scn=>{
      const rows = [];
      (scn.given||[]).forEach(g=>{
        rows.push(`<div class="gwt given"><span class="k">Given</span><div class="c">`+
          `<div class="cn">${esc(g.title)}</div>`+ (g.ref?refChip(g.refKind,g.ref):"")+
          (g.examples?`<div class="ex">${esc(JSON.stringify(g.examples))}</div>`:"")+`</div></div>`);
      });
      if(scn.when){
        const w=scn.when;
        rows.push(`<div class="gwt when"><span class="k">When</span><div class="c">`+
          `<div class="cn">${esc(w.title)}</div>`+(w.ref?refChip(w.refKind,w.ref):"")+
          (w.fields && w.fields.length?`<div class="ex">${w.fields.map(esc).join(", ")}</div>`:"")+`</div></div>`);
      }
      (scn.then||[]).forEach(t=>{
        rows.push(`<div class="gwt then${t.error?" err":""}"><span class="k">Then</span><div class="c">`+
          `<div class="cn">${esc(t.title)}</div>`+
          (t.ref?refChip(t.refKind,t.ref):"")+
          (t.error?`<div class="ex err-msg">${esc(t.error)}</div>`:"")+
          (t.emptyList?`<span class="flag">expect_empty_list</span>`:"")+`</div></div>`);
      });
      return `<div class="scenario"><div class="sh">${esc(scn.title)}</div>${rows.join("")}`+
        (scn.comments||[]).map(comment=>`<div class="comment">💬 ${esc(comment)}</div>`).join("")+`</div>`;
    }).join("");
    secs.push(`<div class="sec"><h3>Scenarios · Given / When / Then</h3>${sc}</div>`);
  } else {
    secs.push(`<div class="sec"><h3>Scenarios</h3><div class="empty">No scenarios documented for this slice.</div></div>`);
  }

  // elements
  const elrows = s.elements.map(e=>{
    const cl = `var(--${e.kind==="readmodel"?"read":e.kind==="processor"?"proc":e.kind==="screen"?"screen":e.kind}-line, var(--line))`;
    const nf = e.fields? e.fields.length+" field"+(e.fields.length>1?"s":"") : "—";
    const meta = [KIND_LABEL[e.kind], e.agg?("◈ "+e.agg):null, e.ctx?((e.external?"↗ ":"")+e.ctx):null].filter(Boolean).join(" · ");
    return `<div class="elrow"><span class="ek" style="--cl:${cl}"></span>`+
      `<div class="ei"><div class="en">${esc(e.title)}</div><div class="ei2">${esc(meta)}</div></div>`+
      `<div class="ec">${nf}</div></div>`;
  }).join("");
  secs.push(`<div class="sec"><h3>Elements</h3>${elrows}</div>`);

  const ownerTxt = s.owner ? s.owner.replace(/^bounded_context\./,"") : null;
  drawer.innerHTML =
    `<header><button class="x" aria-label="Close">✕</button>`+
    `<div class="pt">${PATTERN_LABEL[s.type]} · slice ${String(i+1).padStart(2,"0")}</div>`+
    `<h2>${esc(s.title)}</h2>`+
    (s.description?`<div class="desc">${esc(s.description)}</div>`:"")+
    `<div class="row"><span class="status" style="--sc:${statusVar(s.status||"created")}"><span class="sd"></span>${STATUS_LABEL[s.status||"created"]}</span>`+
    (ownerTxt?`<span class="owner-pill">◈ ${esc(ownerTxt)}</span>`:"")+`</div></header>`+
    `<div class="body">${secs.join("")}</div>`;
  drawer.querySelector(".x").onclick = closeDrawer;
  drawer.classList.add("open"); scrim.classList.add("open");
  drawer.setAttribute("aria-hidden","false");
}
function closeDrawer(){ drawer.classList.remove("open"); scrim.classList.remove("open"); drawer.setAttribute("aria-hidden","true"); }
scrim.onclick = closeDrawer;
document.addEventListener("keydown", e=>{ if(e.key==="Escape") closeDrawer(); });

/* --------------------------- shared namespace for the other views --------------------------- */
const EMC = {
  MODEL, ELEMENTS, SLICE_OF, EDGES, NEIGHBORS,
  el, esc, titleize, statusVar, openSlice,
  KIND_LABEL, PAT_SVG, PATTERN_LABEL, STATUS_LABEL, ACTOR_SVG, GEAR_SVG,
  ctxTitle, ctxExternal, aggTitle, eventCtx,
};
// generalized nodeCenterEdges: rect of `node` relative to `container`
EMC.rectIn = (container, node) => {
  const cr = container.getBoundingClientRect(), r = node.getBoundingClientRect();
  const s = cr.width && container.offsetWidth ? cr.width / container.offsetWidth : 1;
  return { x:(r.left-cr.left)/s, y:(r.top-cr.top)/s, w:r.width/s, h:r.height/s,
           cx:(r.left-cr.left+r.width/2)/s, cy:(r.top-cr.top+r.height/2)/s };
};

/* --------------------------- legend content per active view --------------------------- */
const LEGENDS = {};
const LP = $("#legend-panel");

/* --------------------------- build board (Event Model view) --------------------------- */
let builtEventModel = false;
function renderEventModel(){
  if(builtEventModel) return;
  builtEventModel = true;

  const board = $("#board");
  const wires = $("#wires");
  const cols = MODEL.slices.length;
  board.style.setProperty("--cols", cols);

  const CARD_WIDTH = 176;
  const ACTOR_WIDTH = 152;
  const STAGE_GAP = 20;
  const CELL_PADDING = 28;
  function screenActors(slice){
    const firstScreenByActor = new Map();
    slice.elements.filter(e=>e.kind==="screen"&&e.actor).forEach(screen=>{
      const current = firstScreenByActor.get(screen.actor);
      if(!current || screen.stage<current.stage) firstScreenByActor.set(screen.actor,screen);
    });
    return firstScreenByActor;
  }
  function stageTemplate(slice){
    const stages = Math.max(2, slice.stageCount||1);
    const widths = Array(stages).fill(CARD_WIDTH);
    screenActors(slice).forEach(screen=>{
      const stage = Math.min(screen.stage||0,stages-1);
      widths[stage] = Math.max(widths[stage],ACTOR_WIDTH+12+CARD_WIDTH);
    });
    return widths.map(width=>width+"px").join(" ");
  }
  function sliceWidth(slice){
    const widths = stageTemplate(slice).split(" ").map(width=>Number.parseInt(width,10));
    const stageDemand = CELL_PADDING + widths.reduce((total,width)=>total+width,0) + (widths.length-1)*STAGE_GAP;
    const events = slice.elements.filter(e=>e.kind==="event").length;
    const eventDemand = events ? CELL_PADDING + events*CARD_WIDTH + Math.max(0,events-1)*12 : 0;
    return Math.max(480, stageDemand, eventDemand);
  }
  const sliceWidths = MODEL.slices.map(sliceWidth);
  board.style.gridTemplateColumns = `var(--rail-w) ${sliceWidths.map(w=>w+"px").join(" ")}`;

  function fieldRow(f){
    const badges = [];
    if(f.id) badges.push('<i class="id">id</i>');
    if(f.pii) badges.push('<i class="pii">pii</i>');
    return `<div class="field"><span class="fn">${esc(f.name)}</span><span class="fty">${esc(f.type)}</span><span class="fb">${badges.join("")}</span></div>`;
  }

  function cardHTML(e){
    const parts = [];
    parts.push(`<div class="kind"><span class="kdot"></span><span class="kn">${KIND_LABEL[e.kind]}</span>` +
      (e.agg ? `<span class="agg">◈ ${esc(e.agg)}</span>` : (e.ctx ? `<span class="agg">${e.external?"↗ ":""}${esc(e.ctx)}</span>` : ``)) +
      `</div>`);

    if(e.kind==="processor"){
      parts.push(`<div class="proc-head">${GEAR_SVG}<span class="ct">${esc(e.title)}</span></div>`);
    } else {
      parts.push(`<div class="ct">${esc(e.title)}</div>`);
    }
    if(e.question) parts.push(`<div class="q">“${esc(e.question)}”</div>`);
    if(e.api) parts.push(`<div class="api">${esc(e.api)}</div>`);
    if(e.kind==="screen"){
      parts.push(`<div class="wire-frame"></div>`);
    }
    if(e.kind==="screen_image"){
      parts.push(`<div class="image-preview${e.imageUrl?"":" failed"}">`+
        `<img src="${esc(e.imageUrl||"")}" alt="${esc(e.title)}" loading="lazy" referrerpolicy="no-referrer">`+
        `<span class="image-preview-fallback">Preview unavailable</span></div>`);
    }
    if(e.given) parts.push(`<div class="tags"><span class="tag">given / upstream</span></div>`);
    else if(e.tags) parts.push(`<div class="tags">${e.tags.map(t=>`<span class="tag">${esc(t)}</span>`).join("")}</div>`);
    if(e.fields) parts.push(`<div class="fields">${e.fields.map(fieldRow).join("")}</div>`);

    const c = el("div", "card "+e.kind+(e.external?" external":""), parts.join(""));
    c.dataset.id = e.id;
    c.dataset.slice = SLICE_OF[e.id];
    if(e.actor) c.dataset.actor = e.actor;
    if(e.ctx) c.dataset.ctx = e.ctx;
    const image = c.querySelector(".image-preview img");
    if(image) image.addEventListener("error", ()=>image.parentElement.classList.add("failed"));
    return c;
  }

  function actorCard(actorID, actor, sliceIndex){
    const button = el("button", "actor-card",
      `<span class="person">${ACTOR_SVG}</span><span class="actor-copy"><span class="an">${esc(actor.title)}</span>`+
      `<span class="am">Actor</span></span>${actor.authRequired?LOCK_SVG:""}`);
    button.type = "button";
    button.dataset.actor = actorID;
    button.dataset.slice = sliceIndex;
    button.setAttribute("aria-label", actor.title+(actor.authRequired?", authentication required":""));
    return button;
  }

  // --- header rows + band rows, cell by cell in grid order ---
  const frag = document.createDocumentFragment();

  // Row 1: chapter band. Rail corner + one cell per slice, chapters span their ranges.
  const corner1 = el("div","cell rail-corner r-chapter"); frag.appendChild(corner1);
  const sliceIndexById = {}; MODEL.slices.forEach((s,i)=>sliceIndexById[s.id]=i);
  const chapterCells = MODEL.slices.map(()=>null);
  MODEL.chapters.forEach(ch => {
    const idxs = ch.slices.map(id=>sliceIndexById[id]).filter(i=>i!=null).sort((a,b)=>a-b);
    if(!idxs.length) return;
    const start=idxs[0], span=idxs[idxs.length-1]-idxs[0]+1;
    const cell = el("div","cell chap-row-cell");
    cell.style.gridColumn = (start+2)+" / span "+span;
    cell.appendChild(el("div","chapter",
      `<span class="arw">▸</span><span class="nm">${esc(ch.title)}</span><span class="ct">${span} slice${span>1?"s":""}</span>`));
    chapterCells[start] = cell;
  });
  MODEL.slices.forEach((s,i)=>{ if(chapterCells[i]) frag.appendChild(chapterCells[i]); else {
    const gap = el("div","cell chap-row-cell"); gap.style.gridColumn=(i+2)+" / span 1"; frag.appendChild(gap);
  }});

  // Row 2: slice headers
  const corner2 = el("div","cell rail-corner r-header"); frag.appendChild(corner2);
  MODEL.slices.forEach((s,i)=>{
    const h = el("div","cell slice-head");
    h.dataset.slice = i;
    h.dataset.nodeId = "slice__"+s.id;
    h.innerHTML =
      `<span class="slice-idx">${String(i+1).padStart(2,"0")}</span>`+
      `<div class="top"><span class="pat" title="${PATTERN_LABEL[s.type]}">${PAT_SVG[s.type]||""}</span>`+
      `<span class="ttl">${esc(s.title)}</span></div>`+
      `<div class="btm"><span class="ptype">${PATTERN_LABEL[s.type]}</span>`+
      `<span class="status" style="--sc:${statusVar(s.status||"created")}"><span class="sd"></span>${STATUS_LABEL[s.status||"created"]}</span></div>`;
    frag.appendChild(h);
  });

  // Rows 3-6: element swimlanes
  const BANDS = [
    {key:"screens",    name:"Screens",    sub:"interfaces"},
    {key:"processors", name:"Processors", sub:"automation"},
    {key:"domain",     name:"Model",      sub:"commands & views"},
  ];
  const sliceHotspots = new Map();
  const placedSliceHotspots = new Set();
  MODEL.hotspots.forEach(h => {
    const i = MODEL.slices.findIndex(s => h.onId === "slice__"+s.id);
    if(i < 0) return;
    if(!sliceHotspots.has(i)) sliceHotspots.set(i, []);
    sliceHotspots.get(i).push(h);
  });
  BANDS.forEach(b => {
    const rail = el("div","cell rail-lane");
    rail.appendChild(el("div","txt", `${b.name}<small>${b.sub}</small>`));
    frag.appendChild(rail);

    MODEL.slices.forEach((s,i)=>{
      const cell = el("div","cell band "+b.key);
      const stages = Math.max(2,s.stageCount||1);
      cell.style.gridTemplateColumns = stageTemplate(s);
      if(b.key==="events"){
        const items = s.elements.filter(e=>BAND[e.kind]===b.key);
        if(items.length){
          const strip = el("div","event-strip");
          const upstream = el("div","event-group upstream");
          const outcome = el("div","event-group outcome");
          items.forEach(e=>(e.given?upstream:outcome).appendChild(cardHTML(e)));
          if(upstream.childElementCount) strip.appendChild(upstream);
          if(outcome.childElementCount) strip.appendChild(outcome);
          cell.appendChild(strip);
        }
      } else {
        const byStage = new Map();
        const firstScreenByActor = b.key==="screens" ? screenActors(s) : new Map();
        s.elements.filter(e=>BAND[e.kind]===b.key).forEach(e=>{
          const stage = Math.min(e.stage||0,stages-1);
          if(!byStage.has(stage)) byStage.set(stage,[]);
          byStage.get(stage).push(e);
        });
        const hasSliceHs = b.key==="screens" && sliceHotspots.has(i);
        if(hasSliceHs){
          const row = el("div","slice-hotspots");
          row.style.gridColumn = "1 / -1";
          row.style.gridRow = "1";
          sliceHotspots.get(i).forEach(h=>{ row.appendChild(hotspotNote(h)); placedSliceHotspots.add(h); });
          cell.appendChild(row);
        }
        [...byStage.entries()].sort((a,b)=>a[0]-b[0]).forEach(([stage,items])=>{
          const stack = el("div","stage-stack");
          stack.style.gridColumn = (stage+1);
          items.forEach(e=>{
            const card = cardHTML(e);
            if(b.key!=="screens" || !e.actor || firstScreenByActor.get(e.actor)!==e){
              stack.appendChild(card);
              return;
            }
            const pair = el("div","screen-pair");
            const actor = MODEL.actors[e.actor];
            if(actor) pair.appendChild(actorCard(e.actor,actor,i));
            pair.appendChild(card);
            stack.appendChild(pair);
          });
          if(hasSliceHs) stack.style.gridRow = "2";
          cell.appendChild(stack);
        });
      }
      frag.appendChild(cell);
    });
  });

  // Event lanes: one grid row per aggregate, grouped under a bounded-context header.
  function eventLaneCell(slice, sliceIx, ctx, agg, cix, aix){
    const cell = el("div","cell band events agg-band");
    cell.dataset.ctx = ctx; cell.dataset.agg = agg; cell.dataset.cix = cix; cell.dataset.aix = aix;
    if(sliceIx === 0) cell.classList.add("lane-start");
    if(sliceIx === MODEL.slices.length - 1) cell.classList.add("lane-end");
    cell.style.gridTemplateColumns = stageTemplate(slice);
    const items = slice.elements.filter(e => e.kind==="event" && eventCtx(e)===ctx && (e.agg||"__none")===agg);
    if(items.length){
      const strip = el("div","event-strip");
      const upstream = el("div","event-group upstream");
      const outcome = el("div","event-group outcome");
      items.forEach(e => (e.given?upstream:outcome).appendChild(cardHTML(e)));
      if(upstream.childElementCount) strip.appendChild(upstream);
      if(outcome.childElementCount) strip.appendChild(outcome);
      cell.appendChild(strip);
    } else {
      cell.classList.add("lane-empty");
      cell.appendChild(el("div","lane-empty-mark","—"));
    }
    return cell;
  }

  let aggIx = -1;
  CTX_ORDER.forEach((ctx, cix) => {
    const aggs = CTX_AGGS[ctx];

    const headRail = el("div","cell ctx-head-rail");
    headRail.dataset.ctx = ctx; headRail.dataset.cix = cix;
    headRail.innerHTML = `<span class="ctx-rail-tag">${esc(ctxTitle(ctx))}</span>`;
    frag.appendChild(headRail);

    const head = el("div","cell ctx-head");
    head.dataset.ctx = ctx; head.dataset.cix = cix;
    head.style.gridColumn = "2 / -1";
    head.innerHTML =
      `<button class="ctx-toggle" type="button" aria-expanded="true" data-ctx="${esc(ctx)}">`+
        `<span class="ctx-caret" aria-hidden="true">▾</span>`+
        `<span class="ctx-name">${esc(ctxTitle(ctx))}${ctxExternal(ctx)?' <span class="ext">↗</span>':''}</span>`+
        `<span class="ctx-count">${aggs.length} aggregate${aggs.length>1?"s":""}</span>`+
      `</button>`;
    frag.appendChild(head);

    aggs.forEach((agg, ai) => {
      aggIx++;
      const rail = el("div","cell rail-lane agg-lane"+(ai===0?" ctx-start":"")+(ai===aggs.length-1?" ctx-end":""));
      rail.dataset.ctx = ctx; rail.dataset.agg = agg; rail.dataset.cix = cix; rail.dataset.aix = aggIx;
      rail.innerHTML =
        `<div class="agg-rail">`+
          `<span class="ctx-kicker${ai===0?"":" sub"}">${esc(ctxTitle(ctx))}</span>`+
          `<span class="agg-name">◈ ${esc(aggTitle(agg))}</span>`+
        `</div>`;
      frag.appendChild(rail);
      MODEL.slices.forEach((s, si) => frag.appendChild(eventLaneCell(s, si, ctx, agg, cix, aggIx)));
    });
  });

  board.appendChild(frag);

  // --- hotspots: pin onto visible targets; keep the rest in the legend ---
  const UNPINNED_HOTSPOTS = [];
  function hotspotNote(h){
    const dot = el("div","hotspot");
    dot.innerHTML = '<span class="hs-mark">?!</span><span class="hs-label">Hotspot</span><span class="hs-q">'+esc(h.question)+'</span>';
    dot.setAttribute("tabindex","0");
    dot.setAttribute("data-q", h.question + "  ·  ["+h.status+"]");
    return dot;
  }
  MODEL.hotspots.forEach(h => {
    if(placedSliceHotspots.has(h)) return;
    const target = board.querySelector('.card[data-id="'+h.onId+'"]');
    if(!target){ UNPINNED_HOTSPOTS.push(h); return; }
    target.appendChild(hotspotNote(h));
  });

  /* --------------------------- wires --------------------------- */
  function nodeCenterEdges(c){
    const br = board.getBoundingClientRect(), r = c.getBoundingClientRect();
    const s = br.width && board.offsetWidth ? br.width / board.offsetWidth : 1;
    return { x:(r.left-br.left)/s, y:(r.top-br.top)/s, w:r.width/s, h:r.height/s,
             cx:(r.left-br.left+r.width/2)/s, cy:(r.top-br.top+r.height/2)/s, node:c };
  }
  function cardCenterEdges(id){
    const c = board.querySelector('.card[data-id="'+id+'"]');
    return c ? nodeCenterEdges(c) : null;
  }
  let pathEls = [];
  let actorLinkEls = [];
  function drawActorLinks(){
    actorLinkEls.forEach(p=>p.remove()); actorLinkEls=[];
    board.querySelectorAll(".actor-card").forEach(actor=>{
      const A = nodeCenterEdges(actor);
      const screens = [...board.querySelectorAll(".card.screen")].filter(screen=>
        screen.dataset.slice===actor.dataset.slice && screen.dataset.actor===actor.dataset.actor);
      screens.forEach(screen=>{
        const B = cardCenterEdges(screen.dataset.id);
        if(!B) return;
        const leftToRight = B.cx>=A.cx;
        const sx = leftToRight ? A.x+A.w : A.x;
        const ex = leftToRight ? B.x : B.x+B.w;
        const dx = Math.max(28,Math.abs(ex-sx)*.42);
        const p = document.createElementNS("http://www.w3.org/2000/svg","path");
        p.classList.add("actor-screen-link");
        p.setAttribute("d",`M ${sx} ${A.cy} C ${sx+(leftToRight?dx:-dx)} ${A.cy} ${ex-(leftToRight?dx:-dx)} ${B.cy} ${ex} ${B.cy}`);
        p.dataset.actor = actor.dataset.actor;
        p.dataset.slice = actor.dataset.slice;
        wires.appendChild(p); actorLinkEls.push(p);
      });
    });
  }
  function drawWires(){
    pathEls.forEach(p=>p.remove()); pathEls=[];
    actorLinkEls.forEach(p=>p.remove()); actorLinkEls=[];
    const bw = board.scrollWidth, bh = board.scrollHeight;
    wires.setAttribute("viewBox", `0 0 ${bw} ${bh}`);
    wires.setAttribute("width", bw); wires.setAttribute("height", bh);
    EDGES.forEach(([a,b])=>{
      const A = cardCenterEdges(a), B = cardCenterEdges(b);
      if(!A||!B) return;
      let sx,sy,ex,ey,c1x,c1y,c2x,c2y;
      const horiz = Math.abs(B.cx-A.cx) > 16;
      if(horiz){
        const ltr = B.cx >= A.cx;
        sx = ltr ? A.x+A.w : A.x;  sy = A.cy;
        ex = ltr ? B.x : B.x+B.w;  ey = B.cy;
        const dx = Math.max(40, Math.abs(ex-sx)*0.45);
        c1x = sx + (ltr?dx:-dx); c1y = sy; c2x = ex - (ltr?dx:-dx); c2y = ey;
      } else {
        const down = B.cy >= A.cy;
        sx = A.cx; sy = down ? A.y+A.h : A.y;
        ex = B.cx; ey = down ? B.y : B.y+B.h;
        const dy = Math.max(28, Math.abs(ey-sy)*0.5);
        c1x = sx; c1y = sy + (down?dy:-dy); c2x = ex; c2y = ey - (down?dy:-dy);
      }
      const p = document.createElementNS("http://www.w3.org/2000/svg","path");
      p.setAttribute("d", `M ${sx} ${sy} C ${c1x} ${c1y} ${c2x} ${c2y} ${ex} ${ey}`);
      p.setAttribute("marker-end","url(#ah)");
      p.dataset.a=a; p.dataset.b=b;
      if(B.cx<A.cx) p.classList.add("backward");
      const dimmed = A.node.classList.contains("filtered") || B.node.classList.contains("filtered");
      if(dimmed) p.classList.add("dim");
      wires.appendChild(p); pathEls.push(p);
    });
    drawActorLinks();
  }
  window.relayoutEventModel = drawWires;

  /* --------------------------- hover highlight --------------------------- */
  board.addEventListener("mouseover", e=>{
    const card = e.target.closest(".card"); if(!card) return;
    const id = card.dataset.id;
    const hot = new Set([id, ...(NEIGHBORS[id]||[])]);
    board.classList.add("hovering");
    board.querySelectorAll(".card").forEach(c=>c.classList.toggle("is-hot", hot.has(c.dataset.id)));
    const actor = card.dataset.actor;
    const slice = card.dataset.slice;
    board.querySelectorAll(".actor-card").forEach(button=>button.classList.toggle("is-hot", !!actor && button.dataset.actor===actor && button.dataset.slice===slice));
    pathEls.forEach(p=>p.classList.toggle("hot", p.dataset.a===id||p.dataset.b===id));
    actorLinkEls.forEach(p=>p.classList.toggle("hot", !!actor && p.dataset.actor===actor && p.dataset.slice===slice));
    pathEls.forEach(p=>{ if(p.classList.contains("hot")) p.setAttribute("marker-end","url(#ah-hot)"); });
  });
  board.addEventListener("mouseout", e=>{
    if(e.relatedTarget && e.target.closest(".card") && e.target.closest(".card").contains(e.relatedTarget)) return;
    if(e.relatedTarget && e.relatedTarget.closest && e.relatedTarget.closest(".actor-card")) return;
    board.classList.remove("hovering");
    board.querySelectorAll(".card.is-hot").forEach(c=>c.classList.remove("is-hot"));
    board.querySelectorAll(".actor-card.is-hot").forEach(actor=>actor.classList.remove("is-hot"));
    pathEls.forEach(p=>{ p.classList.remove("hot"); p.setAttribute("marker-end","url(#ah)"); });
    actorLinkEls.forEach(p=>p.classList.remove("hot"));
  });

  function highlightActor(button, active){
    const screenIds = new Set([...board.querySelectorAll(".card.screen")]
      .filter(screen=>screen.dataset.slice===button.dataset.slice && screen.dataset.actor===button.dataset.actor)
      .map(screen=>screen.dataset.id));
    board.classList.toggle("hovering", active);
    board.querySelectorAll(".actor-card").forEach(actor=>actor.classList.toggle("is-hot", active && actor===button));
    board.querySelectorAll(".card").forEach(card=>card.classList.toggle("is-hot", active && screenIds.has(card.dataset.id)));
    pathEls.forEach(path=>{
      const hot = active && (screenIds.has(path.dataset.a)||screenIds.has(path.dataset.b));
      path.classList.toggle("hot", hot);
      path.setAttribute("marker-end", hot?"url(#ah-hot)":"url(#ah)");
    });
    actorLinkEls.forEach(path=>path.classList.toggle("hot", active && path.dataset.actor===button.dataset.actor && path.dataset.slice===button.dataset.slice));
  }
  board.querySelectorAll(".actor-card").forEach(actor=>{
    actor.addEventListener("mouseenter",()=>highlightActor(actor,true));
    actor.addEventListener("mouseleave",()=>highlightActor(actor,false));
    actor.addEventListener("focus",()=>highlightActor(actor,true));
    actor.addEventListener("blur",()=>highlightActor(actor,false));
  });

  /* --------------------------- filters --------------------------- */
  const state = EMC.filterState = {chapter:"__all", statuses:new Set(), context:"__all"};

  function applyFilters(){
    MODEL.slices.forEach((s,i)=>{
      const inChapter = state.chapter==="__all" ||
        (MODEL.chapters.find(c=>c.id===state.chapter)?.slices.includes(s.id));
      const st = s.status||"created";
      const okStatus = state.statuses.size===0 || state.statuses.has(st);
      const sliceVisible = inChapter && okStatus;
      board.querySelector('.slice-head[data-slice="'+i+'"]').classList.toggle("filtered", !sliceVisible);
      board.querySelectorAll('.actor-card[data-slice="'+i+'"]').forEach(actor=>actor.classList.toggle("filtered", !sliceVisible));
      s.elements.forEach(e=>{
        const c = board.querySelector('.card[data-id="'+e.id+'"]'); if(!c) return;
        const okCtx = state.context==="__all" || !e.ctx || e.ctx===state.context;
        c.classList.toggle("filtered", !(sliceVisible && okCtx));
      });
    });
    board.querySelectorAll(".cell[data-ctx]").forEach(n => {
      n.classList.toggle("lane-dim", state.context !== "__all" && n.dataset.ctx !== state.context);
    });
    requestAnimationFrame(drawWires);
    window.applyStormingFilters && window.applyStormingFilters(state);
    window.applyCompactFilters && window.applyCompactFilters(state);
  }

  // chapter segmented
  const fChapter = $("#f-chapter");
  [["__all","All"], ...MODEL.chapters.map(c=>[c.id,c.title])].forEach(([v,lab],i)=>{
    const b = el("button","btn"+(v==="__all"?" on":""), esc(lab)); b.dataset.v=v;
    b.onclick=()=>{ state.chapter=v; fChapter.querySelectorAll(".btn").forEach(x=>x.classList.toggle("on",x.dataset.v===v)); applyFilters(); };
    fChapter.appendChild(b);
  });

  // status chips (only statuses present)
  const fStatus = $("#f-status");
  const present = [...new Set(MODEL.slices.map(s=>s.status||"created"))];
  present.forEach(st=>{
    const chip = el("button","chip", `<span class="sw" style="--c:${statusVar(st)}"></span>${STATUS_LABEL[st]}`);
    chip.setAttribute("aria-pressed","true"); chip.dataset.st=st;
    chip.onclick=()=>{
      const on = chip.getAttribute("aria-pressed")==="true";
      // treat as an active-set: click toggles membership; empty set = show all
      if(state.statuses.size===0){ present.forEach(s=>state.statuses.add(s)); }
      if(on){ state.statuses.delete(st); } else { state.statuses.add(st); }
      if(state.statuses.size===present.length) state.statuses.clear();
      fStatus.querySelectorAll(".chip").forEach(c=>{
        const active = state.statuses.size===0 || state.statuses.has(c.dataset.st);
        c.setAttribute("aria-pressed", active?"true":"false");
      });
      applyFilters();
    };
    fStatus.appendChild(chip);
  });

  // context segmented
  const fContext = $("#f-context");
  const ctxs = [["__all","All"], ...Object.entries(MODEL.contexts).map(([id,c])=>[id, c.title+(c.external?" ↗":"")])];
  ctxs.forEach(([v,lab])=>{
    const b = el("button","btn"+(v==="__all"?" on":""), esc(lab)); b.dataset.v=v;
    b.onclick=()=>{ state.context=v; fContext.querySelectorAll(".btn").forEach(x=>x.classList.toggle("on",x.dataset.v===v)); applyFilters(); };
    fContext.appendChild(b);
  });

  /* --------------------------- click to open drawer --------------------------- */
  board.addEventListener("click", e=>{
    if(e.target.closest(".hotspot")) return;
    const sh = e.target.closest(".slice-head");
    if(sh){ openSlice(+sh.dataset.slice); return; }
    const card = e.target.closest(".card");
    if(card){ openSlice(+card.dataset.slice); }
  });

  /* --------------------------- legend --------------------------- */
  const elLeg = [
    ["event","Domain event","var(--event-fill)","var(--event-line)"],
    ["external-event","External event","var(--external-event-fill)","var(--external-event-line)"],
    ["command","Command","var(--command-fill)","var(--command-line)"],
    ["readmodel","Read model","var(--read-fill)","var(--read-line)"],
    ["screen","Screen","var(--screen-fill)","var(--screen-line)"],
    ["processor","Processor","var(--proc-fill)","var(--proc-line)"],
    ["hotspot","Hotspot","var(--hot-sticky-fill)","var(--hot-sticky-line)"],
  ].map(([k,l,f,c])=>`<div class="row"><span class="sw" style="--fl:${f};--cl:${c}"></span>${l}</div>`).join("");
  const patLeg = Object.entries(PATTERN_LABEL).map(([k,l])=>`<div class="row"><span class="pg">${PAT_SVG[k]}</span>${l}</div>`).join("");
  const stLeg = Object.entries(STATUS_LABEL).map(([k,l])=>`<div class="row"><span class="sd" style="background:${statusVar(k)}"></span>${l}</div>`).join("");
  const hotspotLeg = UNPINNED_HOTSPOTS.map(h=>`<div class="row"><span class="sw" style="--fl:var(--hot-sticky-fill);--cl:var(--hot-sticky-line)"></span>`+
    `<span>${esc(h.question)}${h.target?` · <span class="mono">${esc(h.target)}</span>`:""}</span></div>`).join("");
  const placedActors = new Set(MODEL.slices.flatMap(slice=>slice.elements.map(element=>element.actor).filter(Boolean)));
  const actorLeg = Object.entries(MODEL.actors).map(([id,actor])=>`<div class="row"><span class="sw" style="--fl:#8FE3D8;--cl:#5DBFB3"></span>`+
    `<span>${esc(actor.title)}${placedActors.has(id)?"":` · <span class="mono">unassigned</span>`}</span></div>`).join("");
  LEGENDS.model =
    `<div class="grp"><h4>Elements</h4>${elLeg}</div>`+
    (actorLeg?`<div class="grp"><h4>Actors</h4>${actorLeg}</div>`:"")+
    `<div class="grp"><h4>Patterns (slice types)</h4>${patLeg}</div>`+
    `<div class="grp"><h4>Slice status</h4>${stLeg}</div>`+
    (hotspotLeg?`<div class="grp"><h4>Other hotspots</h4>${hotspotLeg}</div>`:"");

  applyFilters();      // paints filters + first wire pass
}

/* --------------------------- meta (view-independent chrome) --------------------------- */
$("#m-title").textContent = MODEL.title;
$("#m-version").textContent = MODEL.version;
const counts = {slices:MODEL.slices.length, actors:Object.keys(MODEL.actors).length};
["command","event","readmodel","processor","screen"].forEach(k=>counts[k]=0);
Object.values(ELEMENTS).forEach(e=>{ if(counts[e.kind]!=null) counts[e.kind]++; });
const statBits = [["slices","Slices"],["actors","Actors"],["event","Events"],["command","Commands"],["readmodel","Read models"],["processor","Processors"]];
$("#m-stats").innerHTML = statBits.map(([k,lab])=>`<div class="stat"><span class="n">${counts[k]}</span><span class="k">${lab}</span></div>`).join("");

/* --------------------------- theme --------------------------- */
const THEMES = [["auto","◐","Auto"],["light","☀","Light"],["dark","☾","Dark"]];
let themeIx = 0;
try{ const saved=localStorage.getItem("emc-theme"); if(saved){ themeIx=THEMES.findIndex(t=>t[0]===saved); if(themeIx<0)themeIx=0; } }catch(_){}
function relayoutAll(){
  requestAnimationFrame(()=>{
    window.relayoutEventModel && window.relayoutEventModel();
    window.relayoutEventStorming && window.relayoutEventStorming();
    window.relayoutContextMap && window.relayoutContextMap();
    window.relayoutCompact && window.relayoutCompact();
  });
}
function applyTheme(){
  const [v,gl,tx]=THEMES[themeIx];
  if(v==="auto") document.documentElement.removeAttribute("data-theme");
  else document.documentElement.setAttribute("data-theme", v);
  $("#theme-gl").textContent=gl; $("#theme-tx").textContent=tx;
  try{ localStorage.setItem("emc-theme", v); }catch(_){}
  relayoutAll();
}
$("#t-theme").onclick=()=>{ themeIx=(themeIx+1)%THEMES.length; applyTheme(); };
applyTheme();

/* --------------------------- zoom --------------------------- */
const ZOOM_MIN = 0.25, ZOOM_MAX = 2, ZOOM_STEPS = [0.25,0.33,0.5,0.67,0.8,0.9,1,1.1,1.25,1.5,1.75,2];
const zoomByView = {model:1, storming:1, compact:1};
const fZoomEl = $("#f-zoom");
function zoomBoard(){
  const v = document.body.dataset.view;
  return v === "model" ? boardModel : v === "storming" ? boardES : v === "compact" ? boardCompact : null;
}
function updateZoomLabel(){
  const v = document.body.dataset.view;
  $("#z-reset").textContent = Math.round((zoomByView[v] || 1) * 100) + "%";
}
function scrollNoSmooth(sc, fn){
  const prev = sc.style.scrollBehavior;
  sc.style.scrollBehavior = "auto";
  fn();
  sc.style.scrollBehavior = prev;
}
function setZoom(z, anchor){
  const board = zoomBoard();
  if(!board) return;
  const v = document.body.dataset.view;
  z = Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, z));
  const old = zoomByView[v];
  if(z === old){ updateZoomLabel(); return; }
  const sc = document.querySelector(".canvas-scroll");
  const sr = sc.getBoundingClientRect(), br = board.getBoundingClientRect();
  if(!anchor) anchor = {x:sr.left + sr.width/2, y:sr.top + sr.height/2};
  const px = (anchor.x - br.left) / old, py = (anchor.y - br.top) / old;
  board.style.zoom = z === 1 ? "" : String(z);
  zoomByView[v] = z;
  const br2 = board.getBoundingClientRect();
  scrollNoSmooth(sc, ()=>{
    sc.scrollLeft += (br2.left + px*z) - anchor.x;
    sc.scrollTop += (br2.top + py*z) - anchor.y;
  });
  updateZoomLabel();
  relayoutAll();
}
function zoomIn(){
  const cur = zoomByView[document.body.dataset.view];
  const next = ZOOM_STEPS.find(s => s > cur + 0.001);
  if(next !== undefined) setZoom(next);
}
function zoomOut(){
  const cur = zoomByView[document.body.dataset.view];
  const prev = [...ZOOM_STEPS].reverse().find(s => s < cur - 0.001);
  if(prev !== undefined) setZoom(prev);
}
function zoomFit(){
  const board = zoomBoard();
  if(!board) return;
  const sc = document.querySelector(".canvas-scroll");
  const cs = getComputedStyle(sc);
  const avail = sc.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight);
  setZoom(Math.min(1, Math.max(ZOOM_MIN, avail / board.scrollWidth)));
  scrollNoSmooth(sc, ()=>{ sc.scrollLeft = 0; });
}
$("#z-out").onclick = zoomOut;
$("#z-in").onclick = zoomIn;
$("#z-reset").onclick = () => setZoom(1);
$("#z-fit").onclick = zoomFit;
document.querySelector(".canvas-scroll").addEventListener("wheel", e=>{
  if(!(e.ctrlKey || e.metaKey)) return;
  if(!zoomBoard()) return;
  e.preventDefault();
  const f = Math.exp(-e.deltaY * (e.deltaMode === 1 ? 0.05 : 0.0015));
  setZoom(zoomByView[document.body.dataset.view] * f, {x:e.clientX, y:e.clientY});
}, {passive:false});
document.addEventListener("keydown", e=>{
  if(e.ctrlKey || e.metaKey || e.altKey) return;
  const t = e.target;
  if(t && (/^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName) || t.isContentEditable)) return;
  if($("#drawer").getAttribute("aria-hidden") !== "true") return;
  if(!zoomBoard()) return;
  if(e.key === "+" || e.key === "=") zoomIn();
  else if(e.key === "-" || e.key === "_") zoomOut();
  else if(e.key === "0") setZoom(1);
  else if(e.key === "f" || e.key === "F") zoomFit();
  else return;
  e.preventDefault();
});

/* --------------------------- view switcher --------------------------- */
const boardModel = $("#board"), boardES = $("#board-es"), boardCM = $("#board-cm"), boardCompact = $("#board-compact");
const fChapterEl = $("#f-chapter"), fStatusEl = $("#f-status"), fContextEl = $("#f-context"), fFieldsEl = $("#f-fields-switch");
const fieldPrefs = {model:true, storming:false, compact:true};

$("#t-fields").addEventListener("change", e=>{
  const view = document.body.dataset.view;
  if(view !== "model" && view !== "storming" && view !== "compact") return;
  fieldPrefs[view] = e.target.checked;
  const board = view === "model" ? boardModel : (view === "storming" ? boardES : boardCompact);
  board.classList.toggle("show-fields", e.target.checked);
  relayoutAll();
});

function setFiltersVisible(visible){
  [fChapterEl, fStatusEl, fContextEl, fFieldsEl].forEach(node=>{ if(node) node.style.display = visible?"":"none"; });
}

function setView(name){
  document.body.dataset.view = name;
  boardModel.hidden = name !== "model";
  boardES.hidden = name !== "storming";
  boardCM.hidden = name !== "contextmap";
  boardCompact.hidden = name !== "compact";
  setFiltersVisible(name !== "contextmap");
  fZoomEl.style.display = (name === "model" || name === "storming" || name === "compact") ? "" : "none";
  updateZoomLabel();
  if(name === "model" || name === "storming" || name === "compact"){
    const showFields = fieldPrefs[name];
    $("#t-fields").checked = showFields;
    const board = name === "model" ? boardModel : (name === "storming" ? boardES : boardCompact);
    board.classList.toggle("show-fields", showFields);
  }
  if(name === "model") renderEventModel();
  else if(name === "storming") window.renderEventStorming && window.renderEventStorming();
  else if(name === "contextmap") window.renderContextMap && window.renderContextMap();
  else if(name === "compact") window.renderCompact && window.renderCompact();
  LP.innerHTML = LEGENDS[name] || `<div class="empty">No legend available for this view.</div>`;
  relayoutAll();
}

const fView = $("#f-view");
[["model","Model"],["compact","Compact"],["storming","Storming"],["contextmap","Context Map"]].forEach(([v,lab])=>{
  const b = el("button","btn"+(v==="model"?" on":""), esc(lab)); b.dataset.v=v;
  b.onclick=()=>{ fView.querySelectorAll(".btn").forEach(x=>x.classList.toggle("on",x.dataset.v===v)); setView(v); };
  fView.appendChild(b);
});

/* --------------------------- go --------------------------- */
window.addEventListener("resize", relayoutAll);
setView("model");
setTimeout(relayoutAll, 60);   // after fonts/layout settle
