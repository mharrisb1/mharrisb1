class BlogMenu extends HTMLElement {
  async connectedCallback() {
    this.innerHTML = `<div class="text-secondary">Loading blogs...</div>`;
    try {
      const response = await fetch("/data/blogs.json");
      let blogs = await response.json();

      blogs = blogs
        .filter((b) => !b.draft)
        .toSorted((a, b) => new Date(b.date) - new Date(a.date));

      this.innerHTML = `
        <div class="space-y-6 w-full">
          <h2 class="mb-4">Blog</h2>
          <div class="space-y-4">
            ${blogs
              .map(
                (blog) => `
              <div class="border px-2 py-1 cursor-pointer" data-id="${blog.id}" style="padding: 1rem;">
                <div class="flex justify-between items-center mb-2">
                  <h3>${blog.title}</h3>
                  <span class="text-secondary" style="white-space: nowrap; margin-left: 1rem;">${blog.date}</span>
                </div>
                <div class="flex" style="gap: 1.5rem; flex-wrap: wrap;">
                  <div style="flex: 1 1 250px;">
                    <p class="text-primary mb-4">${blog.blurb}</p>
                    <div class="flex" style="gap: 0.5rem; flex-wrap: wrap;">
                      ${blog.tags.map((tag) => `<span class="bg-primary text-secondary px-2 py-1" style="font-size: 0.875rem;">#${tag}</span>`).join("")}
                    </div>
                  </div>
                  ${blog.thumbnail ? `<img src="${blog.thumbnail}" style="width: 240px; height: 135px; object-fit: cover; border-radius: 6px; border: 1px solid var(--color-zinc); flex-shrink: 0;" />` : ""}
                </div>
              </div>
            `,
              )
              .join("")}
          </div>
        </div>
      `;

      this.querySelectorAll("[data-id]").forEach((el) => {
        el.addEventListener("click", () => {
          const id = el.getAttribute("data-id");
          this.dispatchEvent(
            new CustomEvent("navigate", {
              detail: { path: `/blog?id=${id}` },
              bubbles: true,
              composed: true,
            }),
          );
        });
      });
    } catch (_) {
      this.innerHTML = `<div class="text-accent">Failed to load blogs.</div>`;
    }
  }
}

customElements.define("blog-menu", BlogMenu);
