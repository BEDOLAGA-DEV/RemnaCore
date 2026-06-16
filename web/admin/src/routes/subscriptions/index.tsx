import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { useAdminSubscriptions, formatDate } from "@remnacore/shared";
import type { Subscription, SubscriptionStatus } from "@remnacore/shared";
import {
  PageHeader,
  DataTable,
  type Column,
  StatusPill,
  type Tone,
  TermButton,
} from "@/components/ui";

function statusTone(status: SubscriptionStatus): Tone {
  const tones: Record<SubscriptionStatus, Tone> = {
    active: "ok",
    pending: "warn",
    paused: "warn",
    cancelled: "danger",
    expired: "danger",
  };
  return tones[status];
}

export function AdminSubscriptionsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [pagination, setPagination] = useState({ limit: 50, offset: 0 });
  const { data: subs, isLoading } = useAdminSubscriptions(pagination);

  const columns: Column<Subscription>[] = [
    {
      key: "id",
      header: "ID",
      render: (s) => (
        <span className="tabular-nums text-accent">{s.id.slice(0, 8)}</span>
      ),
    },
    {
      key: "user",
      header: t("admin.subscriptions.userId"),
      render: (s) => (
        <span className="tabular-nums text-t2">{s.user_id.slice(0, 8)}</span>
      ),
    },
    {
      key: "plan",
      header: t("admin.subscriptions.planId"),
      render: (s) => (
        <span className="tabular-nums tracking-[0.5px] text-t5">
          {s.plan_id.slice(0, 8)}
        </span>
      ),
    },
    {
      key: "status",
      header: t("common.status"),
      render: (s) => (
        <StatusPill label={s.status.toUpperCase()} tone={statusTone(s.status)} />
      ),
    },
    {
      key: "renews",
      header: t("subscriptions.periodEnd"),
      render: (s) => (
        <span className="tabular-nums text-t5">{formatDate(s.period_end)}</span>
      ),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      render: () => <span className="text-t7">⋯</span>,
    },
  ];

  return (
    <div className="space-y-3.5">
      <PageHeader
        title="SUBSCRIPTIONS"
        breadcrumb="REMNAWAVE PROVIDER / BILLING / RECURRING"
      />

      <DataTable<Subscription>
        columns={columns}
        rows={isLoading ? [] : (subs ?? [])}
        cols=".9fr 1.1fr 1.1fr 1fr 1.1fr 40px"
        rowKey={(s) => s.id}
        onRowClick={(s) =>
          navigate({ to: "/subscriptions/$id", params: { id: s.id } })
        }
        empty={isLoading ? t("common.loading").toUpperCase() : "NO SUBSCRIPTIONS"}
      />

      <div className="flex items-center justify-end gap-2">
        <TermButton
          type="button"
          variant="ghost"
          onClick={() =>
            setPagination((p) => ({
              ...p,
              offset: Math.max(0, p.offset - p.limit),
            }))
          }
          disabled={pagination.offset === 0}
        >
          {t("common.back")}
        </TermButton>
        <TermButton
          type="button"
          variant="ghost"
          onClick={() =>
            setPagination((p) => ({ ...p, offset: p.offset + p.limit }))
          }
          disabled={(subs?.length ?? 0) < pagination.limit}
        >
          {t("common.next")}
        </TermButton>
      </div>
    </div>
  );
}
