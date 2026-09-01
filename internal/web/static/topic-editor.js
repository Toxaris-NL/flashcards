(() => {
  const pairs = document.querySelector("[data-pairs]");
  const addButton = document.querySelector("[data-add-pair]");
  if (!pairs || !addButton) return;
  addButton.addEventListener("click", () => {
    const row = document.createElement("div");
    row.dataset.pair = "";
    row.innerHTML = '<input name="id" type="hidden"><input name="side_a"><input name="side_b"><select name="side_a_language"><option value="">Standaard</option><option value="fr">Frans</option><option value="de">Duits</option><option value="en">Engels</option><option value="nl">Nederlands</option></select><select name="side_b_language"><option value="">Standaard</option><option value="fr">Frans</option><option value="de">Duits</option><option value="en">Engels</option><option value="nl">Nederlands</option></select><button type="button" data-remove-pair>Verwijderen</button>';
    pairs.append(row);
  });
  pairs.addEventListener("click", (event) => {
    if (event.target.matches("[data-remove-pair]")) event.target.closest("[data-pair]").remove();
  });

  const importFile = document.querySelector("[data-import-file]");
  const importPreview = document.querySelector("[data-import-preview]");
  const confirmImport = document.querySelector("[data-confirm-import]");
  const importMessage = document.querySelector("[data-import-message]");
  const editorForm = document.querySelector('form[action="/student/topics"]');
  let importedCards = [];

  function appendCard(card) {
    const row = document.createElement("div");
    row.dataset.pair = "";
    row.innerHTML = '<input name="id" type="hidden"><input name="side_a"><input name="side_b"><select name="side_a_language"><option value="">Standaard</option><option value="fr">Frans</option><option value="de">Duits</option><option value="en">Engels</option><option value="nl">Nederlands</option></select><select name="side_b_language"><option value="">Standaard</option><option value="fr">Frans</option><option value="de">Duits</option><option value="en">Engels</option><option value="nl">Nederlands</option></select><button type="button" data-remove-pair>Verwijderen</button>';
    row.querySelector('[name="side_a"]').value = card.front;
    row.querySelector('[name="side_b"]').value = card.back;
    pairs.append(row);
  }

  importFile?.addEventListener("change", async () => {
    const subject = editorForm.querySelector('[name="subject"]').value.trim();
    const period = editorForm.querySelector('[name="period"]').value.trim();
    const file = importFile.files[0];
    if (!subject || !period || !file) {
      importMessage.textContent = "Vul eerst vak en periode in voordat je importeert.";
      return;
    }
    const data = new URLSearchParams();
    data.set("csrf_token", editorForm.querySelector('[name="csrf_token"]').value);
    data.set("subject", subject);
    data.set("data", await file.text());
    try {
      const response = await fetch("/student/topics/import", {
        method: "POST",
        body: data,
        headers: { "Content-Type": "application/x-www-form-urlencoded" }
      });
      if (!response.ok) throw new Error();
      const result = await response.json();
      importedCards = result.Cards || result.cards || [];
      const errors = result.Errors || result.errors || [];
      importPreview.innerHTML = `<strong>${importedCards.length} kaarten klaar om toe te voegen</strong><ul>${importedCards.map((card) => `<li>${card.front} - ${card.back}</li>`).join("")}</ul>${errors.length ? `<p>${errors.join("; ")}</p>` : ""}`;
      importPreview.hidden = false;
      confirmImport.hidden = importedCards.length === 0;
      importMessage.textContent = "";
    } catch {
      importMessage.textContent = "CSV kon niet worden gelezen.";
    }
  });

  confirmImport?.addEventListener("click", () => {
    importedCards.forEach(appendCard);
    importedCards = [];
    importPreview.hidden = true;
    confirmImport.hidden = true;
    importFile.value = "";
    importMessage.textContent = "Kaarten toegevoegd. Kies Opslaan om de lijst te bewaren.";
  });
})();