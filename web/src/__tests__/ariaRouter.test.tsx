import { describe, it, expect, vi } from "vitest";
import type { ReactElement } from "react";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { render } from "@testing-library/react";
import { RouterProvider as AriaRouterProvider } from "react-aria-components";
import { Link as HeroLink } from "@heroui/react";
import { createAriaNavigate, createAriaUseHref } from "@/lib/ariaRouter";

describe("aria router integration", () => {
  it("HeroUI Link works with aria router provider", async () => {
    const navigateMock = vi.fn();
    const mockRouter = {
      navigate: navigateMock,
      buildLocation: (options: { to: string }) => ({ href: options.to }),
    };

    const TestComponent = (): ReactElement => (
      <div>
        <HeroLink href="/target" data-testid="hero-link">
          Navigate
        </HeroLink>
      </div>
    );

    render(
      <AriaRouterProvider
        navigate={createAriaNavigate(mockRouter)}
        useHref={createAriaUseHref(mockRouter)}
      >
        <TestComponent />
      </AriaRouterProvider>,
    );

    const link = screen.getByTestId("hero-link");
    expect(link).toHaveAttribute("href", "/target");

    await userEvent.click(link);

    // Verify navigate was called with the target href
    expect(navigateMock).toHaveBeenCalled();
    expect(navigateMock.mock.calls[0][0]).toEqual({ to: "/target", replace: undefined });
  });

  it("useHref builds href correctly for breadcrumbs and links", async () => {
    const navigateMock = vi.fn();
    const mockRouter = {
      navigate: navigateMock,
      buildLocation: (options: { to: string }) => ({ href: `/app${options.to}` }),
    };

    const TestComponent = (): ReactElement => (
      <div>
        <HeroLink href="/path1" data-testid="link1">
          Link 1
        </HeroLink>
        <HeroLink href="/path2" data-testid="link2">
          Link 2
        </HeroLink>
      </div>
    );

    render(
      <AriaRouterProvider
        navigate={createAriaNavigate(mockRouter)}
        useHref={createAriaUseHref(mockRouter)}
      >
        <TestComponent />
      </AriaRouterProvider>,
    );

    const link1 = screen.getByTestId("link1");
    const link2 = screen.getByTestId("link2");

    // Links should have their hrefs from the useHref function
    expect(link1).toHaveAttribute("href", "/app/path1");
    expect(link2).toHaveAttribute("href", "/app/path2");

    await userEvent.click(link1);
    expect(navigateMock.mock.calls[0][0]).toEqual({ to: "/path1", replace: undefined });

    await userEvent.click(link2);
    expect(navigateMock.mock.calls[1][0]).toEqual({ to: "/path2", replace: undefined });
  });

  it("breadcrumb items navigate via aria router provider", async () => {
    const navigateMock = vi.fn();
    const mockRouter = {
      navigate: navigateMock,
      buildLocation: (options: { to: string }) => ({ href: options.to }),
    };

    const Breadcrumbs = (): ReactElement => (
      <nav aria-label="Breadcrumb">
        <HeroLink href="/" data-testid="home-link">
          Home
        </HeroLink>
        <HeroLink href="/servers" data-testid="servers-link">
          Servers
        </HeroLink>
      </nav>
    );

    render(
      <AriaRouterProvider
        navigate={createAriaNavigate(mockRouter)}
        useHref={createAriaUseHref(mockRouter)}
      >
        <Breadcrumbs />
      </AriaRouterProvider>,
    );

    const homeLink = screen.getByTestId("home-link");
    const serversLink = screen.getByTestId("servers-link");

    await userEvent.click(homeLink);
    expect(navigateMock.mock.calls[0][0]).toEqual({ to: "/", replace: undefined });

    await userEvent.click(serversLink);
    expect(navigateMock.mock.calls[1][0]).toEqual({ to: "/servers", replace: undefined });
  });
});
