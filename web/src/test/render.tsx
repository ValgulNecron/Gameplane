// Helpers that render a component under the same providers it would
// see in production (TanStack Query + a memory router from TanStack
// Router) so route-level tests don't have to repeat the wiring.

import { type ReactElement } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, type RenderOptions } from "@testing-library/react";

// A fresh client per render avoids cache leaks between tests. retry is
// off so error states surface immediately instead of silently retrying.
export function makeClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0, staleTime: 0 },
      mutations: { retry: false },
    },
  });
}

export function renderWithQuery(
  ui: ReactElement,
  options: Omit<RenderOptions, "wrapper"> & { client?: QueryClient } = {},
) {
  const { client = makeClient(), ...rest } = options;
  return {
    client,
    ...render(
      <QueryClientProvider client={client}>{ui}</QueryClientProvider>,
      rest,
    ),
  };
}
