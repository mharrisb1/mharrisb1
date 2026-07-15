const contentCache = new Map();

class ContentWindow extends HTMLElement {
  static get observedAttributes() {
    return ["src"];
  }

  constructor() {
    super();
    this.handleClose = this.handleClose.bind(this);
    this.handleKeydown = this.handleKeydown.bind(this);
  }

  connectedCallback() {
    this.innerHTML = `
      <div class="flex justify-end mb-2">
        <button id="close-btn" class="text-secondary hover:text-white cursor-pointer font-bold">[X]</button>
      </div>
      <div class="border px-12 py-8">
        <div id="ajax-content" class="text-primary">Loading...</div>
      </div>
    `;

    this.querySelector("#close-btn").addEventListener("click", this.handleClose);
    document.addEventListener("keydown", this.handleKeydown);

    if (this.hasAttribute("src")) {
      this.loadContent(this.getAttribute("src"));
    }
  }

  attributeChangedCallback(name, oldValue, newValue) {
    if (name === "src" && oldValue !== newValue) {
      if (this.querySelector("#ajax-content")) {
        this.loadContent(newValue);
      }
    }
  }

  async loadContent(src) {
    const container = this.querySelector("#ajax-content");
    if (!container) return;
    
    container.innerHTML = "Loading...";

    if (contentCache.has(src)) {
      container.innerHTML = contentCache.get(src);
      return;
    }

    try {
      const response = await fetch(src);
      if (response.ok) {
        const html = await response.text();
        contentCache.set(src, html);
        container.innerHTML = html;
      } else {
        container.innerHTML = `<span class="text-accent">Failed to load content.</span>`;
      }
    } catch (error) {
      container.innerHTML = `<span class="text-accent">Error loading content.</span>`;
    }
  }

  disconnectedCallback() {
    const btn = this.querySelector("#close-btn");
    if (btn) btn.removeEventListener("click", this.handleClose);
    document.removeEventListener("keydown", this.handleKeydown);
  }

  handleClose() {
    this.dispatchEvent(new CustomEvent("close-window", { bubbles: true, composed: true }));
  }

  handleKeydown(e) {
    if (e.key === "Escape") {
      this.handleClose();
    }
  }
}

customElements.define("content-window", ContentWindow);
