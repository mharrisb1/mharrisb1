class AppClock extends HTMLElement {
  constructor() {
    super();
    this.textContent = "--:--:--";
  }

  connectedCallback() {
    this.clockTick();
    this.interval = setInterval(() => this.clockTick(), 1000);
  }

  disconnectedCallback() {
    clearInterval(this.interval);
  }

  clockTick() {
    const d = new Date();
    const [hh, mm, ss] = [d.getHours(), d.getMinutes(), d.getSeconds()].map(
      (n) => ("0" + n).substr(-2)
    );
    this.textContent = `${hh}:${mm}:${ss}`;
  }
}

customElements.define("app-clock", AppClock);
