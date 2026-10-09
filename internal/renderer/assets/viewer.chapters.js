/* Chapter overview: one card per chapter, previewing its first and last slice. */
(function(){
  const lanes = [
    {name:"Screens", kinds:["screen","screen_image"]},
    {name:"Processors", kinds:["processor"]},
    {name:"Model", kinds:["command","readmodel","table"]},
    {name:"Events", kinds:["event"]}
  ];
  const kindLabels = {screen:"Screen",screen_image:"Image",processor:"Processor",command:"Command",readmodel:"Read model",table:"Table",event:"Event"};
  const kindOrder = Object.fromEntries(lanes.flatMap((lane,index)=>lane.kinds.map(kind=>[kind,index])));
  const kindCss = kind => kind === "screen_image" ? "screen" : kind;
  function sliceSummary(slice){
    const node = document.createElement("section");
    node.className = "chapter-slice";
    const heading = document.createElement("h3");
    heading.textContent = slice.title || slice.id;
    node.appendChild(heading);
    const elements = (slice.elements || []).slice().sort((a,b)=>(a.stage||0)-(b.stage||0)||(kindOrder[a.kind]??9)-(kindOrder[b.kind]??9));
    const maxStage = elements.reduce((max,element)=>Math.max(max,element.stage||0),0);
    const diagram = document.createElement("div");
    diagram.className = "chapter-diagram";
    diagram.style.setProperty("--chapter-stages", maxStage+1);
    lanes.forEach(lane=>{
      const laneElements = elements.filter(element=>lane.kinds.includes(element.kind));
      if(!laneElements.length) return;
      const row = document.createElement("div");
      row.className = "chapter-lane";
      const label = document.createElement("span");
      label.className = "chapter-lane-label";
      label.textContent = lane.name;
      row.appendChild(label);
      const track = document.createElement("div");
      track.className = "chapter-lane-track";
      const byStage = new Map();
      laneElements.forEach(element=>{
        const stage = Math.max(0,element.stage||0);
        if(!byStage.has(stage)) byStage.set(stage,[]);
        byStage.get(stage).push(element);
      });
      [...byStage.entries()].forEach(([stage,elementsAtStage])=>{
        const stageColumn = document.createElement("div");
        stageColumn.className = "chapter-stage";
        stageColumn.style.gridColumn = (stage+1);
        elementsAtStage.forEach(element=>{
          const card = document.createElement("div");
          card.className = `chapter-node ${kindCss(element.kind)}${element.external ? " external" : ""}`;
          card.dataset.elementId = element.id;
          const kind = document.createElement("span");
          kind.className = "chapter-node-kind";
          kind.textContent = kindLabels[element.kind] || element.kind;
          const title = document.createElement("span");
          title.className = "chapter-node-title";
          title.textContent = element.title || element.id;
          card.append(kind,title);
          
          stageColumn.appendChild(card);
        });
        track.appendChild(stageColumn);
      });
      row.appendChild(track);
      diagram.appendChild(row);
    });
    if(diagram.childElementCount) {
      node.appendChild(diagram);
    } else {
      const empty = document.createElement("p");
      empty.className = "chapter-empty";
      empty.textContent = "No elements in this slice";
      node.appendChild(empty);
    }
    return node;
  }
  window.renderChapters = function(){
    const host = document.getElementById("board-chapters");
    const source = window.EMC && EMC.FULL_MODEL;
    if(!host || !source) return;
    host.replaceChildren();
    host.hidden = false;
    const slices = new Map((source.slices || []).map(slice=>[slice.id,slice]));
    const records = EMC.chapters || [];
    if(!records.length){
      const empty = document.createElement("p");
      empty.className = "chapter-empty-model";
      empty.textContent = "No chapters are available.";
      host.appendChild(empty);
      return;
    }
    const list = document.createElement("div");
    list.className = "chapter-list";
    records.forEach((chapter,index)=>{
      const card = document.createElement("article");
      card.className = "chapter-overview";
      const sliceIDs = (chapter.slices || []).filter(id=>slices.has(id));
      const header = document.createElement("div");
      header.className = "chapter-overview-header";
      const number = document.createElement("span");
      number.className = "chapter-number mono";
      number.textContent = String(index+1).padStart(2,"0");
      const title = document.createElement("h2");
      // One link per card; CSS stretches its hit area over the whole card.
      const link = document.createElement("a");
      link.className = "chapter-link";
      link.href = EMC.chapterHref(chapter.id) + "/model";
      link.textContent = chapter.title;
      title.appendChild(link);
      const count = document.createElement("span");
      count.className = "chapter-count mono";
      count.textContent = `${sliceIDs.length} ${sliceIDs.length===1?"slice":"slices"}`;
      header.append(number,title,count);
      card.appendChild(header);
      const preview = document.createElement("div");
      const ordered = sliceIDs.map(id=>slices.get(id));
      preview.className = "chapter-preview " + (ordered.length===1 ? "one-slice" : ordered.length===2 ? "two-slices" : "many-slices");
      if(!ordered.length){
        const noSlices = document.createElement("p");
        noSlices.className = "chapter-empty";
        noSlices.textContent = "No slices";
        preview.appendChild(noSlices);
      } else {
        const addSlice = slice => preview.appendChild(sliceSummary(slice));
        addSlice(ordered[0]);
        if(ordered.length > 2){
          const omitted = ordered.length-2;
          const omission = document.createElement("div");
          omission.className = "chapter-omission mono";
          omission.innerHTML = `<span aria-hidden="true">···</span><span>${omitted} more ${omitted===1?"slice":"slices"}</span>`;
          preview.appendChild(omission);
        }
        if(ordered.length > 1) addSlice(ordered[ordered.length-1]);
      }
      card.appendChild(preview);
      list.appendChild(card);
    });
    host.appendChild(list);
  };
})();
