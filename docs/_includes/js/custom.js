// Guard Just the Docs search focus handling when focus leaves the document.
// Upstream reads evt.relatedTarget.id without checking for null.
jtd.onReady(function() {
  function guardSearchFocusout(evt) {
    if (evt.relatedTarget !== null) return;

    document.documentElement.classList.remove('search-active');
    evt.stopImmediatePropagation();
  }

  var searchInput = document.getElementById('search-input');
  var searchResults = document.getElementById('search-results');

  if (searchInput) searchInput.addEventListener('focusout', guardSearchFocusout, true);
  if (searchResults) searchResults.addEventListener('focusout', guardSearchFocusout, true);
});
