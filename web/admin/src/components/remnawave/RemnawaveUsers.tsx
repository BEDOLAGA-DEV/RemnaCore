import { useCallback, useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiGet, apiPost, cn, ENDPOINTS, LoadingSpinner } from "@remnacore/shared";
import {
  Loader2,
  Power,
  PowerOff,
  RotateCcw,
  Search,
  Users,
} from "lucide-react";

// ─── Types ───────────────────────────────────────────────────────────────────

type RemnawaveUserEntry = {
  panel_id: string;
  user: {
    uuid: string;
    username: string;
    status: string;
    trafficLimitBytes: number;
    expireAt: string;
    usedTrafficBytes: number;
    subscriptionUrl: string;
    shortUuid: string;
  };
};

type ActionParams = {
  panelID: string;
  userUUID: string;
};

// ─── Constants ───────────────────────────────────────────────────────────────

const QUERY_KEY = ["remnawave", "users"] as const;

const DEBOUNCE_MS = 300;

const STATUS_COLORS: Record<string, string> = {
  active: "bg-emerald-500/10 text-emerald-500",
  disabled: "bg-red-500/10 text-red-500",
  expired: "bg-yellow-500/10 text-yellow-500",
  limited: "bg-orange-500/10 text-orange-500",
};

const TWENTY_FOUR_HOURS_MS = 24 * 60 * 60 * 1000;

// ─── Helpers ─────────────────────────────────────────────────────────────────

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const value = bytes / 1024 ** i;
  return `${value.toFixed(value < 10 ? 2 : 1)} ${units[i]}`;
}

function formatExpiry(dateStr: string): string {
  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) return "\u2014";
  return date.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function isExpiringSoon(dateStr: string): boolean {
  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) return false;
  return date.getTime() - Date.now() < TWENTY_FOUR_HOURS_MS;
}

function trafficPercentage(used: number, limit: number): number {
  if (limit <= 0) return 0;
  return Math.min(100, Math.round((used / limit) * 100));
}

// ─── Hooks ───────────────────────────────────────────────────────────────────

function useDebounce(value: string, delay: number): string {
  const [debouncedValue, setDebouncedValue] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedValue(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);

  return debouncedValue;
}

function useRemnawaveUsers(search: string) {
  const searchParams = search ? { q: search } : undefined;

  return useQuery({
    queryKey: [...QUERY_KEY, search],
    queryFn: () =>
      apiGet<RemnawaveUserEntry[]>(ENDPOINTS.remnawave.users, searchParams),
  });
}

function useUserAction(action: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ panelID, userUUID }: ActionParams) =>
      apiPost<{ success: boolean }>(
        `/api/remnawave/users/${panelID}/${userUUID}/${action}`,
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
    },
  });
}

// ─── Sub-components ──────────────────────────────────────────────────────────

function StatusBadge({ status }: { status: string }) {
  const normalized = status.toLowerCase();
  const colorClass = STATUS_COLORS[normalized] ?? "bg-muted text-muted-foreground";

  return (
    <span
      className={cn(
        "inline-block rounded-full px-2.5 py-0.5 font-mono text-[11px] font-medium",
        colorClass,
      )}
    >
      {status}
    </span>
  );
}

function TrafficCell({
  used,
  limit,
}: {
  used: number;
  limit: number;
}) {
  const pct = trafficPercentage(used, limit);

  return (
    <div className="space-y-1">
      <span className="font-mono text-[11px] text-foreground">
        {formatBytes(used)}
        {" / "}
        {limit > 0 ? formatBytes(limit) : "Unlimited"}
      </span>
      {limit > 0 && (
        <div className="h-1.5 w-20 rounded-full bg-muted">
          <div
            className="h-full rounded-full bg-primary"
            style={{ width: `${pct}%` }}
          />
        </div>
      )}
    </div>
  );
}

function ActionButton({
  onClick,
  isPending,
  icon: Icon,
  label,
  variant = "default",
}: {
  onClick: () => void;
  isPending: boolean;
  icon: typeof Power;
  label: string;
  variant?: "default" | "danger";
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={isPending}
      title={label}
      className={cn(
        "rounded-lg p-1.5 text-muted-foreground transition-colors disabled:opacity-40",
        variant === "danger"
          ? "hover:bg-red-500/10 hover:text-red-500"
          : "hover:bg-secondary hover:text-foreground",
      )}
    >
      {isPending ? (
        <Loader2 size={14} className="animate-spin" />
      ) : (
        <Icon size={14} />
      )}
    </button>
  );
}

