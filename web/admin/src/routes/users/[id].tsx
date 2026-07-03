import {
  formatDate,
  LoadingSpinner,
  useAdminUser,
  useAssignCustomRole,
  useCustomRoles,
} from "@remnacore/shared";
import { Link, useParams } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { type ReactNode, useState } from "react";
import { useTranslation } from "react-i18next";
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
              <span className="tabular-nums">
                {formatDate(user.created_at)}
              </span>
            </Row>
            <Row label={t("common.updatedAt")}>
              <span className="tabular-nums">
                {formatDate(user.updated_at)}
              </span>
            </Row>
          </div>
        </Panel>
      </div>

      <AssignCustomRolePanel userId={user.id} />
    </div>
  );
}

function AssignCustomRolePanel({ userId }: { userId: string }) {
  const { t } = useTranslation();
  const { data: roles } = useCustomRoles();
  const assign = useAssignCustomRole(userId);
  const [roleId, setRoleId] = useState("");

  const selectClassName =
    "w-full border border-line bg-input px-3 py-2.5 text-[12px] text-t2 outline-none focus:border-accent/45";

  return (
    <Panel>
      <PanelHeader title={t("admin.users.assignRole")} />
      <div className="flex items-end gap-3 p-4">
        <div className="min-w-0 flex-1">
          <div className="mb-2 text-[9px] uppercase tracking-[1.5px] text-t6">
            {t("admin.roles.title")}
          </div>
          <select
            value={roleId}
            onChange={(e) => setRoleId(e.target.value)}
            className={selectClassName}
          >
            <option value="">{t("admin.users.selectRole")}</option>
            {(roles ?? []).map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
                {r.scope_kind === "shop" ? " (shop)" : ""}
              </option>
            ))}
          </select>
        </div>
        <TermButton
          type="button"
          variant="primary"
          disabled={!roleId || assign.isPending}
          onClick={() => {
            const role = roles?.find((r) => r.id === roleId);
            assign.mutate(
              { role_id: roleId, tenant_id: role?.tenant_id ?? null },
              { onSuccess: () => setRoleId("") },
            );
          }}
        >
          {t("admin.users.assign")}
        </TermButton>
      </div>
      {assign.isSuccess && (
        <p className="px-4 pb-3 text-[10px] uppercase tracking-[1px] text-accent">
          {t("admin.users.roleAssigned")}
        </p>
      )}
    </Panel>
  );
}
