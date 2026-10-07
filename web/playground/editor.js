// editor.js wires a plain <textarea>/<select>/<iframe>/status-list page to
// the eventModelingRender / eventModelingFormat globals cmd/wasm exposes.
// It knows nothing about WHERE it is hosted — only the DOM element ids
// below — so this exact file is copied verbatim into the website's
// playground (see em-hcl-spec's Makefile `build-wasm` target and the
// website's deploy workflow) rather than reimplemented there. Treat any
// change to the DOM contract (the ids referenced here) or to the WASM
// function names/return shape as a breaking change to both call sites.
//
// Expected host markup:
//   <textarea id="editor">...</textarea>
//   <select id="profile"><option value="workshop">...<option value="valid">...<option value="strict">...</select>
//   <iframe id="preview"></iframe>
//   <ul id="diagnostics"></ul>
//   <button id="format-btn">Format</button>
//   <span id="status"></span>
// Optional, enables multi-file mode (several .em.hcl files form one model):
//   <div id="files"></div>
(function () {
  "use strict";

  var SEED = typeof window.EVENT_MODELING_SEED === 'string' ? window.EVENT_MODELING_SEED : '';

  var DEBOUNCE_MS = 300;

  var editorEl, profileEl, previewEl, diagnosticsEl, formatBtnEl, statusEl, sourceEditor;
  var debounceHandle = null;
  var hasRendered = false;

  function setStatus(text) {
    if (statusEl) statusEl.textContent = text;
    document.dispatchEvent(new CustomEvent('eventmodeling:status', { detail: text }));
  }

  function renderDiagnostics(diagnostics) {
    diagnosticsEl.innerHTML = '';
    if (!diagnostics || diagnostics.length === 0) {
      var empty = document.createElement('li');
      empty.className = 'diagnostic-empty';
      empty.textContent = 'No diagnostics.';
      diagnosticsEl.appendChild(empty);
      document.dispatchEvent(new CustomEvent('eventmodeling:diagnostics', { detail: [] }));
      return;
    }
    diagnostics.forEach(function (diagnostic) {
      var item = document.createElement('li');
      item.className = 'diagnostic diagnostic-' + String(diagnostic.severity || '').toLowerCase();

      var head = document.createElement('code');
      head.textContent = (diagnostic.severity || '') + ' ' + (diagnostic.code || '');
      item.appendChild(head);

      var summary = document.createTextNode(': ' + (diagnostic.summary || ''));
      item.appendChild(summary);

      if (diagnostic.detail) {
        item.appendChild(document.createTextNode(' — ' + diagnostic.detail));
      }
      var file = filesEl ? diagnostic.file : '';
      if (diagnostic.line) {
        var where = document.createElement('span');
        where.className = 'diagnostic-location';
        where.textContent = (file ? ' ' + file : '') + ' (line ' + diagnostic.line + ', column ' + diagnostic.column + ')';
        item.appendChild(where);
      } else if (file) {
        var whereFile = document.createElement('span');
        whereFile.className = 'diagnostic-location';
        whereFile.textContent = ' ' + file;
        item.appendChild(whereFile);
      }
      if (file && findFile(file)) {
        item.style.cursor = 'pointer';
        item.addEventListener('click', function () { activateFile(file); });
      }
      diagnosticsEl.appendChild(item);
    });
    document.dispatchEvent(new CustomEvent('eventmodeling:diagnostics', { detail: diagnostics || [] }));
  }

  // renderNow calls into WASM immediately (no debounce) — used both by the
  // debounced input handler and directly after Format, so a Format that
  // changes the source is reflected in the preview without waiting.
  function renderNow() {
    if (typeof window.eventModelingRender !== 'function') return;
    var input = filesEl
      ? files.map(function (file) { return { name: file.name, source: file.source }; })
      : sourceEditor.getValue();
    var result = window.eventModelingRender(input, profileEl.value);
    if (result && result.error) {
      setStatus('Error: ' + result.error);
      return;
    }
    renderDiagnostics(result.diagnostics);
    if (result.html) {
      previewEl.srcdoc = result.html;
      hasRendered = true;
      setStatus('Rendered.');
      document.dispatchEvent(new CustomEvent('eventmodeling:rendered', { detail: result }));
    } else {
      if (!hasRendered) {
        previewEl.srcdoc =
          '<!doctype html><meta charset="utf-8"><body style="font:14px system-ui;padding:16px;color:#a33">' +
          'Model has errors — see diagnostics.</body>';
      }
      setStatus(hasRendered ? 'Model has errors; showing last valid diagram.' : 'Model has errors.');
    }
  }

  function scheduleRender() {
    if (debounceHandle) clearTimeout(debounceHandle);
    debounceHandle = setTimeout(renderNow, DEBOUNCE_MS);
  }

  function formatNow() {
    if (typeof window.eventModelingFormat !== 'function') return;
    var result = window.eventModelingFormat(sourceEditor.getValue());
    if (result && result.error) {
      setStatus('Error: ' + result.error);
      return;
    }
    if (result.diagnostics && result.diagnostics.length > 0) {
      if (filesEl) {
        // The wasm formats a single anonymous source; attribute its
        // diagnostics to the file being formatted.
        result.diagnostics.forEach(function (diagnostic) { diagnostic.file = activeFile; });
      }
      renderDiagnostics(result.diagnostics);
      setStatus('Could not format — fix the errors below first.');
      return;
    }
    sourceEditor.setValue(result.source);
    if (filesEl) {
      var active = findFile(activeFile);
      if (active) active.source = result.source;
    }
    renderNow();
  }

  // ---- Optional multi-file mode -------------------------------------------
  // Active only when the host page has a #files container. Each file is a
  // {name, source} pair; the tab strip lists them sorted by name, which is
  // also the model order the wasm renders them in.

  var filesEl = null;
  var files = [];
  var activeFile = '';
  var filesDisabled = true;

  function sortedFiles() {
    return files.slice().sort(function (a, b) {
      return a.name < b.name ? -1 : a.name > b.name ? 1 : 0;
    });
  }

  function findFile(name) {
    for (var index = 0; index < files.length; index++) {
      if (files[index].name === name) return files[index];
    }
    return null;
  }

  // initialFiles returns the host-provided window.EventModelingInitialFiles
  // when it is a non-empty array of {name, source} strings with unique,
  // valid file names, else one file holding the single-file initial source.
  // It uses an index loop so that holes in a sparse array fail the check.
  function initialFiles() {
    var given = window.EventModelingInitialFiles;
    if (Array.isArray(given) && given.length > 0) {
      var valid = true;
      var seen = {};
      for (var index = 0; index < given.length; index++) {
        var file = given[index];
        if (!file || typeof file.name !== 'string' || typeof file.source !== 'string' ||
            Object.prototype.hasOwnProperty.call(seen, file.name) ||
            validateFileName(file.name, file.name) !== '') {
          valid = false;
          break;
        }
        seen[file.name] = true;
      }
      if (valid) {
        return given.map(function (file) { return { name: file.name, source: file.source }; });
      }
    }
    return [{
      name: 'model.em.hcl',
      source: typeof window.EventModelingInitialSource === 'string' ? window.EventModelingInitialSource : SEED
    }];
  }

  // saveActiveFile copies the editor text into the active file.
  function saveActiveFile() {
    var active = findFile(activeFile);
    if (active) active.source = sourceEditor.getValue();
  }

  function notifyFiles() {
    document.dispatchEvent(new CustomEvent('eventmodeling:files', {
      detail: {
        files: sortedFiles().map(function (file) { return file.name; }),
        active: activeFile
      }
    }));
  }

  // validateFileName returns the reason a name is unusable, or '' when it is
  // fine. currentName is the file being renamed (it may keep its own name).
  function validateFileName(name, currentName) {
    if (name.charAt(0) === '.') return 'File name must not start with a dot.';
    if (name.indexOf('/') >= 0 || name.indexOf('\\') >= 0) return 'File name must not contain / or \\.';
    if (name.length < '.em.hcl'.length || name.slice(-'.em.hcl'.length) !== '.em.hcl') {
      return 'File name must end in .em.hcl.';
    }
    if (name !== currentName && findFile(name)) return 'A file named ' + name + ' already exists.';
    return '';
  }

  // markActiveTab updates the selected state of the existing tabs in place
  // (no rebuild, so a double-click on a tab still reaches the same element).
  function markActiveTab() {
    var tabs = filesEl.querySelectorAll('.file-tab');
    for (var index = 0; index < tabs.length; index++) {
      var selected = tabs[index].getAttribute('data-file') === activeFile;
      tabs[index].className = 'file-tab' + (selected ? ' file-tab-active' : '');
      tabs[index].setAttribute('aria-selected', selected ? 'true' : 'false');
    }
  }

  function renderTabs() {
    var sorted = sortedFiles();
    filesEl.innerHTML = '';
    sorted.forEach(function (file) {
      var wrap = document.createElement('span');
      wrap.className = 'file-tab-wrap';
      wrap.setAttribute('role', 'presentation');

      var tab = document.createElement('button');
      tab.type = 'button';
      tab.className = 'file-tab';
      tab.setAttribute('role', 'tab');
      tab.setAttribute('data-action', 'select');
      tab.setAttribute('data-file', file.name);
      tab.textContent = file.name;
      tab.disabled = filesDisabled;
      wrap.appendChild(tab);

      if (sorted.length >= 2) {
        var remove = document.createElement('button');
        remove.type = 'button';
        remove.className = 'file-tab-remove';
        remove.title = 'Remove file';
        remove.setAttribute('aria-label', 'Remove ' + file.name);
        remove.setAttribute('data-action', 'remove');
        remove.setAttribute('data-file', file.name);
        remove.textContent = '\u00d7';
        remove.disabled = filesDisabled;
        wrap.appendChild(remove);
      }
      filesEl.appendChild(wrap);
    });

    var add = document.createElement('button');
    add.type = 'button';
    add.className = 'file-tab-add';
    add.title = 'Add file';
    add.setAttribute('aria-label', 'Add file');
    add.setAttribute('data-action', 'add');
    add.textContent = '+';
    add.disabled = filesDisabled;
    filesEl.appendChild(add);

    markActiveTab();
  }

  // activateFile saves the editor text into the current file, then shows the
  // named file in the editor.
  function activateFile(name) {
    var target = findFile(name);
    if (!target || name === activeFile) return;
    saveActiveFile();
    activeFile = name;
    sourceEditor.setValue(target.source);
    markActiveTab();
    notifyFiles();
  }

  function addFile() {
    var answer = window.prompt('New file name (must end in .em.hcl):', '');
    if (answer === null) return;
    var name = answer.trim();
    var reason = validateFileName(name, null);
    if (reason) {
      setStatus(reason);
      return;
    }
    saveActiveFile();
    files.push({ name: name, source: '' });
    activeFile = name;
    sourceEditor.setValue('');
    renderTabs();
    notifyFiles();
    renderNow();
  }

  function renameFile(oldName) {
    var file = findFile(oldName);
    if (!file) return;
    var answer = window.prompt('Rename file (must end in .em.hcl):', oldName);
    if (answer === null) return;
    var name = answer.trim();
    if (name === oldName) return;
    var reason = validateFileName(name, oldName);
    if (reason) {
      setStatus(reason);
      return;
    }
    saveActiveFile();
    file.name = name;
    if (activeFile === oldName) activeFile = name;
    renderTabs();
    notifyFiles();
    renderNow();
  }

  function removeFile(name) {
    if (files.length < 2 || !findFile(name)) return;
    if (!window.confirm('Remove ' + name + '? Its text is lost.')) return;
    saveActiveFile();
    files = files.filter(function (file) { return file.name !== name; });
    if (activeFile === name) {
      activeFile = sortedFiles()[0].name;
      sourceEditor.setValue(findFile(activeFile).source);
    }
    renderTabs();
    notifyFiles();
    renderNow();
  }

  // actionTarget finds the nearest element at or above target (below the
  // tab strip) that carries a data-action.
  function actionTarget(target) {
    var element = target;
    while (element && element !== filesEl) {
      if (element.getAttribute && element.getAttribute('data-action')) return element;
      element = element.parentNode;
    }
    return null;
  }

  function onFilesClick(event) {
    var element = actionTarget(event.target);
    if (!element) return;
    var action = element.getAttribute('data-action');
    if (action === 'select') activateFile(element.getAttribute('data-file'));
    else if (action === 'remove') removeFile(element.getAttribute('data-file'));
    else if (action === 'add') addFile();
  }

  function onFilesDblClick(event) {
    var element = actionTarget(event.target);
    if (element && element.getAttribute('data-action') === 'select') {
      renameFile(element.getAttribute('data-file'));
    }
  }

  // ready is invoked by cmd/wasm once the module has finished initializing
  // (window.onEventModelingReady, wired below, before the module loads).
  function ready() {
    setStatus('Ready.');
    sourceEditor.setDisabled(false);
    formatBtnEl.disabled = false;
    if (filesEl) {
      filesDisabled = false;
      renderTabs();
    }
    renderNow();
  }

  // init runs synchronously as soon as this file is parsed — deliberately
  // NOT deferred to DOMContentLoaded. cmd/wasm calls window.onEventModelingReady
  // (set here) as soon as the module finishes loading, which can happen
  // before DOMContentLoaded would otherwise fire; the callback must exist
  // by then. This is safe only because the host page includes this script
  // via <script src="editor.js"> placed AFTER the markup below in the
  // document (so the elements already exist) and BEFORE the wasm-loading
  // script (so this callback is registered before the module can call it).
  // Both host pages (this one and the website's /playground/) must keep
  // that ordering.
  function init() {
    editorEl = document.getElementById('editor');
    profileEl = document.getElementById('profile');
    previewEl = document.getElementById('preview');
    diagnosticsEl = document.getElementById('diagnostics');
    formatBtnEl = document.getElementById('format-btn');
    statusEl = document.getElementById('status');
    filesEl = document.getElementById('files');

    sourceEditor = window.EventModelingEditorAdapter || {
      getValue: function () { return editorEl.value; },
      setValue: function (value) { editorEl.value = value; },
      setDisabled: function (disabled) { editorEl.disabled = disabled; },
      onChange: function (listener) { editorEl.addEventListener('input', listener); }
    };

    if (filesEl) {
      files = initialFiles();
      activeFile = sortedFiles()[0].name;
      sourceEditor.setValue(findFile(activeFile).source);
    } else {
      sourceEditor.setValue(typeof window.EventModelingInitialSource === 'string' ? window.EventModelingInitialSource : SEED);
    }
    sourceEditor.setDisabled(true);
    formatBtnEl.disabled = true;

    if (filesEl) {
      sourceEditor.onChange(function () {
        saveActiveFile();
        scheduleRender();
      });
      filesEl.addEventListener('click', onFilesClick);
      filesEl.addEventListener('dblclick', onFilesDblClick);
      renderTabs();
      notifyFiles();
    } else {
      sourceEditor.onChange(scheduleRender);
    }
    profileEl.addEventListener('change', renderNow);
    formatBtnEl.addEventListener('click', formatNow);

    window.onEventModelingReady = ready;
  }

  init();
})();
