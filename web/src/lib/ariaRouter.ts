/**
 * AriaRouterProvider adapters for TanStack Router.
 * These functions bridge react-aria-components' RouterProvider to the app's TanStack Router instance.
 */

interface TanStackRouter {
  navigate: (options: { to: string; replace?: boolean }) => Promise<void>;
  buildLocation: (options: { to: string }) => { href: string };
}

/**
 * Creates a navigate callback for AriaRouterProvider.
 * @param router The TanStack Router instance
 * @returns A navigate function compatible with AriaRouterProvider
 */
export function createAriaNavigate(router: TanStackRouter) {
  return (to: string, options?: { replace?: boolean }) => {
    void router.navigate({ to, replace: options?.replace });
  };
}

/**
 * Creates a useHref callback for AriaRouterProvider.
 * @param router The TanStack Router instance
 * @returns A useHref function compatible with AriaRouterProvider
 */
export function createAriaUseHref(router: TanStackRouter) {
  return (to: string) => router.buildLocation({ to }).href;
}
