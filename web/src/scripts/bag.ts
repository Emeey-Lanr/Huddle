// The only JavaScript on the shop pages. All other motion is CSS, so it runs
// off the main thread and stays smooth on cheap phones.

const calm = matchMedia("(prefers-reduced-motion: reduce)").matches;
const pop = (el: Element) => {
  if (!calm) el.animate([{ transform: "scale(1)" }, { transform: "scale(1.5)" }, { transform: "scale(1)" }], { duration: 320, easing: "cubic-bezier(.2,.8,.2,1)" });
};

let toastTimer: ReturnType<typeof setTimeout>;
function toast(message: string) {
  const el = document.querySelector("[data-toast]");
  if (!el) return;
  el.textContent = message;
  el.classList.add("is-showing");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.classList.remove("is-showing"), 2800);
}

document.addEventListener("click", async (event) => {
  const button = (event.target as Element).closest<HTMLButtonElement>("[data-add]");
  if (!button || button.disabled) return;

  button.disabled = true;
  const label = button.querySelector("[data-label]");
  try {
    const res = await fetch("/api/cart", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ product_id: button.dataset.add }),
    });
    const body = await res.json();
    if (!res.ok) throw new Error(body.message);

    const count = document.querySelector("[data-bag-count]");
    if (count) {
      count.textContent = String(body.count);
      pop(count);
    }
    button.classList.add("is-added");
    if (label) label.textContent = "Added";
    setTimeout(() => {
      button.classList.remove("is-added");
      if (label) label.textContent = "Add to bag";
    }, 1400);
  } catch (err) {
    toast(err instanceof Error && err.message ? err.message : "Couldn't add that. Check your connection and try again.");
  } finally {
    button.disabled = false;
  }
});
