(() => {
  const subject = document.querySelector('select[name="subject"]');
  const period = document.querySelector('select[name="period"]');
  const editLink = document.querySelector("[data-edit-list]");
  if (!subject || !period || !editLink) return;

  function updateEditLink() {
    const query = new URLSearchParams({ subject: subject.value, period: period.value });
    editLink.href = `/student/topics/edit?${query}`;
  }

  subject.addEventListener("change", updateEditLink);
  period.addEventListener("change", updateEditLink);
  updateEditLink();
})();