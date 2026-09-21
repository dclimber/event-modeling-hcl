/* --------------------------- Context Map view --------------------------- */
(function(){
  const svgNS = "http://www.w3.org/2000/svg";
  const MIN_RADIUS = 32;
  const MAX_RADIUS = 58;
  const COLUMN_GAP = 300;
  const ROW_GAP = 176;
  const PADDING_X = 110;
  const PADDING_Y = 104;
  let built = false;
  let board;
  let wires;
  let nodesByID = {};
  let edges = [];
  let radii = {};

  const svg = (name, attrs, text) => {
    const node = document.createElementNS(svgNS, name);
    Object.entries(attrs || {}).forEach(([key, value]) => node.setAttribute(key, String(value)));
    if (text != null) node.textContent = text;
    return node;
  };

  function legendHTML(empty){
    const rows = empty
      ? '<div class="row">No bounded contexts were declared in this model.</div>'
      : '<div class="row"><span class="sw" style="--fl:var(--cm-node-fill);--cl:var(--cm-node-line)"></span>Bounded context · circle size approximates event count</div>'+
        '<div class="row"><span class="pg">♙</span>Team label · owning team</div>'+
        '<div class="row"><span class="pg">↗</span>Dashed circle · external context</div>';
    return '<div class="grp"><h4>Contexts</h4>'+rows+'</div>'+ 
      '<div class="grp"><h4>Derived relationships</h4>'+
        '<div class="row"><span class="pg">U → D</span>Upstream → downstream ends</div>'+
        '<div class="row"><span class="sw" style="--fl:var(--cm-tag-fill);--cl:var(--cm-tag-line)"></span>CS · customer/supplier — cross-context event consumption</div>'+
        '<div class="row"><span class="sw" style="--fl:var(--cm-tag-fill);--cl:var(--cm-tag-line)"></span>ACL · anti-corruption layer — consumer is a translation</div>'+
        '<div class="row">OHS/CF/SK/Partnership are not derived: the source model declares no such relationships.</div>'+
      '</div>';
  }

  function clearWires(){
    wires.querySelectorAll('.cm-edge, .cm-edge-badge, .cm-edge-tag').forEach(node => node.remove());
  }

  function drawWires(){
    if (!built || !board || !wires) return;
    clearWires();
    edges.forEach(edge => {
      const upstream = nodesByID[edge.upstream];
      const downstream = nodesByID[edge.downstream];
      if (!upstream || !downstream) return;
      const from = EMC.rectIn(board, upstream.querySelector('.cm-disc'));
      const to = EMC.rectIn(board, downstream.querySelector('.cm-disc'));
      const dx = to.cx - from.cx;
      const dy = to.cy - from.cy;
      const distance = Math.hypot(dx, dy);
      if (!distance) return;
      const ux = dx / distance;
      const uy = dy / distance;
      const fromRadius = radii[edge.upstream];
      const toRadius = radii[edge.downstream];
      const startX = from.cx + ux * fromRadius;
      const startY = from.cy + uy * fromRadius;
      const endX = to.cx - ux * toRadius;
      const endY = to.cy - uy * toRadius;
      const path = svg('path', {
        class: 'cm-edge',
        d: `M ${startX} ${startY} L ${endX} ${endY}`,
        'marker-end': 'url(#ah-cm)',
        'data-upstream': edge.upstream,
        'data-downstream': edge.downstream,
      });
      wires.appendChild(path);

      const badge = (label, x, y) => {
        wires.appendChild(svg('text', {
          class: 'cm-edge-badge', x, y,
          'text-anchor': 'middle',
          'dominant-baseline': 'middle',
          'data-upstream': edge.upstream,
          'data-downstream': edge.downstream,
        }, label));
      };
      badge('U', startX + ux * 15 - uy * 15, startY + uy * 15 + ux * 15);
      badge('D', endX - ux * 15 - uy * 15, endY - uy * 15 + ux * 15);

      const tag = edge.pattern === 'anticorruption' ? 'ACL' : 'CS';
      const tagX = (startX + endX) / 2 - uy * 15;
      const tagY = (startY + endY) / 2 + ux * 15;
      const width = tag === 'ACL' ? 31 : 25;
      const group = svg('g', {class: 'cm-edge-tag', 'data-upstream': edge.upstream, 'data-downstream': edge.downstream});
      group.appendChild(svg('rect', {x: tagX - width / 2, y: tagY - 10, width, height: 20, rx: 4, ry: 4}));
      group.appendChild(svg('text', {x: tagX, y: tagY, 'text-anchor': 'middle', 'dominant-baseline': 'middle'}, tag));
      wires.appendChild(group);
    });
  }

  function setHighlight(id, active){
    if (!built) return;
    const related = new Set([id]);
    edges.forEach(edge => {
      if (edge.upstream === id) related.add(edge.downstream);
      if (edge.downstream === id) related.add(edge.upstream);
    });
    board.classList.toggle('cm-hovering', active);
    Object.entries(nodesByID).forEach(([nodeID, node]) => node.classList.toggle('is-hot', active && related.has(nodeID)));
    wires.querySelectorAll('.cm-edge, .cm-edge-tag, .cm-edge-badge').forEach(node => {
      const upstream = node.dataset.upstream;
      const downstream = node.dataset.downstream;
      node.classList.toggle('hot', active && (upstream === id || downstream === id));
    });
    wires.querySelectorAll('.cm-edge').forEach(path => path.setAttribute('marker-end', path.classList.contains('hot') ? 'url(#ah-cm-hot)' : 'url(#ah-cm)'));
  }

  window.relayoutContextMap = drawWires;
  window.renderContextMap = function(){
    if (built) return;
    built = true;
    board = document.querySelector('#board-cm');
    wires = document.querySelector('#wires-cm');
    window.relayoutContextMap = drawWires;
    const map = (EMC.MODEL && EMC.MODEL.contextMap) || {};
    const nodes = Array.isArray(map.nodes) ? map.nodes : [];
    edges = Array.isArray(map.edges) ? map.edges.filter(edge => edge && edge.upstream && edge.downstream) : [];
    LEGENDS.contextmap = legendHTML(nodes.length === 0);

    if (!nodes.length) {
      board.appendChild(EMC.el('div', 'cm-empty', 'No bounded contexts declared.'));
      return;
    }

    const predecessors = {};
    nodes.forEach(node => { predecessors[node.id] = []; });
    edges.forEach(edge => {
      if (edge.upstream in predecessors && edge.downstream in predecessors) {
        predecessors[edge.downstream].push(edge.upstream);
      }
    });
    const depth = {};
    const state = {};
    function computeDepth(id) {
      if (depth[id] !== undefined) return depth[id];
      state[id] = "visiting";
      let best = 0;
      predecessors[id].forEach(predecessor => {
        if (state[predecessor] === "visiting") return; // back-edge closing the active recursion: ignored entirely, edge still drawn
        best = Math.max(best, computeDepth(predecessor) + 1);
      });
      state[id] = "done";
      depth[id] = best;
      return best;
    }
    nodes.forEach(node => computeDepth(node.id));
    const columns = [];
    nodes.forEach(node => (columns[depth[node.id]] = columns[depth[node.id]] || []).push(node));
    const columnCount = columns.length;
    const tallestColumn = Math.max(...columns.map(column => column ? column.length : 0));
    const maxEvents = Math.max(...nodes.map(node => Number(node.events) || 0));
    const height = PADDING_Y * 2 + Math.max(0, tallestColumn - 1) * ROW_GAP + MAX_RADIUS * 2 + 70;
    const width = PADDING_X * 2 + Math.max(0, columnCount - 1) * COLUMN_GAP + MAX_RADIUS * 2;
    board.style.width = `${width}px`;
    board.style.minHeight = `${height}px`;

    columns.forEach((column, columnIndex) => {
      if (!column) return;
      const columnHeight = Math.max(0, column.length - 1) * ROW_GAP;
      const topOffset = (height - columnHeight) / 2;
      column.forEach((node, rowIndex) => {
        const events = Number(node.events) || 0;
        const radius = maxEvents > 0
          ? MIN_RADIUS + (MAX_RADIUS - MIN_RADIUS) * Math.sqrt(events / maxEvents)
          : (MIN_RADIUS + MAX_RADIUS) / 2;
        radii[node.id] = radius;
        const centerX = PADDING_X + MAX_RADIUS + columnIndex * COLUMN_GAP;
        const centerY = topOffset + rowIndex * ROW_GAP;
        const team = node.team ? `<div class="cm-team">${EMC.ACTOR_SVG}<span>${EMC.esc(node.team)}</span></div>` : '';
        const external = node.external ? '<span class="cm-external">↗ external</span>' : '';
        const element = EMC.el('div', `cm-node${node.external ? ' external' : ''}`,
          `<div class="cm-disc" style="--cm-r:${radius}px"><span class="cm-title">${EMC.esc(node.title)}</span></div>${team}${external}`);
        element.dataset.id = node.id;
        element.style.left = `${centerX - radius}px`;
        element.style.top = `${centerY - radius}px`;
        element.style.width = `${radius * 2}px`;
        element.style.setProperty('--cm-r', `${radius}px`);
        element.tabIndex = 0;
        element.setAttribute('aria-label', `${node.title}${node.external ? ', external context' : ''}`);
        element.addEventListener('mouseenter', () => setHighlight(node.id, true));
        element.addEventListener('mouseleave', () => setHighlight(node.id, false));
        element.addEventListener('focus', () => setHighlight(node.id, true));
        element.addEventListener('blur', () => setHighlight(node.id, false));
        nodesByID[node.id] = element;
        board.appendChild(element);
      });
    });
    requestAnimationFrame(drawWires);
  };
})();
