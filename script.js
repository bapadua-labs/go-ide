(() => {
  const buttons = document.querySelectorAll("[data-copy]");

  buttons.forEach((btn) => {
    btn.addEventListener("click", async () => {
      const text = btn.getAttribute("data-copy");
      if (!text) return;

      try {
        await navigator.clipboard.writeText(text);
      } catch {
        const ta = document.createElement("textarea");
        ta.value = text;
        ta.setAttribute("readonly", "");
        ta.style.position = "fixed";
        ta.style.opacity = "0";
        document.body.appendChild(ta);
        ta.select();
        document.execCommand("copy");
        document.body.removeChild(ta);
      }

      const original = btn.textContent;
      btn.textContent = "Copiado";
      btn.classList.add("copied");
      window.setTimeout(() => {
        btn.textContent = original;
        btn.classList.remove("copied");
      }, 1600);
    });
  });
})();
