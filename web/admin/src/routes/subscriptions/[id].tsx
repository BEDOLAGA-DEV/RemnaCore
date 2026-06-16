import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { useSubscription, LoadingSpinner, formatDate } from "@remnacore/shared";
import type { SubscriptionStatus } from "@remnacore/shared";
import {
  PageHeader,
  Panel,
  PanelHeader,
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

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-baseline justify-between gap-4 border-b border-line px-4 py-3 last:border-b-0">
      <span className="text-[9px] uppercase tracking-[1.5px] text-t7">
        {label}
      </span>
      <span className="text-right text-[12px] text-t2">{children}</span>
    </div>
  );
}

export function AdminSubscriptionDetailPage() {
  const { t } = useTranslation();
  const { id } = useParams({ strict: false }) as { id: string };
  const { data: sub, isLoading } = useSubscription(id);

  if (isLoading) return <LoadingSpinner />;

  if (!sub) {
    return (
      <div className="py-12 text-center text-[11px] uppercase tracking-[1px] text-danger">
        {t("common.error")}
      </div>
    );
  }

  return (
    <div className="space-y-3.5">
      <PageHeader
        title="SUBSCRIPTION"
        breadcrumb="REMNAWAVE PROVIDER / BILLING / RECURRING / DETAIL"
        right={
          <Link to="/subscriptions">
            <TermButton type="button" variant="ghost">
              <ArrowLeft size={14} />
              {t("common.back")}
            </TermButton>
          </Link>
        }
      />

      <div className="grid gap-3.5 md:grid-cols-2">
        <Panel>
          <PanelHeader title={t("common.details")} />
          <div>
            <Row label={t("common.status")}>
              <StatusPill
                label={sub.status.toUpperCase()}
                tone={statusTone(sub.status)}
              />
            </Row>
            <Row label={t("admin.subscriptions.userId")}>
              <Link
                to="/users/$id"
                params={{ id: sub.user_id }}
                className="tabular-nums text-accent transition-colors hover:opacity-80"
              >
                {sub.user_id}
              </Link>
            </Row>
            <Row label={t("admin.subscriptions.planId")}>
              <span className="tabular-nums text-t2">{sub.plan_id}</span>
            </Row>
            <Row label="INTERVAL">
              <span className="uppercase tracking-[0.5px] text-t4">
                {sub.period_interval}
              </span>
            </Row>
            <Row label="ADDONS">
              <span className="tabular-nums text-t4">
                {sub.addon_ids.length}
              </span>
            </Row>
          </div>
        </Panel>

        <Panel>
          <PanelHeader title="METADATA" />
          <div>
            <Row label="ID">
              <span className="font-mono text-[11px] tabular-nums text-t5">
                {sub.id}
              </span>
            </Row>
            <Row label="PERIOD START">
              <span className="tabular-nums">
                {formatDate(sub.period_start)}
              </span>
            </Row>
            <Row label={t("subscriptions.periodEnd")}>
              <span className="tabular-nums">{formatDate(sub.period_end)}</span>
            </Row>
            <Row label={t("common.createdAt")}>
              <span className="tabular-nums">{formatDate(sub.created_at)}</span>
            </Row>
            <Row label={t("common.updatedAt")}>
              <span className="tabular-nums">{formatDate(sub.updated_at)}</span>
            </Row>
          </div>
        </Panel>
      </div>
    </div>
  );
}
