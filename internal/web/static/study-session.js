(() => {
  const session = document.querySelector("[data-study-session]");
  if (!session) return;

  const cards = [...session.querySelectorAll("[data-card]")];
  const score = session.querySelector("[data-score]");
  const remaining = session.querySelector("[data-remaining]");
  const current = session.querySelector("[data-current]");
  const progress = session.querySelector("[data-progress]");
  const form = session.querySelector("[data-answer-form]");
  const input = session.querySelector("[data-answer-input]");
  const feedback = session.querySelector("[data-feedback]");
  const completion = session.querySelector("[data-completion]");
  const flip = session.querySelector("[data-flip]");
  const stop = session.querySelector("[data-stop-session]");
  const typedAnswer = session.querySelector("[data-typed-answer]");
  const choiceList = session.querySelector("[data-choice-list]");
  const totalCards = cards.length;
  let answered = 0;
  let correct = 0;
  let isGrading = false;
  let totalAttempts = 0;
  const seenCards = new Set();
  const incorrectCards = new Set();

  function activeCard() { return cards[0]; }
  function updateStatus() {
    remaining.textContent = String(cards.length);
    current.textContent = String(Math.min(answered + 1, totalCards));
    progress.style.width = `${(answered / totalCards) * 100}%`;
  }
  function showAnswer() { activeCard()?.classList.add("is-flipped"); }
  function showQuestionMode() {
    const card = activeCard();
    const isMultipleChoice = card.dataset.questionMode === "multiple_choice";
    typedAnswer.hidden = isMultipleChoice;
    choiceList.hidden = !isMultipleChoice;
    input.required = !isMultipleChoice;
    choiceList.replaceChildren();
    if (!isMultipleChoice) return;
    card.querySelectorAll("[data-choice]").forEach((choice) => {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "choice-button";
      button.textContent = choice.value;
      button.addEventListener("click", () => gradeAnswer(choice.value, button));
      choiceList.append(button);
    });
  }
  function nextCard(isCorrect) {
    const card = activeCard();
    card.classList.add("is-hidden");
    cards.shift();
    if (isCorrect) {
      answered += 1;
    } else {
      card.classList.remove("is-flipped");
      cards.push(card);
      session.querySelector("[data-flashcard]").append(card);
    }
    if (cards.length === 0) {
      session.querySelectorAll("[data-card]").forEach((completedCard) => {
        completedCard.classList.remove("is-hidden", "is-flipped");
        cards.push(completedCard);
      });
      answered = 0;
      feedback.textContent = "Alle kaarten zijn geweest. De serie begint opnieuw.";
    }
    cards[0].classList.remove("is-hidden");
    input.value = "";
    showQuestionMode();
    if (!typedAnswer.hidden) input.focus();
    updateStatus();
  }

  flip.addEventListener("click", showAnswer);
  stop.addEventListener("click", () => {
    form.classList.add("is-hidden");
    completion.classList.remove("is-hidden");
    session.querySelector("[data-final-score]").textContent = `${correct} goed beantwoord`;
    window.speechSynthesis?.cancel();
    const data = new URLSearchParams({
      csrf_token: form.dataset.csrfToken,
      subject: form.dataset.subject,
      period: form.dataset.period,
      mode: "mixed",
      cards_seen: String(seenCards.size),
      correct_first_try: String([...seenCards].filter((cardID) => !incorrectCards.has(cardID)).length),
      total_attempts: String(totalAttempts)
    });
    if (seenCards.size > 0) {
      fetch("/student/study/complete", { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body: data });
    }
  });
  function gradeAnswer(value, selectedButton) {
    if (isGrading) return;
    isGrading = true;
    const answer = value.trim().toLocaleLowerCase();
    const expected = activeCard().dataset.answer.trim().toLocaleLowerCase();
    const isCorrect = answer === expected;
    seenCards.add(activeCard().dataset.cardId);
    totalAttempts += 1;
    if (!isCorrect) incorrectCards.add(activeCard().dataset.cardId);
    if (selectedButton) {
      choiceList.querySelectorAll(".choice-button").forEach((button) => {
        button.disabled = true;
        if (button.textContent.trim().toLocaleLowerCase() === expected) {
          button.classList.add("is-correct");
        }
      });
      if (!isCorrect) selectedButton.classList.add("is-incorrect");
    }
    if (isCorrect) correct += 1;
    score.textContent = String(correct);
    feedback.textContent = isCorrect ? "Goed gedaan." : `Antwoord: ${activeCard().dataset.answer}`;
    showAnswer();
    window.setTimeout(() => {
      isGrading = false;
      nextCard(isCorrect);
    }, 850);
  }
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    gradeAnswer(input.value);
  });
  updateStatus();
  showQuestionMode();
  if (!typedAnswer.hidden) input.focus();
})();