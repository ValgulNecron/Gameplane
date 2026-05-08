import { setupServer } from "msw/node";
import { handlers } from "./handlers";

// Single shared MSW node server — created once and managed via the
// hooks in setup.ts so individual test files don't have to wire it up.
export const server = setupServer(...handlers);
