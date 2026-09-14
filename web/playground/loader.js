// loader.js is shared unchanged by the standalone and hosted playgrounds.
// It retries without streaming when a static host serves the WASM file with
// an incorrect MIME type.
(function () {
  "use strict";

  var status = document.getElementById("status");
  var assetURL = new URL("emhcl.wasm", document.currentScript.src);
  var go = new Go();

  fetch(assetURL)
    .then(function (response) {
      if (!response.ok) throw new Error("HTTP " + response.status);
      var fallback = response.clone();
      if (typeof WebAssembly.instantiateStreaming !== "function") {
        return fallback.arrayBuffer().then(function (bytes) {
          return WebAssembly.instantiate(bytes, go.importObject);
        });
      }
      return WebAssembly.instantiateStreaming(response, go.importObject).catch(function () {
        return fallback.arrayBuffer().then(function (bytes) {
          return WebAssembly.instantiate(bytes, go.importObject);
        });
      });
    })
    .then(function (result) { go.run(result.instance); })
    .catch(function (err) {
      status.textContent = "Failed to load emhcl.wasm: " + err;
    });
})();
