class AppMenu extends HTMLElement {
  constructor() {
    super();
    this.handleKeydown = this.handleKeydown.bind(this);
  }

  handleKeydown(e) {
    const key = e.key.toLowerCase();
    if (key === "a") {
      this.navigateTo("/about");
    } else if (key === "b") {
      this.navigateTo("/blog");
    } else if (key === "g") {
      const el = this.querySelector('a[href*="github.com"]');
      if (el) el.click();
    } else if (key === "l") {
      const el = this.querySelector('a[href*="linkedin.com"]');
      if (el) el.click();
    }
  }

  connectedCallback() {
    this.renderMenu();
    document.addEventListener("keydown", this.handleKeydown);

    const aboutLink = this.querySelector('a[data-path="/about"]');
    if (aboutLink) {
      aboutLink.addEventListener("click", (e) => {
        e.preventDefault();
        this.navigateTo("/about");
      });
    }
    
    const blogLink = this.querySelector('a[data-path="/blog"]');
    if (blogLink) {
      blogLink.addEventListener("click", (e) => {
        e.preventDefault();
        this.navigateTo("/blog");
      });
    }
  }

  navigateTo(path) {
    this.dispatchEvent(new CustomEvent("navigate", {
      detail: { path },
      bubbles: true,
      composed: true
    }));
  }

  disconnectedCallback() {
    document.removeEventListener("keydown", this.handleKeydown);
  }

  renderMenu() {
    this.innerHTML = `
        <div class="flex justify-between items-center text-secondary">
          <a class="text-secondary" href="#" data-path="/about">About</a>
          <div>&lt;a&gt;</div>
        </div>
        <div class="flex justify-between items-center text-secondary">
          <a class="text-secondary" href="#" data-path="/blog">Blog</a>
          <div>&lt;b&gt;</div>
        </div>
        <div class="flex justify-between items-center text-secondary">
          <a
            class="text-secondary"
            href="https://github.com/mharrisb1"
            target="_blank"
          >
            GitHub
          </a>
          <div>&lt;g&gt;</div>
        </div>
        <div class="flex justify-between items-center text-secondary">
          <a
            class="text-secondary"
            href="https://linkedin.com/in/mharrisb1"
            target="_blank"
          >
            LinkedIn
          </a>
          <div>&lt;l&gt;</div>
        </div>
    `;
  }
}

customElements.define("app-menu", AppMenu);
