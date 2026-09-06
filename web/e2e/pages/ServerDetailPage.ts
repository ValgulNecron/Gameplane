import type { Locator, Page } from "@playwright/test";

// Page Object for /servers/$name. Encapsulates common interactions on the
// ServerDetail page: navigating tabs, accessing lifecycle actions, etc.
export class ServerDetailPage {
  readonly page: Page;
  readonly tablist: Locator;
  readonly restartButton: Locator;
  readonly stopButton: Locator;
  readonly startButton: Locator;
  readonly wakeButton: Locator;
  readonly openConsoleButton: Locator;

  constructor(page: Page) {
    this.page = page;
    // Tab navigation uses Tabs component with aria-label="Server detail tabs"
    // Individual tabs are accessed via getByRole("tab", { name: ... })
    this.tablist = page.getByRole("tablist", { name: /Server detail tabs/i });

    // Header buttons for lifecycle actions (first occurrence of each)
    this.restartButton = page.getByRole("button", { name: /^restart$/i }).first();
    this.stopButton = page.getByRole("button", { name: /^stop$/i }).first();
    this.startButton = page.getByRole("button", { name: /^start$/i }).first();
    this.wakeButton = page.getByRole("button", { name: /^wake$/i }).first();
    this.openConsoleButton = page.getByRole("button", { name: /open console/i }).first();
  }

  async goto(serverName: string, ns?: string): Promise<void> {
    const url = ns ? `/servers/${serverName}?ns=${ns}` : `/servers/${serverName}`;
    await this.page.goto(url);
  }

  // Navigate to a tab by label (Overview, Events, Console, Logs, Files, Mods, Modpacks, Players, Backups, Capture, Settings)
  async clickTab(label: string): Promise<void> {
    await this.tablist.getByRole("tab", { name: new RegExp(`^${label}$`, "i") }).click();
  }

  // Lifecycle action helpers
  async clickRestart(): Promise<void> {
    await this.restartButton.click();
  }

  async clickStop(): Promise<void> {
    await this.stopButton.click();
  }

  async clickStart(): Promise<void> {
    await this.startButton.click();
  }

  async clickWake(): Promise<void> {
    await this.wakeButton.click();
  }

  async clickOpenConsole(): Promise<void> {
    await this.openConsoleButton.click();
  }
}
