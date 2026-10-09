/* Chapter overview: one card per chapter, previewing its first and last slice. */
(function(){
  function sliceSummary(slice){
    const node = document.createElement("section");
    node.className = "chapter-slice";
    const heading = document.createElement("h3");
    heading.textContent = slice.title || slice.id;
    node.append(heading, EMC.sliceTimeline(slice));
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