function UserActions({ entry }: { entry: RemnawaveUserEntry }) {
  const enableMutation = useUserAction("enable");
  const disableMutation = useUserAction("disable");
  const resetTrafficMutation = useUserAction("reset-traffic");

  const params: ActionParams = {
    panelID: entry.panel_id,
    userUUID: entry.user.uuid,
  };

  const isActive = entry.user.status.toLowerCase() === "active";

  return (
    <div className="flex items-center justify-end gap-1">
      {isActive ? (
        <ActionButton
          onClick={() => disableMutation.mutate(params)}
          isPending={disableMutation.isPending}
          icon={PowerOff}
          label="Disable"
          variant="danger"
        />
      ) : (
        <ActionButton
          onClick={() => enableMutation.mutate(params)}
          isPending={enableMutation.isPending}
          icon={Power}
          label="Enable"
        />
      )}
      <ActionButton
        onClick={() => resetTrafficMutation.mutate(params)}
        isPending={resetTrafficMutation.isPending}
        icon={RotateCcw}
        label="Reset Traffic"
      />
    </div>
  );
}

// ─── Main Component ──────────────────────────────────────────────────────────

export default function RemnawaveUsers() {
  const [searchInput, setSearchInput] = useState("");
  const debouncedSearch = useDebounce(searchInput, DEBOUNCE_MS);
  const { data: users, isLoading, isError } = useRemnawaveUsers(debouncedSearch);

  const inputRef = useRef<HTMLInputElement>(null);

  const handleSearchChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      setSearchInput(e.target.value);
    },
    [],
  );

  if (isError) {
    return (
      <div className="py-12 text-center">
        <p className="text-[13px] text-destructive">
          Failed to load VPN users. Please try again.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Users size={18} className="text-muted-foreground" />
          <div>
            <h1 className="text-[15px] font-semibold text-foreground">
              VPN Users
            </h1>
            {users && (
              <p className="font-mono text-[11px] text-muted-foreground">
                {users.length} {users.length === 1 ? "user" : "users"}
              </p>
            )}
          </div>
        </div>

        {/* Search */}
        <div className="flex items-center gap-2 rounded-lg border border-border bg-background px-3 py-2">
          <Search size={14} className="text-muted-foreground" />
          <input
            ref={inputRef}
            type="text"
            placeholder="Search users..."
            value={searchInput}
            onChange={handleSearchChange}
            className="bg-transparent text-[13px] text-foreground placeholder:text-muted-foreground/50 outline-none"
          />
        </div>
      </div>

      {/* Loading */}
      {isLoading && <LoadingSpinner />}

      {/* Empty state */}
      {!isLoading && users && users.length === 0 && (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
          <Users size={32} className="mb-3 text-muted-foreground/40" />
          <p className="text-[13px] text-muted-foreground">
            {debouncedSearch
              ? `No users matching "${debouncedSearch}"`
              : "No VPN users found"}
          </p>
        </div>
      )}

      {/* Table */}
      {!isLoading && users && users.length > 0 && (
        <div className="overflow-x-auto rounded-xl border border-border">
          <table className="w-full text-left">
            <thead>
              <tr className="border-b border-border bg-muted/30">
                <th className="px-4 py-3 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                  Username
                </th>
                <th className="px-4 py-3 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                  Panel
                </th>
                <th className="px-4 py-3 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                  Status
                </th>
                <th className="px-4 py-3 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                  Traffic
                </th>
                <th className="px-4 py-3 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                  Expires
                </th>
                <th className="px-4 py-3 text-[11px] font-medium uppercase tracking-wider text-muted-foreground text-right">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {users.map((entry) => (
                <tr
                  key={`${entry.panel_id}-${entry.user.uuid}`}
                  className="transition-colors hover:bg-muted/20"
                >
                  {/* Username */}
                  <td className="px-4 py-3">
                    <span className="font-mono text-[13px] font-semibold text-foreground">
                      {entry.user.username}
                    </span>
                  </td>

                  {/* Panel ID */}
                  <td className="px-4 py-3">
                    <span className="font-mono text-[11px] text-muted-foreground">
                      {entry.panel_id}
                    </span>
                  </td>

                  {/* Status */}
                  <td className="px-4 py-3">
                    <StatusBadge status={entry.user.status} />
                  </td>

                  {/* Traffic */}
                  <td className="px-4 py-3">
                    <TrafficCell
                      used={entry.user.usedTrafficBytes}
                      limit={entry.user.trafficLimitBytes}
                    />
                  </td>

                  {/* Expires */}
                  <td className="px-4 py-3">
                    <span
                      className={cn(
                        "font-mono text-[11px]",
                        isExpiringSoon(entry.user.expireAt)
                          ? "font-medium text-red-500"
                          : "text-muted-foreground",
                      )}
                    >
                      {formatExpiry(entry.user.expireAt)}
                    </span>
                  </td>

                  {/* Actions */}
                  <td className="px-4 py-3">
                    <UserActions entry={entry} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
