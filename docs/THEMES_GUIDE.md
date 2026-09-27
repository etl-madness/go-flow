# Flow Builder Theme Customization Guide

This guide explains how the Flow Builder theming system works and provides step-by-step instructions for adding new themes.

---

## 1. Theme Engine Architecture

Flow Builder's theme engine operates via client-side DOM manipulation and CSS styling:

1. **Root Attribute**: The theme is set on the root `<html>` element using the `data-theme` attribute (e.g. `<html data-theme="soft-white">`).
2. **Persistence**: The chosen theme is automatically persisted in browser `localStorage` under the key `flow_builder_theme`. When the page loads, `loadSavedTheme()` retrieves and applies the stored theme immediately.
3. **Template Synchronization**: Flow Builder uses two synchronized files for its UI markup:
   - `builder/html_content.go`: Contains the compiled Go constant string `IndexHTML`.
   - `builder/tmpl/index.html`: Contains the raw template counterpart.
   *(Any theme added to one must also be added to the other to ensure consistency across build configurations and tests).*

---

## 2. Built-In Themes

Flow Builder includes a broad range of themes to accommodate different environments and preferences:

| Theme Key | Name | Type | Notes |
| :--- | :--- | :--- | :--- |
| `slate` | **Slate (Default)** | Dark | Default deep slate palette (`#020617` / `#0f172a`) |
| `midnight` | **Midnight Blue** | Dark | Deep navy with soft blue highlights |
| `nord` | **Nord Frost** | Dark | Arctic ice-blue Nordic palette |
| `emerald` | **Emerald Matrix** | Dark | High-tech emerald and mint palette |
| `amber` | **Warm Amber** | Dark | Low-fatigue golden amber tones |
| `cyberpunk` | **Cyberpunk Neon** | Dark | High-contrast neon cyan and magenta accents |
| `vscode-dark-plus` | **VS Code Dark+** | Dark | Classic VS Code dark UI theme |
| `vscode-light-plus` | **VS Code Light+** | Light | Classic VS Code light UI theme |
| `vscode-monokai` | **VS Code Monokai** | Dark | Monokai code palette with warm accents |
| `vscode-dracula` | **VS Code Dracula** | Dark | Gothic purple and vibrant pastel accents |
| `visual-studio-dark` | **Visual Studio Dark** | Dark | Visual Studio IDE charcoal palette |
| `visual-studio-light` | **Visual Studio Light** | Light | Classic Visual Studio light gray/blue palette |
| `visual-studio-blue` | **Visual Studio Blue** | Custom | Iconic Visual Studio blue IDE theme |
| `soft-white` | **Soft White (Blue & Black)** | Light | Gentle soft white background with deep black typography and vibrant blue accents |
| `light` | **Clean Light** | Light | Inverted high-contrast light theme |

---

## 3. Theme Implementation Approaches

There are two approaches to theming in Flow Builder:

### Approach A: CSS Filter / Variable Theming (Quick)
Useful for subtle tone shifts or dark variants where the default dark structure remains intact:
```css
html[data-theme="nord"] {
    filter: hue-rotate(190deg) brightness(0.95) saturate(1.1);
}
```

### Approach B: Explicit Tailwind Utility Overrides (Recommended for True Themes)
Required for light themes (such as **Soft White**) or distinct custom color schemes (such as **Visual Studio Blue**). This explicitly overrides Tailwind classes used across the studio:
- `.bg-slate-950`: Canvas background, deep containers, inputs, terminal blocks.
- `.bg-slate-900`: Sidebars, header, modals, section containers.
- `.node-card`: Leaf steps and container cards on the canvas.
- `.bg-slate-800`, `.bg-slate-700`: Buttons, subtle tags, inactive tab pills.
- `.border-slate-800`, `.border-slate-700`: Structural borders and divider lines.
- `.text-slate-100`, `.text-slate-200`, `.text-white`: Primary text, titles, headings.
- `.text-slate-400`, `.text-slate-500`: Secondary text, metadata, descriptions.
- `.text-cyan-400`, `.text-blue-400`: Accents, step tags, highlights, links.
- `.bg-blue-600`: Primary action buttons and active tabs.

---

## 4. Step-by-Step: Adding a New Theme

### Step 1: Add Theme Root Declaration
In `<style>` in both `builder/html_content.go` and `builder/tmpl/index.html`:
```css
html[data-theme="my-custom-theme"] {
    --bg-body: #f8fafc;
    filter: none;
}
```

### Step 2: Add CSS Overrides
Target the core layout classes for your theme:
```css
/* Custom Theme: Soft White with Blue & Black */
html[data-theme="my-custom-theme"],
html[data-theme="my-custom-theme"] body {
    background-color: #f8fafc !important;
    color: #0f172a !important;
}
html[data-theme="my-custom-theme"] .bg-slate-950 {
    background-color: #f8fafc !important;
}
html[data-theme="my-custom-theme"] .bg-slate-900,
html[data-theme="my-custom-theme"] header,
html[data-theme="my-custom-theme"] aside {
    background-color: #ffffff !important;
}
html[data-theme="my-custom-theme"] .node-card,
html[data-theme="my-custom-theme"] .component-card {
    background-color: #ffffff !important;
}
html[data-theme="my-custom-theme"] .component-card:hover,
html[data-theme="my-custom-theme"] .node-card:hover {
    background-color: #f8fafc !important;
    border-color: #2563eb !important;
}
html[data-theme="my-custom-theme"] .border-slate-800,
html[data-theme="my-custom-theme"] .border-slate-700 {
    border-color: #cbd5e1 !important;
}
html[data-theme="my-custom-theme"] .text-slate-100,
html[data-theme="my-custom-theme"] .text-slate-200,
html[data-theme="my-custom-theme"] .text-white {
    color: #0f172a !important;
}
html[data-theme="my-custom-theme"] .text-slate-400,
html[data-theme="my-custom-theme"] .text-slate-500 {
    color: #334155 !important;
}
html[data-theme="my-custom-theme"] .text-cyan-400,
html[data-theme="my-custom-theme"] .text-blue-400 {
    color: #1d4ed8 !important;
}
html[data-theme="my-custom-theme"] .bg-blue-600 {
    background-color: #1d4ed8 !important;
    color: #ffffff !important;
}
```

### Step 3: Add the Option to the Theme Selector
Add your `<option>` inside `<select id="theme-selector">` in both files:
```html
<option value="my-custom-theme" class="bg-slate-900 text-slate-200">My Custom Theme</option>
```

### Step 4: Add Automated Unit Tests
Add assertions in `builder/builder_test.go` to ensure that templates and the live HTTP handler contain the new theme:
```go
func TestMyCustomTheme(t *testing.T) {
    requiredSnippets := []string{
        `html[data-theme="my-custom-theme"]`,
        `<option value="my-custom-theme"`,
        `My Custom Theme`,
    }
    // verify tmpl/index.html, IndexHTML, and http GET "/"
}
```

### Step 5: Verify
Run the test suite:
```bash
go test ./builder -v
```

---

## 5. Reference: The Soft White Theme

The **Soft White (Blue & Black)** theme (`soft-white`) serves as a prime reference implementation:

- **Canvas Background**: `#f8fafc` (Slate 50 soft white).
- **Cards & Surfaces**: `#ffffff` (Clean white) with `#cbd5e1` crisp borders.
- **Primary Typography**: Deep black (`#0f172a`) and dark slate (`#334155`).
- **Primary Accents & Actions**: Royal blue (`#1d4ed8` / `#2563eb`).
- **Inputs & Terminal**: Pure white backgrounds with dark black text and blue focus rings.
