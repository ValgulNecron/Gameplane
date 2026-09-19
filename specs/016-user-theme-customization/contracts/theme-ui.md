# Contract: Theme Customization UI

**Feature**: `016-user-theme-customization`  
**Binding Modules**: `web/src/components/hero/ThemeSettingsModal.tsx`, `web/src/components/hero/TopBar.tsx`, `web/src/components/hero/Sidebar.tsx`  
**Status**: Binding  

---

## 1. Triggers & Navigation

### 1.1 TopBar User Menu

The user avatar dropdown in `TopBar.tsx` gains a dedicated item:
- **Label**: `"Theme & Appearance"`
- **Icon**: `Palette` (from `lucide-react`)
- **Action**: Opens `ThemeSettingsModal`

```text
+------------------------------+
| [Avatar] Alex Rivera         |
| operator                     |
|------------------------------|
| [Palette] Theme & Appearance |
| [LogOut]  Sign out           |
+------------------------------+
```

### 1.2 Sidebar Footer Appearance Section

The `Sidebar.tsx` footer appearance row is enhanced:
- Retains the quick light/dark/system mode toggle (`AppearanceToggle`).
- Adds a small settings icon button (`Palette` or `Sliders`) with `aria-label="Customize theme"` that also opens `ThemeSettingsModal`.

---

## 2. Theme Settings Modal Structure

Composed entirely from HeroUI primitives (`Modal`, `ModalHeader`, `ModalBody`, `ModalFooter`, `Tabs`, `Tab`, `Button`, `RadioGroup`, `Radio`, `Input`, `Textarea`, `Alert`):

```text
+-------------------------------------------------------------+
| Theme & Appearance                                      [X] |
+-------------------------------------------------------------+
| [ Presets ]  [ Custom Colors ]  [ Custom CSS ]              |
|-------------------------------------------------------------|
| Choose a preset theme:                                      |
|                                                             |
| +-------------------------+     +-------------------------+ |
| | (o) Modern Pink         |     | ( ) Legacy Orange       | |
| | [Pink Swatch]           |     | [Orange Swatch]         | |
| | Modern HeroUI brand     |     | Original Gameplane      | |
| +-------------------------+     +-------------------------+ |
|                                                             |
| Appearance Mode:                                            |
| [ ( ) Light  (o) Dark  ( ) System ]                         |
+-------------------------------------------------------------+
| [Reset to Defaults]                       [Cancel]  [Save]  |
+-------------------------------------------------------------+
```

---

## 3. Tabs & Controls

### 3.1 Tab 1: Presets (`preset`)

- Shows two interactive radio cards:
  - **Modern Pink** (`presetId: "pink"`): Shows pink accent swatch (`#FF4FA3`) and dark preview swatch (`#1C1A20`).
  - **Legacy Orange** (`presetId: "legacy"`): Shows orange accent swatch (`#F97316`) and classic dark preview swatch (`#171717`).
- Appearance mode selector: Segmented control for Light / Dark / System.

### 3.2 Tab 2: Custom Colors (`custom_colors`)

- **Primary Accent**:
  - Color picker input or palette swatches (Blue, Emerald, Purple, Amber, Cyan, Rose, Orange).
  - Preview chip showing button with accent color and computed contrast text.
- **Surface Tone**:
  - Dropdown or Radio cards: `"Dark Slate"`, `"Midnight"`, `"Charcoal"`, `"Crisp Light"`.
- Live preview: All dashboard elements underneath the modal immediately show the updated colors.

### 3.3 Tab 3: Custom CSS (`custom_css`)

- Textarea code input with monospace font (`font-mono`, `Geist Mono` or `JetBrains Mono`).
- Placeholder text showing examples:
  ```css
  /* Example: customize typography or borders */
  :root {
    --radius: 14px;
  }
  ```
- Character count indicator (`X / 65,536`).
- Validation warning if forbidden tags (e.g. `<script>`) are detected.
- Warning alert: *"Custom CSS modifies application appearance directly. If an error occurs, use the Safe Mode URL parameter `?safe-theme=1` to recover."*

---

## 4. Safe Mode UI Banner

When the app is loaded with `?safe-theme=1` or when Safe Mode is manually toggled:
- A dismissible top alert banner appears:
  ```text
  [Alert Icon] Safe Mode Active: Custom CSS is disabled. 
               [Open Appearance Settings] | [Dismiss]
  ```
- Custom CSS is completely disabled in the DOM.
- Allows user to edit or clear their custom stylesheet without being locked out.
