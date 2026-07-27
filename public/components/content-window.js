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
      <div class="flex flex-col" style="margin: 0.5rem; width: calc(100% - 1rem); height: calc(100% - 1rem);">
        <div class="flex justify-end mb-2">
          <button id="close-btn" class="text-secondary hover:text-white cursor-pointer font-bold">[X]</button>
        </div>
        <div class="border px-4 py-2" style="overflow-y: auto; flex-grow: 1; min-height: 0;">
          <div id="ajax-content" class="text-primary">Loading...</div>
        </div>
      </div>
    `;

    this.querySelector("#close-btn").addEventListener(
      "click",
      this.handleClose,
    );
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
      this.processContent(container);
      return;
    }

    try {
      const response = await fetch(src);
      if (response.ok) {
        const html = await response.text();
        contentCache.set(src, html);
        container.innerHTML = html;
        this.processContent(container);
      } else {
        container.innerHTML = `<span class="text-accent">Failed to load content.</span>`;
      }
    } catch (_) {
      container.innerHTML = `<span class="text-accent">Error loading content.</span>`;
    }
  }

  processContent(container) {
    if (window.hljs) {
      container.querySelectorAll('pre code').forEach(block => hljs.highlightElement(block));
    }
    container.querySelectorAll('table').forEach(table => {
      if (!table.parentElement.classList.contains('table-wrapper')) {
        const wrapper = document.createElement('div');
        wrapper.className = 'table-wrapper';
        table.parentNode.insertBefore(wrapper, table);
        wrapper.appendChild(table);
      }
    });
    container.querySelectorAll('script').forEach(oldScript => {
      const newScript = document.createElement('script');
      Array.from(oldScript.attributes).forEach(attr => newScript.setAttribute(attr.name, attr.value));
      newScript.appendChild(document.createTextNode(oldScript.innerHTML));
      oldScript.parentNode.replaceChild(newScript, oldScript);
    });
  }

  disconnectedCallback() {
    const btn = this.querySelector("#close-btn");
    if (btn) btn.removeEventListener("click", this.handleClose);
    document.removeEventListener("keydown", this.handleKeydown);
  }

  handleClose() {
    this.dispatchEvent(
      new CustomEvent("close-window", { bubbles: true, composed: true }),
    );
  }

  handleKeydown(e) {
    if (e.key === "Escape") {
      this.handleClose();
    }
  }
}

customElements.define("content-window", ContentWindow);
