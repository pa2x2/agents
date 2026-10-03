// Helpers that draw the project's UI for the mockups in content.js, such as a phone frame, an app
// bar, a list row or an icon. Shared helpers keep every slide's mockups consistent and short.
const card = (rows, footer) => `<div class="v-card">${rows.map((r) => `<div class="v-row">${r}</div>`).join("")}${footer}</div>`;
