(() => {
  const card = document.querySelector("[data-study-session]");
  const supportsSpeech = "speechSynthesis" in window && "SpeechSynthesisUtterance" in window;

  if (!card) {
    return;
  }

  if (!supportsSpeech) {
    document.querySelectorAll(".pronunciation-button").forEach((button) => {
      button.hidden = true;
    });
    return;
  }

  let activeButton;
  let voices = [];
  function loadVoices() { voices = window.speechSynthesis.getVoices(); }
  loadVoices();
  window.speechSynthesis.addEventListener?.("voiceschanged", loadVoices);

  document.querySelectorAll(".pronunciation-button").forEach((button) => {
    button.setAttribute("aria-label", window.flashcardsStringsNL.pronunciation);
    button.addEventListener("click", () => {
      const side = button.closest("[data-card-side]");
      const text = side.querySelector("[data-card-text]").textContent.trim();
      if (!text) {
        return;
      }
      if (activeButton) {
        window.speechSynthesis.cancel();
      }
      window.speechSynthesis.cancel();
      const utterance = new SpeechSynthesisUtterance(text);
      if (side.dataset.language) {
        utterance.lang = side.dataset.language.toLowerCase();
        const voice = voices.find((candidate) => {
          const voiceLanguage = candidate.lang.toLowerCase();
          return voiceLanguage === utterance.lang || voiceLanguage.startsWith(`${utterance.lang}-`) || utterance.lang.startsWith(`${voiceLanguage}-`);
        });
        if (voice) {
          utterance.voice = voice;
        }
      }
      activeButton = button;
      button.disabled = true;
      utterance.onend = utterance.onerror = () => {
        if (activeButton === button) activeButton = undefined;
        button.disabled = false;
      };
      window.speechSynthesis.speak(utterance);
    });
  });
})();