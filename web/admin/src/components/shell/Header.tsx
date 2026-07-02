import { useAuthStore, useLogout } from "@remnacore/shared";
import { LogOut } from "lucide-react";
import { useClock } from "../../hooks/useClock.js";

function initial(user: { email: string; display_name: string | null }): string {
  const src = user.display_name?.trim() || user.email;
  return src.charAt(0).toUpperCase();
}

const ROLE_LABEL: Record<string, string> = {
  admin: "SUPER ADMIN",
  reseller: "RESELLER",
  customer: "CUSTOMER",
};

export function Header({ onOpenSearch }: { onOpenSearch: () => void }) {
  const { user } = useAuthStore();
  const logout = useLogout();
  const clock = useClock();

  return (
    <header className="sticky top-0 z-10 flex h-14 items-center justify-between border-b border-line bg-raised px-[22px]">
      <div className="flex items-center gap-3.5">
        <div className="text-[12px] tracking-[1px] text-t6">
          ADMIN<span className="text-accent">/</span>PANEL
        </div>
        <div
          className="flex items-center gap-[7px] border px-2.5 py-1 text-[10px] uppercase tracking-[1.5px] text-accent"
          style={{
            borderColor: "color-mix(in srgb,var(--accent) 40%,transparent)",
          }}
        >
          <span
            className="h-1.5 w-1.5 rounded-full bg-accent"
            style={{ boxShadow: "0 0 8px var(--accent)" }}
          />
          LIVE
        </div>
      </div>

      <div className="flex items-center gap-3">
        <button
          type="button"
          onClick={onOpenSearch}
          className="flex items-center gap-2.5 border border-line bg-input px-3 py-[7px] text-[11px] tracking-[.5px] text-t5 transition-colors hover:text-t2"
        >
          <span className="opacity-60">⌕</span> SEARCH ANYTHING
          <span className="border border-line px-1.5 py-px text-[9px] text-t3">
            ⌘K
          </span>
        </button>

        <div className="text-[12px] tabular-nums tracking-[1px] text-t4">
          {clock} <span className="text-t8">UTC</span>
        </div>

        {user && (
          <div className="flex items-center gap-2 border-l border-line pl-3">
            <div className="text-right leading-tight">
              <div className="text-[11px] text-t2">
                {user.display_name || user.email.split("@")[0]}
              </div>
              <div className="text-[9px] text-t7">
                {ROLE_LABEL[user.role] ?? user.role.toUpperCase()}
              </div>
            </div>
            <div className="flex h-[30px] w-[30px] items-center justify-center bg-accent text-[12px] font-bold text-on-accent">
              {initial(user)}
            </div>
            <button
              type="button"
              onClick={logout}
              className="ml-1 p-1.5 text-t6 transition-colors hover:text-danger"
              aria-label="Log out"
            >
              <LogOut size={15} />
            </button>
          </div>
        )}
      </div>
    </header>
  );
}
