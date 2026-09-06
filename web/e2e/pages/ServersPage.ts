import type { Locator, Page } from "@playwright/test";

// Page Object for /servers. Encapsulates interactions with the servers list page:
// table navigation, filtering, status tabs, search, etc.
export class ServersPage {
  readonly page: Page;
  readonly serversTable: Locator;
  readonly statusTabs: Locator;
  readonly searchInput: Locator;
  readonly filterButton: Locator;
  readonly createServerButton: Locator;

  constructor(page: Page) {
    this.page = page;
    // Table with aria-label="Server list"
    this.serversTable = page.getByRole("table", { name: /Server list/i });

    // Status filter tabs: All, Running, Stopped
    this.statusTabs = page.getByRole("tablist", { name: /Server status filter/i });

    // Search input with aria-label="Search servers"
    this.searchInput = page.getByRole("textbox", { name: /Search servers/i });

    // Filter button
    this.filterButton = page.getByRole("button", { name: /^filter$/i }).first();

    // Create server button
    this.createServerButton = page.getByRole("button", { name: /Create server/i });
  }

  async goto(): Promise<void> {
    await this.page.goto("/servers");
  }

  // Click a status filter tab by name (e.g., "All", "Running", "Stopped")
  async clickStatusTab(tabName: string): Promise<void> {
    await this.statusTabs.getByRole("tab", { name: new RegExp(`^${tabName}`, "i") }).click();
  }

  // Search for servers by name
  async search(query: string): Promise<void> {
    await this.searchInput.fill(query);
  }

  // Get a specific server row by name
  getServerRow(serverName: string): Locator {
    return this.serversTable.getByRole("row").filter({ hasText: serverName });
  }

  // Get server name link in a row
  getServerLink(serverName: string): Locator {
    return this.getServerRow(serverName).getByRole("link", { name: serverName });
  }
}
