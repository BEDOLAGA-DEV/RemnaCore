import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { useAdminUser, LoadingSpinner, formatDate } from "@remnacore/shared";
import {
  PageHeader,
  Panel,
  PanelHeader,
  StatusPill,
  TermButton,
} from "@/components/ui";

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

export function UserDetailPage() {
  const { t } = useTranslation();
  const { id } = useParams({ strict: false }) as { id: string };
  const { data: user, isLoading } = useAdminUser(id);

  if (isLoading) return <LoadingSpinner />;

  if (!user) {
    return (
      <div className="py-12 text-center text-[11px] uppercase tracking-[1px] text-danger">
        {t("common.error")}
      </div>
    );
  }

  return (
    <div className="space-y-3.5">
      <PageHeader
        title={user.email}
        breadcrumb="REMNAWAVE PROVIDER / CUSTOMERS / DETAIL"
        right={
          <Link to="/users">
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
            <Row label={t("common.email")}>
              <span className="text-t2">{user.email}</span>
            </Row>
            <Row label={t("admin.users.emailVerified")}>
              {user.email_verified ? (
                <StatusPill label="VERIFIED" tone="ok" />
              ) : (
                <StatusPill label="UNVERIFIED" tone="muted" />
              )}
            </Row>
            <Row label={t("admin.users.role")}>
              <span className="uppercase tracking-[0.5px] text-t4">
                {user.role}
              </span>
            </Row>
            <Row label={t("profile.displayName")}>
              {user.display_name ?? "—"}
            </Row>
            {user.telegram_id != null && (
              <Row label="TELEGRAM ID">
                <span className="tabular-nums">{user.telegram_id}</span>
              </Row>
            )}
          </div>
        </Panel>

        <Panel>
          <PanelHeader title="METADATA" />
          <div>
            <Row label="ID">
              <span className="font-mono text-[11px] tabular-nums text-t5">
                {user.id}
              </span>
            </Row>
            <Row label={t("common.createdAt")}>
              <span className="tabular-nums">{formatDate(user.created_at)}</span>
            </Row>
            <Row label={t("common.updatedAt")}>
              <span className="tabular-nums">{formatDate(user.updated_at)}</span>
            </Row>
          </div>
        </Panel>
      </div>
    </div>
  );
}
