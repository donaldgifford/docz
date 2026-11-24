import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@donaldgifford/design-system/components/dropdown-menu";
import {
  SidebarMenuButton,
  useSidebar,
} from "@donaldgifford/design-system/components/sidebar";
import {
  AvatarMark,
  ShellSidebarIdentity,
} from "@donaldgifford/design-system/shell/sidebar";
import { initialsOf } from "@donaldgifford/design-system/shell/top-bar";
import { IconLogout } from "@tabler/icons-react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router";

import { useLogout } from "@/api/__generated__/docz-api";

import type { Session } from "@/api/__generated__/docz-api.schemas";

/*
 * The signed-in half of the session chrome (DESIGN-0028 §6): the
 * identity at the foot of the sidebar (GitHub `login`, OIDC `email`),
 * which opens the design system's DropdownMenu with sign-out. On the
 * icon rail it is the initials alone. The menu's keyboard, focus, and
 * dismissal are the package's. session-menu.tsx loads this module only
 * for a signed-in session.
 */

export function AccountMenu({ session }: { session: Session }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { isMobile } = useSidebar();
  const logout = useLogout({
    mutation: {
      // Settled, not success: even a failed POST (say the cookie
      // already expired) must not strand stale per-session data — the
      // design's invariant is "nothing fetched under the old session
      // outlives it". Navigate BEFORE clearing so the page being left
      // doesn't refetch everything into the fresh cache.
      onSettled: async () => {
        await navigate("/login");
        queryClient.clear();
      },
    },
  });

  const identity = session.login ?? session.email ?? session.subject;
  const provider = `via ${session.provider}`;

  return (
    <DropdownMenu>
      <SidebarMenuButton
        size="lg"
        aria-label={`Account: ${identity}`}
        render={<DropdownMenuTrigger />}
      >
        <ShellSidebarIdentity
          mark={<AvatarMark initials={initialsOf(session.login, identity)} />}
          title={identity}
          subtitle={provider}
        />
      </SidebarMenuButton>
      <DropdownMenuContent
        // Beside the sidebar; over it in the sheet, which has no beside.
        side={isMobile ? "top" : "right"}
        align="end"
        className="min-w-52"
      >
        <DropdownMenuGroup>
          {/* An identity is spelled as it is, so the label's capitals
              and tracking are off. */}
          <DropdownMenuLabel className="flex flex-col gap-0.5 tracking-normal normal-case">
            <span className="truncate text-sm font-medium text-foreground">
              {identity}
            </span>
            <span className="font-mono text-xs font-normal">{provider}</span>
          </DropdownMenuLabel>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          disabled={logout.isPending}
          onClick={() => {
            logout.mutate();
          }}
        >
          <IconLogout />
          Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
