/* Compact view: Model-view rows with ONE events row whose cells stack bounded-context boxes.
   Depends on the shared EMC namespace from viewer.js. */
(function(){
  let built = false;
  let board = null, wires = null;
  let pathEls = [];
  const nodeById = new Map();      // data-id -> card / sticky node
  const creatorOf = {};            // event id -> creator node id (ext__… / agg__…)
  const visualEdges = [];          // [from, to]: the model's own edges (command → event → read model)

  const CARD_WIDTH = 176;
  const ACTOR_WIDTH = 152;
  const STAGE_GAP = 20;
  const CELL_PADDING = 28;

  const BAND = {screen:"screens", screen_image:"screens", command:"domain", readmodel:"domain", processor:"processors", table:"domain", event:"events"};
  const BANDS = [
    {key:"screens",    name:"Screens",    sub:"interfaces"},
    {key:"processors", name:"Processors", sub:"automation"},
    {key:"domain",     name:"Model",      sub:"commands & views"},
  ];

  const CONTEXT_COLORS = [
    {fill:"rgba(112,176,214,0.20)", line:"#3d87b0"},
    {fill:"rgba(126,184,106,0.20)", line:"#5a8f45"},
    {fill:"rgba(214,168,74,0.22)", line:"#b1842e"},
    {fill:"rgba(150,140,210,0.20)", line:"#6d62a8"},
    {fill:"rgba(90,176,168,0.20)", line:"#2f8f88"},
    {fill:"rgba(214,122,74,0.20)", line:"#b85a2e"},
    {fill:"rgba(120,150,200,0.20)", line:"#4d6f9e"},
    {fill:"rgba(168,186,92,0.22)", line:"#7a8c38"},
  ];
  const EXTERNAL_COLOR = {fill:"rgba(239,159,190,0.24)", line:"#ce7395"};
  function contextColor(i){
    if(i < CONTEXT_COLORS.length) return CONTEXT_COLORS[i];
    const hue = (i * 47) % 300;
    return {fill:`hsla(${hue},55%,62%,0.22)`, line:`hsl(${hue},45%,36%)`};
  }

  const LOCK_SVG = '<svg class="lock" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect width="18" height="11" x="3" y="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>';
  const SVG_NS = "http://www.w3.org/2000/svg";

  const fieldRow = f => {
    const badges = [];
    if(f.id) badges.push('<i class="id">id</i>');
    if(f.pii) badges.push('<i class="pii">pii</i>');
    return `<div class="field"><span class="fn">${EMC.esc(f.name)}</span><span class="fty">${EMC.esc(f.type)}</span><span class="fb">${badges.join("")}</span></div>`;
  };

  function screenActors(slice){
    const firstScreenByActor = new Map();
    slice.elements.filter(e=>e.kind==="screen"&&e.actor).forEach(screen=>{
      const current = firstScreenByActor.get(screen.actor);
      if(!current || screen.stage<current.stage) firstScreenByActor.set(screen.actor,screen);
    });
    return firstScreenByActor;
  }
  function stageWidths(slice){
    const stages = Math.max(2, slice.stageCount||1);
    const widths = Array(stages).fill(CARD_WIDTH);
    screenActors(slice).forEach(screen=>{
      const stage = Math.min(screen.stage||0,stages-1);
      widths[stage] = Math.max(widths[stage],ACTOR_WIDTH+12+CARD_WIDTH);
    });
    return widths;
  }
  function stageTemplate(slice){
    return stageWidths(slice).map(width=>width+"px").join(" ");
  }

  const BOX_GAP = 12, BOX_CHROME = 28, EDGE_MARGIN = 20;
  const ctxRank = ctx => {
    if(ctx === "__unmapped") return Infinity;
    const ix = Object.keys(EMC.MODEL.contexts).indexOf(ctx);
    return ix < 0 ? Infinity : ix;
  };
  const planCache = new Map();
  // Horizontal placement of a slice's context boxes. Rules: a box sits to the right of the
  // command that emits its events, and to the left of the read model its events feed.
  // Band cells are shifted by `lead` so a read model can sit right of the given event's box.
  function slicePlan(slice){
    if(planCache.has(slice)) return planCache.get(slice);
    const widths = stageWidths(slice);
    const stages = widths.length;
    const x0 = k => widths.slice(0,k).reduce((t,w)=>t+w,0) + k*STAGE_GAP;
    const stageOf = e => Math.min(e.stage||0, stages-1);
    const byId = new Map(slice.elements.map(e=>[e.id,e]));
    const events = slice.elements.map((e,ix)=>({e,ix})).filter(x=>x.e.kind==="event");
    const boxes = new Map();
    events.forEach(({e})=>{
      const ctx = EMC.eventCtx(e);
      if(!boxes.has(ctx)) boxes.set(ctx, {ctx, events:[], width:0, given:false, minStage:Infinity, cmds:[], rms:[]});
      const box = boxes.get(ctx);
      box.width = Math.max(box.width, CARD_WIDTH + BOX_CHROME);
      box.events.push(e);
      box.given = box.given || !!e.given;
      box.minStage = Math.min(box.minStage, e.stage||0);
      EMC.EDGES.forEach(([a,b])=>{
        if(b===e.id && byId.get(a) && byId.get(a).kind==="command") box.cmds.push(byId.get(a));
        if(a===e.id && byId.get(b) && byId.get(b).kind==="readmodel") box.rms.push(byId.get(b));
      });
    });
    const order = [...boxes.values()].sort((a,b)=>
      ((a.given?0:1)-(b.given?0:1)) || (a.minStage-b.minStage) || (ctxRank(a.ctx)-ctxRank(b.ctx)));
    let lead = CELL_PADDING/2;
    for(let pass=0; pass<2; pass++){
      let cursor = CELL_PADDING/2;
      order.forEach(box=>{
        let minLeft = CELL_PADDING/2;
        box.cmds.forEach(cmd=>{
          const k = stageOf(cmd);
          minLeft = Math.max(minLeft, lead + x0(k) + widths[k] + EDGE_MARGIN);
        });
        box.left = Math.max(cursor, minLeft);
        box.margin = box.left - cursor;
        cursor = box.left + box.width + BOX_GAP;
        box.rms.forEach(rm=>{
          lead = Math.max(lead, box.left + box.width + EDGE_MARGIN - x0(stageOf(rm)));
        });
      });
    }
    const stageDemand = lead + x0(stages) - STAGE_GAP + CELL_PADDING/2;
    const eventDemand = order.length ? order[order.length-1].left + order[order.length-1].width + CELL_PADDING/2 : 0;
    const plan = {widths, lead, order, boxes, total: Math.max(480, stageDemand, eventDemand)};
    planCache.set(slice, plan);
    return plan;
  }
  function sliceWidth(slice){ return slicePlan(slice).total; }

  const SVG_ATTRS = 'class="ic" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"';
  // globe: an outside system
  const EXT_ICON = `<svg ${SVG_ATTRS}><circle cx="12" cy="12" r="9"/><path d="M3 12h18"/><path d="M12 3c2.6 2.6 4 5.6 4 9s-1.4 6.4-4 9c-2.6-2.6-4-5.6-4-9s1.4-6.4 4-9z"/></svg>`;
  // cube: a consistency boundary
  const AGG_ICON = `<svg ${SVG_ATTRS}><path d="M12 2.5 20.5 7v10L12 21.5 3.5 17V7z"/><path d="M3.5 7 12 11.5 20.5 7"/><path d="M12 11.5v10"/></svg>`;

  function cardNode(e){
    const isEvent = e.kind === "event";
    const parts = [];
    parts.push(`<div class="kind"><span class="kdot"></span><span class="kn">${EMC.KIND_LABEL[e.kind]}</span>` +
      (isEvent ? `` : (e.agg ? `<span class="agg">◈ ${EMC.esc(e.agg)}</span>` : (e.ctx ? `<span class="agg">${e.external?"↗ ":""}${EMC.esc(e.ctx)}</span>` : ``))) +
      `</div>`);

    if(e.kind==="processor"){
      parts.push(`<div class="proc-head">${EMC.GEAR_SVG}<span class="ct">${EMC.esc(e.title)}</span></div>`);
    } else {
      parts.push(`<div class="ct">${EMC.esc(e.title)}</div>`);
    }
    if(e.question) parts.push(`<div class="q">“${EMC.esc(e.question)}”</div>`);
    if(e.api) parts.push(`<div class="api">${EMC.esc(e.api)}</div>`);
    if(e.kind==="screen"){
      parts.push(`<div class="wire-frame"></div>`);
    }
    if(e.kind==="screen_image"){
      parts.push(`<div class="image-preview${e.imageUrl?"":" failed"}">`+
        `<img src="${EMC.esc(e.imageUrl||"")}" alt="${EMC.esc(e.title)}" loading="lazy" referrerpolicy="no-referrer">`+
        `<span class="image-preview-fallback">Preview unavailable</span></div>`);
    }
    if(e.given) parts.push(`<div class="tags"><span class="tag">given / upstream</span></div>`);
    else if(e.tags) parts.push(`<div class="tags">${e.tags.map(t=>`<span class="tag">${EMC.esc(t)}</span>`).join("")}</div>`);
    if(e.fields) parts.push(`<div class="fields">${e.fields.map(fieldRow).join("")}</div>`);

    const c = EMC.el("div", "card "+e.kind+(e.external?" external":""), parts.join(""));
    c.dataset.id = e.id;
    c.dataset.slice = EMC.SLICE_OF[e.id];
    if(e.actor) c.dataset.actor = e.actor;
    if(isEvent) c.dataset.ctx = EMC.eventCtx(e);
    else if(e.ctx) c.dataset.ctx = e.ctx;
    const image = c.querySelector(".image-preview img");
    if(image) image.addEventListener("error", ()=>image.parentElement.classList.add("failed"));
    return c;
  }

  function actorCard(actorID, actor, sliceIndex){
    const button = EMC.el("button", "actor-card",
      `<span class="person">${EMC.ACTOR_SVG}</span><span class="actor-copy"><span class="an">${EMC.esc(actor.title)}</span>`+
      `<span class="am">Actor</span></span>${actor.authRequired?LOCK_SVG:""}`);
    button.type = "button";
    button.dataset.actor = actorID;
    button.dataset.slice = sliceIndex;
    button.setAttribute("aria-label", actor.title+(actor.authRequired?", authentication required":""));
    return button;
  }

  function hotspotNote(h){
    const dot = EMC.el("div","hotspot");
    dot.innerHTML = '<span class="hs-mark">?!</span><span class="hs-label">Hotspot</span><span class="hs-q">'+EMC.esc(h.question)+'</span>';
    dot.setAttribute("tabindex","0");
    dot.setAttribute("data-q", h.question + "  ·  ["+h.status+"]");
    return dot;
  }

  /* creator (external-system sticky or aggregate card) for an event, or null */
  function creatorNode(e, ctx){
    const slice = EMC.SLICE_OF[e.id];
    if(e.external || EMC.ctxExternal(ctx)){
      const s = EMC.el("div","ext-sticky",
        `<div class="k">${EXT_ICON}<span>External system</span></div><div class="t">${EMC.esc(EMC.ctxTitle(ctx))}</div>`);
      s.dataset.id = "ext__"+e.id;
      s.dataset.slice = slice;
      s.dataset.ctx = ctx;
      return s;
    }
    if(e.agg){
      const a = EMC.el("div","card aggregate",
        `<div class="kind">${AGG_ICON}<span class="kn">Aggregate</span></div>`+
        `<div class="ct">${EMC.esc(EMC.aggTitle(e.agg))}</div>`);
      a.dataset.id = "agg__"+e.id;
      a.dataset.slice = slice;
      a.dataset.ctx = ctx;
      return a;
    }
    return null;
  }

  function eventsCell(slice, sliceIx, ctxOrder, colors){
    const cell = EMC.el("div","cell band events");
    const events = slice.elements.map((e,ix)=>({e,ix})).filter(x=>x.e.kind==="event");
    let any = false;
    const plan = slicePlan(slice);
    plan.order.forEach(({ctx, width, margin})=>{
      const items = events.filter(x=>EMC.eventCtx(x.e)===ctx)
        .sort((a,b)=>((a.e.stage||0)-(b.e.stage||0)) || (a.ix-b.ix));
      if(!items.length) return;
      any = true;
      const external = EMC.ctxExternal(ctx);
      const color = colors[ctx];
      const box = EMC.el("div","ctx-box"+(external?" external":""));
      box.style.width = width+"px";
      box.style.flex = "0 0 auto";
      if(margin) box.style.marginLeft = margin+"px";
      box.dataset.ctx = ctx;
      box.style.setProperty("--ctx-fill", color.fill);
      box.style.setProperty("--ctx-line", color.line);
      box.appendChild(EMC.el("div","ctx-box-title", EMC.esc(EMC.ctxTitle(ctx)) + (external ? '<span class="ext"> ↗</span>' : "")));
      items.forEach(({e})=>{
        const pair = EMC.el("div","ctx-pair");
        const creator = creatorNode(e, ctx);
        if(creator){
          pair.appendChild(creator);
          creatorOf[e.id] = creator.dataset.id;
        }
        pair.appendChild(cardNode(e));
        box.appendChild(pair);
      });
      cell.appendChild(box);
    });
    if(!any){
      cell.classList.add("lane-empty");
      cell.appendChild(EMC.el("div","lane-empty-mark","—"));
    }
    return cell;
  }

  function buildEdges(){
    const seen = new Set();
    EMC.EDGES.forEach(([a,b])=>{
      const k = a+">"+b;
      if(!seen.has(k)){ seen.add(k); visualEdges.push([a,b]); }
    });
  }

  function render(){
    if(built) return;
    board = document.getElementById("board-compact");
    if(!board) return;
    wires = document.getElementById("wires-compact");
    built = true;

    // clear any previously injected nodes, keep the wire svg
    [...board.children].forEach(n=>{ if(n !== wires) n.remove(); });

    const MODEL = EMC.MODEL;
    board.style.setProperty("--cols", MODEL.slices.length);
    const sliceWidths = MODEL.slices.map(sliceWidth);
    board.style.gridTemplateColumns = `var(--rail-w) ${sliceWidths.map(w=>w+"px").join(" ")}`;

    // context order: contexts owning an event, then "__unmapped"
    const owning = new Set();
    MODEL.slices.forEach(s=>s.elements.forEach(e=>{ if(e.kind==="event") owning.add(EMC.eventCtx(e)); }));
    const ctxOrder = [
      ...Object.keys(MODEL.contexts).filter(c=>owning.has(c)),
      ...(owning.has("__unmapped") ? ["__unmapped"] : []),
    ];
    const colors = {};
    let colorIx = 0;
    ctxOrder.forEach(ctx=>{ colors[ctx] = EMC.ctxExternal(ctx) ? EXTERNAL_COLOR : contextColor(colorIx++); });

    const frag = document.createDocumentFragment();

    // Row 1: slice headers
    frag.appendChild(EMC.el("div","cell rail-corner r-header"));
    MODEL.slices.forEach((s,i)=>{
      const h = EMC.el("div","cell slice-head");
      h.dataset.slice = i;
      h.dataset.nodeId = "slice__"+s.id;
      h.innerHTML =
        `<span class="slice-idx">${String(i+1).padStart(2,"0")}</span>`+
        `<div class="top"><span class="pat" title="${EMC.PATTERN_LABEL[s.type]}">${EMC.PAT_SVG[s.type]||""}</span>`+
        `<span class="ttl">${EMC.esc(s.title)}</span></div>`+
        `<div class="btm"><span class="ptype">${EMC.PATTERN_LABEL[s.type]}</span>`+
        `<span class="status" style="--sc:${EMC.statusVar(s.status||"created")}"><span class="sd"></span>${EMC.STATUS_LABEL[s.status||"created"]}</span></div>`;
      frag.appendChild(h);
    });

    // Rows 3-5: screens / processors / domain
    const sliceHotspots = new Map();
    const placedSliceHotspots = new Set();
    MODEL.hotspots.forEach(h=>{
      const i = MODEL.slices.findIndex(s=>h.onId === "slice__"+s.id);
      if(i < 0) return;
      if(!sliceHotspots.has(i)) sliceHotspots.set(i, []);
      sliceHotspots.get(i).push(h);
    });
    BANDS.forEach(b=>{
      const rail = EMC.el("div","cell rail-lane");
      rail.appendChild(EMC.el("div","txt", `${b.name}<small>${b.sub}</small>`));
      frag.appendChild(rail);

      MODEL.slices.forEach((s,i)=>{
        const cell = EMC.el("div","cell band "+b.key);
        const stages = Math.max(2,s.stageCount||1);
        cell.style.gridTemplateColumns = stageTemplate(s);
        cell.style.paddingLeft = slicePlan(s).lead + "px";
        const byStage = new Map();
        const firstScreenByActor = b.key==="screens" ? screenActors(s) : new Map();
        s.elements.filter(e=>BAND[e.kind]===b.key).forEach(e=>{
          const stage = Math.min(e.stage||0,stages-1);
          if(!byStage.has(stage)) byStage.set(stage,[]);
          byStage.get(stage).push(e);
        });
        const hasSliceHs = b.key==="screens" && sliceHotspots.has(i);
        if(hasSliceHs){
          const row = EMC.el("div","slice-hotspots");
          row.style.gridColumn = "1 / -1";
          row.style.gridRow = "1";
          sliceHotspots.get(i).forEach(h=>{ row.appendChild(hotspotNote(h)); placedSliceHotspots.add(h); });
          cell.appendChild(row);
        }
        [...byStage.entries()].sort((a,c)=>a[0]-c[0]).forEach(([stage,items])=>{
          const stack = EMC.el("div","stage-stack");
          stack.style.gridColumn = (stage+1);
          items.forEach(e=>{
            const card = cardNode(e);
            if(b.key!=="screens" || !e.actor || firstScreenByActor.get(e.actor)!==e){
              stack.appendChild(card);
              return;
            }
            const pair = EMC.el("div","screen-pair");
            const actor = MODEL.actors[e.actor];
            if(actor) pair.appendChild(actorCard(e.actor,actor,i));
            pair.appendChild(card);
            stack.appendChild(pair);
          });
          if(hasSliceHs) stack.style.gridRow = "2";
          cell.appendChild(stack);
        });
        frag.appendChild(cell);
      });
    });

    // Last row: the single events row
    const evRail = EMC.el("div","cell rail-lane");
    evRail.appendChild(EMC.el("div","txt","Events"));
    frag.appendChild(evRail);
    MODEL.slices.forEach((s,i)=>frag.appendChild(eventsCell(s, i, ctxOrder, colors)));

    board.appendChild(frag);

    // hotspots: pin onto visible cards
    MODEL.hotspots.forEach(h=>{
      if(placedSliceHotspots.has(h)) return;
      const target = board.querySelector('.card[data-id="'+h.onId+'"]');
      if(target) target.appendChild(hotspotNote(h));
    });

    board.querySelectorAll(".card[data-id], .ext-sticky[data-id]").forEach(n=>nodeById.set(n.dataset.id, n));
    buildEdges();

    wireInteractions();

    LEGENDS.compact = legendHTML();

    window.applyCompactFilters(EMC.filterState || {statuses:new Set(), context:"__all"});
  }

  /* --------------------------- wires --------------------------- */
  function drawWires(){
    if(!built) return;
    pathEls.forEach(p=>p.remove()); pathEls = [];
    const bw = board.scrollWidth, bh = board.scrollHeight;
    wires.setAttribute("viewBox", `0 0 ${bw} ${bh}`);
    wires.setAttribute("width", bw); wires.setAttribute("height", bh);
    visualEdges.forEach(([a,b])=>{
      const nodeA = nodeById.get(a), nodeB = nodeById.get(b);
      if(!nodeA || !nodeB) return;
      const A = EMC.rectIn(board, nodeA), B = EMC.rectIn(board, nodeB);
      let sx,sy,ex,ey,c1x,c1y,c2x,c2y;
      // command → event leaves the command's bottom and enters the event's left edge;
      // the aggregate / external sticky stacked on top of the event never covers it
      const commandToEvent = nodeA.classList.contains("command") && nodeB.classList.contains("event");
      const horiz = !commandToEvent && Math.abs(B.cx-A.cx) > 16;
      if(commandToEvent){
        sx = A.cx; sy = A.y+A.h;
        ex = B.x;  ey = B.cy;
        const dy = Math.max(28, Math.abs(ey-sy)*0.5), dx = Math.max(24, Math.abs(ex-sx)*0.35);
        c1x = sx; c1y = sy + dy; c2x = ex - dx; c2y = ey;
      } else if(horiz){
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
      const p = document.createElementNS(SVG_NS,"path");
      p.setAttribute("d", `M ${sx} ${sy} C ${c1x} ${c1y} ${c2x} ${c2y} ${ex} ${ey}`);
      p.setAttribute("marker-end","url(#ah-compact)");
      p.dataset.a = a; p.dataset.b = b;
      if(B.cx < A.cx && !commandToEvent) p.classList.add("backward");
      if(nodeA.classList.contains("filtered") || nodeB.classList.contains("filtered")) p.classList.add("dim");
      wires.appendChild(p); pathEls.push(p);
    });
  }
  window.relayoutCompact = drawWires;

  /* --------------------------- hover + click --------------------------- */
  function wireInteractions(){
    const NODE = ".card, .ext-sticky";
    board.addEventListener("mouseover", e=>{
      const node = e.target.closest(NODE); if(!node) return;
      const id = node.dataset.id;
      const hot = new Set([id]);
      pathEls.forEach(p=>{
        if(p.dataset.a===id) hot.add(p.dataset.b);
        else if(p.dataset.b===id) hot.add(p.dataset.a);
      });
      // the aggregate / external-system sticky beside a hot event lights up with it
      [...hot].forEach(h=>{ if(creatorOf[h]) hot.add(creatorOf[h]); });
      Object.entries(creatorOf).forEach(([ev,cr])=>{ if(cr===id) hot.add(ev); });
      board.classList.add("hovering");
      board.querySelectorAll(NODE).forEach(n=>n.classList.toggle("is-hot", hot.has(n.dataset.id)));
      const actor = node.dataset.actor, slice = node.dataset.slice;
      board.querySelectorAll(".actor-card").forEach(button=>
        button.classList.toggle("is-hot", !!actor && button.dataset.actor===actor && button.dataset.slice===slice));
      pathEls.forEach(p=>{
        const isHot = p.dataset.a===id || p.dataset.b===id;
        p.classList.toggle("hot", isHot);
        p.setAttribute("marker-end", isHot ? "url(#ah-compact-hot)" : "url(#ah-compact)");
      });
    });
    board.addEventListener("mouseout", e=>{
      const node = e.target.closest(NODE);
      if(node && e.relatedTarget && node.contains(e.relatedTarget)) return;
      if(e.relatedTarget && e.relatedTarget.closest && e.relatedTarget.closest(".actor-card")) return;
      board.classList.remove("hovering");
      board.querySelectorAll(".card.is-hot, .ext-sticky.is-hot, .actor-card.is-hot").forEach(n=>n.classList.remove("is-hot"));
      pathEls.forEach(p=>{ p.classList.remove("hot"); p.setAttribute("marker-end","url(#ah-compact)"); });
    });
    board.addEventListener("click", e=>{
      if(e.target.closest(".hotspot")) return;
      const node = e.target.closest(".slice-head, .card, .ext-sticky");
      if(node) EMC.openSlice(+node.dataset.slice);
    });
  }

  /* --------------------------- filters --------------------------- */
  window.applyCompactFilters = function(state){
    if(!built) return;
    const MODEL = EMC.MODEL;
    MODEL.slices.forEach((s,i)=>{
      const st = s.status||"created";
      const sliceVisible = state.statuses.size===0 || state.statuses.has(st);
      const head = board.querySelector('.slice-head[data-slice="'+i+'"]');
      if(head) head.classList.toggle("filtered", !sliceVisible);
      board.querySelectorAll('.actor-card[data-slice="'+i+'"]').forEach(actor=>actor.classList.toggle("filtered", !sliceVisible));
      s.elements.forEach(e=>{
        const okCtx = state.context==="__all" || !e.ctx || e.ctx===state.context;
        const filtered = !(sliceVisible && okCtx);
        const c = nodeById.get(e.id);
        if(c) c.classList.toggle("filtered", filtered);
        const creator = creatorOf[e.id] && nodeById.get(creatorOf[e.id]);
        if(creator) creator.classList.toggle("filtered", filtered);
      });
    });
    board.querySelectorAll(".ctx-box").forEach(box=>{
      box.classList.toggle("lane-dim", state.context !== "__all" && box.dataset.ctx !== state.context);
    });
    requestAnimationFrame(drawWires);
  };

  /* --------------------------- legend --------------------------- */
  function legendHTML(){
    const rows = [
      ["Domain event","var(--event-fill)","var(--event-line)"],
      ["External event","var(--external-event-fill)","var(--external-event-line)"],
      ["Command","var(--command-fill)","var(--command-line)"],
      ["Read model","var(--read-fill)","var(--read-line)"],
      ["Screen","var(--screen-fill)","var(--screen-line)"],
      ["Processor","var(--proc-fill)","var(--proc-line)"],
      ["Aggregate","#f6eda0","#d4c663"],
      ["External system",EXTERNAL_COLOR.fill,EXTERNAL_COLOR.line],
    ].map(([l,f,c])=>`<div class="row"><span class="sw" style="--fl:${f};--cl:${c}"></span>${l}</div>`).join("");
    return `<div class="grp"><h4>Elements</h4>${rows}</div>`+
      `<div class="grp"><h4>Bounded contexts</h4><div class="row ctx-note">`+
      `Each bounded context is a dashed rounded box. color1 is the first non-external context; external systems are always pink.`+
      `</div></div>`;
  }

  window.renderCompact = function(){
    if(!built){ render(); }
    if(built) requestAnimationFrame(drawWires);
  };
})();
