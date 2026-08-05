export function wireQuiz(root) {
  const buttons = root.querySelectorAll("[data-answer]");
  const feedback = root.querySelector("[data-feedback]");
  const correct = root.dataset.correct;
  buttons.forEach((btn) => {
    btn.addEventListener("click", () => {
      const ok = btn.dataset.answer === correct;
      feedback.textContent = ok
        ? root.dataset.ok || "Certo."
        : root.dataset.bad || "Não — tente de novo.";
      feedback.className = "feedback " + (ok ? "ok" : "bad");
    });
  });
}

document.querySelectorAll("[data-quiz]").forEach((el) => wireQuiz(el));
