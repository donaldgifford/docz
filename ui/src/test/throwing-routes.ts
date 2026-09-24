import type { RouteObject } from "react-router";

import { routes } from "@/app/router";

/*
 * A copy of the real route table with one throwing route spliced in,
 * shared by the route-error suite and the axe sweep so both exercise
 * the SHIPPING boundary placement rather than a restatement of it.
 */

/** The message the throwing route throws, asserted by callers. */
export const BOOM_MESSAGE = "kaboom from a route";

/** Path the throwing route is mounted at. */
export const BOOM_PATH = "/boom";

function Boom(): never {
  throw new Error(BOOM_MESSAGE);
}

/**
 * Narrow a RouteObject to the branch that can hold children.
 *
 * RouteObject is a union and only the non-index branch has `children`,
 * so this both type-guards and asserts the shape the boundary depends
 * on — if someone flattens the pathless boundary route away, these
 * tests fail loudly instead of silently covering nothing.
 */
function withChildren(
  route: RouteObject | undefined,
  what: string,
): RouteObject & { children: RouteObject[] } {
  if (route === undefined || route.index === true) {
    throw new Error(`expected ${what} to be a non-index route`);
  }
  if (route.children === undefined) {
    throw new Error(`expected ${what} to have children`);
  }
  return { ...route, children: route.children };
}

export function routesWithThrow(): RouteObject[] {
  const root = withChildren(routes[0], "the root route");
  const boundary = withChildren(
    root.children[0],
    "the pathless boundary route under AppShell",
  );
  return [
    {
      ...root,
      children: [
        {
          ...boundary,
          children: [
            { path: BOOM_PATH.slice(1), Component: Boom },
            ...boundary.children,
          ],
        },
      ],
    },
  ];
}
