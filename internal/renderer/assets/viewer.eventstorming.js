/* Event Storming / Big-Picture view. Depends only on the shared EMC namespace. */
(function(){
  let built = false;

  const priority = {screen:0, screen_image:0, command:1, event:2, processor:3, readmodel:4, table:4};
  const fieldRow = f => {
    const badges = [];
    if(f.id) badges.push('<i class="id">id</i>');
    if(f.pii) badges.push('<i class="pii">pii</i>');
    return `<div class="field"><span class="fn">${EMC.esc(f.name)}</span><span class="fty">${EMC.esc(f.type)}</span><span class="fb">${badges.join("")}</span></div>`;
  };

  function processorLabel(sliceType){
    if(sliceType === "automation") return "Automation";
    if(sliceType === "translation") return "Translation";
    return EMC.KIND_LABEL.processor;
  }

  function noteFor(element, sliceIndex){
    const slice = EMC.MODEL.slices[sliceIndex];
    const parts = [];
    const kind = element.kind === "processor" ? processorLabel(slice.type) :
      (EMC.KIND_LABEL[element.kind] || EMC.titleize(element.kind));
    parts.push(`<div class="es-note-kind">${EMC.esc(kind)}</div>`);
    parts.push(`<div class="es-note-title">${EMC.esc(element.title)}</div>`);
    if(element.question) parts.push(`<div class="q es-note-detail">“${EMC.esc(element.question)}”</div>`);
    if(element.api) parts.push(`<div class="es-api es-note-detail">${EMC.esc(element.api)}</div>`);
    if(element.kind === "screen") parts.push('<div class="es-wire-frame es-note-detail"></div>');
    if(element.kind === "screen_image"){
      parts.push(`<div class="image-preview es-note-detail${element.imageUrl?"":" failed"}"><img src="${EMC.esc(element.imageUrl||"")}" alt="${EMC.esc(element.title)}" loading="lazy" referrerpolicy="no-referrer"><span class="image-preview-fallback">Preview unavailable</span></div>`);
    }
    if(element.given) parts.push('<div class="tags es-note-detail"><span class="tag">given / upstream</span></div>');
    else if(element.tags && element.tags.length) parts.push(`<div class="tags es-note-detail">${element.tags.map(tag=>`<span class="tag">${EMC.esc(tag)}</span>`).join("")}</div>`);
    if(element.fields && element.fields.length) parts.push(`<div class="es-fields es-note-detail">${element.fields.map(fieldRow).join("")}</div>`);

    const note = EMC.el("div", `es-note es-${element.kind}${element.external?" external":""}`, parts.join(""));
    note.dataset.id = element.id;
    note.dataset.slice = sliceIndex;
    if(element.actor) note.dataset.actor = element.actor;
    if(element.ctx) note.dataset.ctx = element.ctx;
    const image = note.querySelector(".image-preview img");
    if(image) image.addEventListener("error", ()=>image.parentElement.classList.add("failed"));
    return note;
  }

  function aggregateFor(element, sliceIndex){
    const note = EMC.el("div", "es-note es-aggregate-note",
      `<div class="es-note-kind">Constraint / aggregate</div>`+
      `<div class="es-note-title">${EMC.esc(EMC.aggTitle(element.agg))}</div>`);
    note.dataset.slice = sliceIndex;
    note.dataset.relatedId = element.id;
    note.dataset.for = element.id;
    if(element.ctx) note.dataset.ctx = element.ctx;
    return note;
  }

  function actorNoteFor(actorID, element, sliceIndex){
    const actor = EMC.MODEL.actors[actorID];
    const title = actor ? actor.title : EMC.titleize(actorID);
    const note = EMC.el("div", "es-note es-actor-note",
      `<div class="es-note-kind">Actor</div>`+
      `<div class="es-note-title">${EMC.ACTOR_SVG}<span>${EMC.esc(title)}</span></div>`);
    note.dataset.slice = sliceIndex;
    note.dataset.relatedId = element.id;
    note.dataset.for = element.id;
    if(element.ctx) note.dataset.ctx = element.ctx;
    return note;
  }

  // Lays out one slice's notes on a small per-slice grid: columns follow the
  // element's local causal stage. Two kinds of sidecar notes decorate a real
  // node without affecting its own placement: a command's aggregate note
  // (half a column after the command) and a screen's actor note (half a
  // column before the screen when the screen precedes a command, or half a
  // column after it when the screen follows a read model). Rows ("lanes")
  // keep a chain aligned with its source and give every additional branch
  // (e.g. a command triggering more than one event) its own lane stacked
  // below the first; a sidecar always inherits its anchor's lane when free.
  function layoutSliceNotes(slice, sliceIndex, flow){
    const elements = slice.elements;
    const byId = new Map(elements.map(element=>[element.id, element]));
    const incoming = new Map(elements.map(element=>[element.id, []]));
    const outgoing = new Map(elements.map(element=>[element.id, []]));
    EMC.EDGES.forEach(([fromId, toId]) => {
      if(byId.has(fromId) && byId.has(toId)){
        incoming.get(toId).push(fromId);
        outgoing.get(fromId).push(toId);
      }
    });

    const realNodes = elements.map(element => ({id:element.id, element, column:element.stage||0}));
    realNodes.sort((a,b)=> a.column-b.column || (priority[a.element.kind]??9)-(priority[b.element.kind]??9));

    const laneOf = new Map();
    const occupied = new Map();
    let nextLane = 0;
    const isFree = (column, lane) => !(occupied.get(column) || new Set()).has(lane);
    const occupy = (column, lane) => {
      if(!occupied.has(column)) occupied.set(column, new Set());
      occupied.get(column).add(lane);
    };
    const assignLane = (column, preferredLanes) => {
      let lane = preferredLanes.find(candidate => candidate != null && isFree(column, candidate));
      if(lane == null) lane = nextLane++;
      occupy(column, lane);
      return lane;
    };

    realNodes.forEach(node => {
      const predecessorLanes = [...new Set((incoming.get(node.id) || []).map(id=>laneOf.get(id)).filter(l=>l != null))].sort((a,b)=>a-b);
      laneOf.set(node.id, assignLane(node.column, predecessorLanes));
    });

    const sidecars = [];
    elements.forEach(element => {
      if(element.kind === "command" && element.agg){
        sidecars.push({kind:"aggregate", anchor:element, column:(element.stage||0)+0.5});
      }
      if((element.kind === "screen" || element.kind === "screen_image") && element.actor){
        const followsCommand = (outgoing.get(element.id) || []).some(id=>byId.get(id)?.kind === "command");
        const followsReadmodel = (incoming.get(element.id) || []).some(id => {
          const kind = byId.get(id)?.kind;
          return kind === "readmodel" || kind === "table";
        });
        const side = followsCommand ? "left" : followsReadmodel ? "right" : (slice.type === "state_view" ? "right" : "left");
        sidecars.push({kind:"actor", anchor:element, actorID:element.actor, column:(element.stage||0)+(side === "left" ? -0.5 : 0.5)});
      }
    });
    sidecars.forEach(sidecar => {
      const anchorLane = laneOf.get(sidecar.anchor.id);
      sidecar.lane = assignLane(sidecar.column, anchorLane != null ? [anchorLane] : []);
    });

    const columns = [...new Set([...realNodes.map(node=>node.column), ...sidecars.map(sidecar=>sidecar.column)])].sort((a,b)=>a-b);
    const trackOf = new Map(columns.map((column,index)=>[column, index+1]));

    const place = (note, column, lane) => {
      const track = trackOf.get(column);
      note.style.gridColumn = String(track);
      note.style.gridRow = String(lane+1);
      if(track > 1) note.style.marginLeft = "-3px";
      flow.appendChild(note);
    };

    realNodes.forEach(node => place(noteFor(node.element, sliceIndex), node.column, laneOf.get(node.id)));
    sidecars.forEach(sidecar => {
      const note = sidecar.kind === "aggregate" ? aggregateFor(sidecar.anchor, sliceIndex) : actorNoteFor(sidecar.actorID, sidecar.anchor, sliceIndex);
      place(note, sidecar.column, sidecar.lane);
    });
  }


  function buildStormingGraph(){
    const count = EMC.MODEL.slices.length;
    const outgoing = Array.from({length:count}, ()=>new Set());
    const undirected = Array.from({length:count}, ()=>new Set());
    EMC.EDGES.forEach(([fromID, toID]) => {
      const from = EMC.SLICE_OF[fromID], to = EMC.SLICE_OF[toID];
      if(!Number.isInteger(from) || !Number.isInteger(to) || from === to) return;
      outgoing[from].add(to);
      undirected[from].add(to);
      undirected[to].add(from);
    });

    const flowBySlice = Array(count).fill(-1);
    let flow = 0;
    for(let start=0; start<count; start++){
      if(flowBySlice[start] >= 0) continue;
      const pending = [start];
      flowBySlice[start] = flow;
      while(pending.length){
        const current = pending.pop();
        [...undirected[current]].sort((a,b)=>b-a).forEach(next => {
          if(flowBySlice[next] >= 0) return;
          flowBySlice[next] = flow;
          pending.push(next);
        });
      }
      flow++;
    }

    let nextIndex = 0;
    const indices = Array(count).fill(-1), low = Array(count).fill(0);
    const stack = [], onStack = Array(count).fill(false);
    const sccOf = Array(count).fill(-1), sccMembers = [];
    function connect(node){
      indices[node] = low[node] = nextIndex++;
      stack.push(node);
      onStack[node] = true;
      [...outgoing[node]].sort((a,b)=>a-b).forEach(next => {
        if(indices[next] < 0){
          connect(next);
          low[node] = Math.min(low[node], low[next]);
        } else if(onStack[next]){
          low[node] = Math.min(low[node], indices[next]);
        }
      });
      if(low[node] !== indices[node]) return;
      const members = [];
      while(stack.length){
        const member = stack.pop();
        onStack[member] = false;
        sccOf[member] = sccMembers.length;
        members.push(member);
        if(member === node) break;
      }
      members.sort((a,b)=>a-b);
      sccMembers.push(members);
    }
    for(let node=0; node<count; node++) if(indices[node] < 0) connect(node);

    const sccOutgoing = sccMembers.map(()=>new Set());
    const indegree = sccMembers.map(()=>0);
    outgoing.forEach((targets, from) => targets.forEach(to => {
      const sourceSCC = sccOf[from], targetSCC = sccOf[to];
      if(sourceSCC === targetSCC || sccOutgoing[sourceSCC].has(targetSCC)) return;
      sccOutgoing[sourceSCC].add(targetSCC);
      indegree[targetSCC]++;
    }));
    const ready = indegree.map((degree, scc)=>degree === 0 ? scc : -1).filter(scc=>scc >= 0).sort((a,b)=>a-b);
    const order = [];
    while(ready.length){
      const scc = ready.shift();
      order.push(scc);
      [...sccOutgoing[scc]].sort((a,b)=>a-b).forEach(next => {
        indegree[next]--;
        if(indegree[next] !== 0) return;
        const at = ready.findIndex(candidate=>candidate > next);
        if(at < 0) ready.push(next); else ready.splice(at, 0, next);
      });
    }
    const columnBySCC = sccMembers.map(()=>0);
    order.forEach(scc => {
      sccOutgoing[scc].forEach(next => {
        columnBySCC[next] = Math.max(columnBySCC[next], columnBySCC[scc] + 1);
      });
    });
    return {flowBySlice, columnBySlice:sccOf.map(scc=>columnBySCC[scc])};
  }

  function chapterFor(title, indices, graph){
    const chapter = EMC.el("section", "es-chapter");
    chapter.appendChild(EMC.el("div", "es-chapter-label", EMC.esc(title)));
    const rows = EMC.el("div", "es-rows");
    const groups = new Map();
    indices.forEach(i => {
      const slice = EMC.MODEL.slices[i];
      const flowID = graph.flowBySlice[i];
      if(!groups.has(flowID)){
        const group = EMC.el("div", "es-flow-group");
        group.dataset.flow = flowID;
        rows.appendChild(group);
        groups.set(flowID, group);
      }
      const row = EMC.el("article", "es-row");
      row.dataset.slice = i;
      row.dataset.column = graph.columnBySlice[i];
      row.style.gridColumn = `${graph.columnBySlice[i] + 1}`;
      const status = slice.status || "created";
      const head = EMC.el("button", "es-row-head",
        `<span class="es-slice-idx">${String(i+1).padStart(2,"0")}</span>`+
        `<span class="es-pattern" title="${EMC.esc(EMC.PATTERN_LABEL[slice.type]||slice.type)}">${EMC.PAT_SVG[slice.type]||""}</span>`+
        `<span class="es-row-title">${EMC.esc(slice.title)}</span>`+
        `<span class="status" style="--sc:${EMC.statusVar(status)}"><span class="sd"></span>${EMC.esc(EMC.STATUS_LABEL[status]||EMC.titleize(status))}</span>`);
      head.type = "button";
      head.dataset.slice = i;
      head.dataset.nodeId = "slice__"+slice.id;
      head.addEventListener("click", ()=>EMC.openSlice(i));
      row.appendChild(head);
      const flow = EMC.el("div", "es-flow");
      layoutSliceNotes(slice, i, flow);
      row.appendChild(flow);
      groups.get(flowID).appendChild(row);
    });
    chapter.appendChild(rows);
    return chapter;
  }

  window.renderEventStorming = function(){
    if(built) return;
    built = true;
    const board = document.querySelector("#board-es");
    const wires = document.querySelector("#wires-es");
    if(!board || !wires) return;
    const graph = buildStormingGraph();
    const used = new Set();
    const fragment = document.createDocumentFragment();
    (EMC.MODEL.chapters || []).forEach(chapter => {
      const indices = (chapter.slices || []).map(id=>EMC.MODEL.slices.findIndex(slice=>slice.id===id)).filter(i=>i>=0 && !used.has(i));
      indices.forEach(i=>used.add(i));
      if(indices.length) fragment.appendChild(chapterFor(chapter.title, indices, graph));
    });
    const ungrouped = EMC.MODEL.slices.map((_,i)=>i).filter(i=>!used.has(i));
    if(ungrouped.length) fragment.appendChild(chapterFor("Ungrouped", ungrouped, graph));
    board.appendChild(fragment);

    const unpinned = [];
    (EMC.MODEL.hotspots || []).forEach(hotspot => {
      const target = [...board.querySelectorAll(".es-note")].find(note=>note.dataset.id===hotspot.onId) ||
        [...board.querySelectorAll(".es-row-head")].find(head=>head.dataset.nodeId===hotspot.onId);
      if(!target){ unpinned.push(hotspot); return; }
      const pin = EMC.el("div", "hotspot es-hotspot",
        `<span class="es-hotspot-mark">?!</span><span class="es-hotspot-label">hotspot</span>`+
        `<span class="es-hotspot-question es-note-detail">${EMC.esc(hotspot.question)}</span>`);
      pin.setAttribute("tabindex", "0");
      pin.setAttribute("data-q", `${hotspot.question}  ·  [${hotspot.status}]`);
      target.appendChild(pin);
    });


    function layoutRows(){
      const groups = [...board.querySelectorAll(".es-flow-group")];
      groups.forEach(group=>group.style.gridTemplateColumns = "");
      const columnCount = Math.max(0, ...graph.columnBySlice) + 1;
      const widths = Array(columnCount).fill(0);
      board.querySelectorAll(".es-row").forEach(row => {
        const column = +row.dataset.column;
        widths[column] = Math.max(widths[column], Math.ceil(row.scrollWidth));
      });
      const template = widths.map(width=>`${width}px`).join(" ");
      groups.forEach(group=>group.style.gridTemplateColumns = template);
    }

    let paths = [];
    function drawWires(){
      if(!built) return;
      paths.forEach(path=>path.remove());
      paths = [];
      const width = board.scrollWidth, height = board.scrollHeight;
      wires.setAttribute("viewBox", `0 0 ${width} ${height}`);
      wires.setAttribute("width", width);
      wires.setAttribute("height", height);
      EMC.EDGES.forEach(([fromId,toId]) => {
        const from = [...board.querySelectorAll(".es-note")].find(note=>note.dataset.id===fromId);
        const to = [...board.querySelectorAll(".es-note")].find(note=>note.dataset.id===toId);
        if(!from || !to || from.dataset.slice === to.dataset.slice) return;
        const a = EMC.rectIn(board, from), b = EMC.rectIn(board, to);
        const leftToRight = b.cx >= a.cx;
        const sx = leftToRight ? a.x+a.w : a.x, ex = leftToRight ? b.x : b.x+b.w;
        const sy = a.cy, ey = b.cy, dx = Math.max(34, Math.abs(ex-sx)*.42);
        const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
        path.setAttribute("d", `M ${sx} ${sy} C ${sx+(leftToRight?dx:-dx)} ${sy} ${ex-(leftToRight?dx:-dx)} ${ey} ${ex} ${ey}`);
        path.setAttribute("marker-end", "url(#ah-es)");
        path.dataset.a = fromId;
        path.dataset.b = toId;
        path.classList.add("es-wire-cross");
        if(from.classList.contains("filtered") || to.classList.contains("filtered")) path.classList.add("dim");
        wires.appendChild(path);
        paths.push(path);
      });
    }
    window.relayoutEventStorming = function(){
      layoutRows();
      drawWires();
    };

    function applyStormingFilters(state){
      if(!state) return;
      EMC.MODEL.slices.forEach((slice, i) => {
        const inChapter = state.chapter === "__all" ||
          (EMC.MODEL.chapters.find(c => c.id === state.chapter)?.slices.includes(slice.id));
        const status = slice.status || "created";
        const okStatus = state.statuses.size === 0 || state.statuses.has(status);
        const rowVisible = inChapter && okStatus;
        board.querySelectorAll('.es-row-head[data-slice="'+i+'"]').forEach(head => head.classList.toggle("filtered", !rowVisible));
        slice.elements.forEach(element => {
          const note = board.querySelector('.es-note[data-id="'+element.id+'"]');
          if(!note) return;
          const okCtx = state.context === "__all" || !element.ctx || element.ctx === state.context;
          const filtered = !(rowVisible && okCtx);
          note.classList.toggle("filtered", filtered);
          board.querySelectorAll('.es-aggregate-note[data-for="'+element.id+'"]').forEach(aggregate => aggregate.classList.toggle("filtered", filtered));
        });
      });
      window.relayoutEventStorming && window.relayoutEventStorming();
    }
    window.applyStormingFilters = applyStormingFilters;

    function setHover(note, active){
      const id = note.dataset.id || note.dataset.relatedId;
      if(!id) return;
      const near = new Set([id, ...(EMC.NEIGHBORS[id] || [])]);
      board.classList.toggle("hovering", active);
      board.querySelectorAll(".es-note").forEach(n=>{
        const noteID = n.dataset.id || n.dataset.relatedId;
        n.classList.toggle("is-hot", active && near.has(noteID));
      });
      paths.forEach(path => {
        const hot = active && (path.dataset.a===id || path.dataset.b===id);
        path.classList.toggle("hot", hot);
        path.setAttribute("marker-end", hot ? "url(#ah-es-hot)" : "url(#ah-es)");
      });
    }
    board.addEventListener("mouseover", event => {
      const note = event.target.closest(".es-note");
      if(note && !note.contains(event.relatedTarget)) setHover(note, true);
    });
    board.addEventListener("mouseout", event => {
      const note = event.target.closest(".es-note");
      if(note && !note.contains(event.relatedTarget)) setHover(note, false);
    });
    board.addEventListener("click", event => {
      if(event.target.closest(".hotspot")) return;
      const note = event.target.closest(".es-note");
      if(note) EMC.openSlice(+note.dataset.slice);
    });

    const swatch = (label, fill, line) => `<div class="row"><span class="sw" style="--fl:${fill};--cl:${line}"></span>${label}</div>`;
    const elements = [
      swatch("Domain event", "var(--es-event-fill)", "var(--es-event-line)"),
      swatch("External event", "var(--es-external-fill)", "var(--es-external-line)"),
      swatch("Command", "var(--es-command-fill)", "var(--es-command-line)"),
      swatch("Constraint / aggregate", "var(--es-agg-fill)", "var(--es-agg-line)"),
      swatch("Automation / Translation", "var(--es-policy-fill)", "var(--es-policy-line)"),
      swatch("Read model", "var(--es-read-fill)", "var(--es-read-line)"),
      swatch("Screen", "var(--es-screen-fill)", "var(--es-screen-line)"),
      swatch("Hotspot", "var(--hot-fill)", "var(--hot-line)"),
    ].join("");
    const placedActors = new Set(EMC.MODEL.slices.flatMap(slice=>slice.elements.map(element=>element.actor).filter(Boolean)));
    const actors = Object.entries(EMC.MODEL.actors).map(([id, actor]) =>
      `<div class="row"><span class="sw" style="--fl:var(--es-actor-fill);--cl:var(--es-actor-line)"></span><span>${EMC.esc(actor.title)}${placedActors.has(id)?"":' · <span class="mono">unassigned</span>'}</span></div>`).join("");
    const patterns = Object.entries(EMC.PATTERN_LABEL).map(([key,label])=>`<div class="row"><span class="pg">${EMC.PAT_SVG[key]}</span>${label}</div>`).join("");
    const statuses = Object.entries(EMC.STATUS_LABEL).map(([key,label])=>`<div class="row"><span class="sd" style="background:${EMC.statusVar(key)}"></span>${label}</div>`).join("");
    const other = unpinned.map(h=>`<div class="row"><span class="sw" style="--fl:var(--hot-fill);--cl:var(--hot-line)"></span><span>${EMC.esc(h.question)}${h.target?` · <span class="mono">${EMC.esc(h.target)}</span>`:""}</span></div>`).join("");
    LEGENDS.storming = `<div class="grp"><h4>Elements</h4>${elements}</div>`+
      (actors?`<div class="grp"><h4>Actors</h4>${actors}</div>`:"")+
      `<div class="grp"><h4>Patterns (slice types)</h4>${patterns}</div><div class="grp"><h4>Slice status</h4>${statuses}</div>`+
      (other?`<div class="grp"><h4>Other hotspots</h4>${other}</div>`:"");
    applyStormingFilters(EMC.filterState);
    layoutRows();
    requestAnimationFrame(drawWires);
  };

  window.relayoutEventStorming = function(){};
})();
