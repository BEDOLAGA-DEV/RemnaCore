import { useTranslation } from "react-i18next";
import { Link, useParams } from "@tanstack/react-router";
import { ArrowLeft, Mail, Shield, Calendar, User } from "lucide-react";
import { useAdminUser, LoadingSpinner, formatDate, cn } from "@remnacore/shared";

export function UserDetailPage() {
  const { t } = useTranslation();
  const { id } = useParams({ strict: false }) as { id: string };
  const { data: user, isLoading } = useAdminUser(id);

  if (isLoading) return <LoadingSpinner />;

  if (!user) {
    return (
      <div className="text-center py-12">
        <p className="text-[12px] text-red-500">{t("common.error")}</p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <Link
        to="/users"
        className="text-[13px] text-muted-foreground hover:text-foreground transition-colors inline-flex items-center gap-1.5"
      >
        <ArrowLeft size={14} />
        {t("common.back")}
      </Link>

      <div className="rounded-xl border border-border bg-card p-6">
        <h1 className="text-[18px] font-semibold text-foreground">
          {user.email}
        </h1>

        <div className="mt-6 grid gap-5 sm:grid-cols-2">
          <div className="flex items-center gap-3">
            <Mail size={16} className="shrink-0 text-muted-foreground" />
            <div>
              <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">
                {t("admin.users.emailVerified")}
              </p>
              <p
                className={cn(
                  "font-mono text-[13px] font-medium",
                  user.email_verified ? "text-green-500" : "text-red-500",
                )}
              >
                {user.email_verified ? t("common.yes") : t("common.no")}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <Shield size={16} className="shrink-0 text-muted-foreground" />
            <div>
              <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">
                {t("admin.users.role")}
              </p>
              <p className="font-mono text-[13px] text-foreground">
                {user.role}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <Calendar size={16} className="shrink-0 text-muted-foreground" />
            <div>
              <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">
                {t("common.createdAt")}
              </p>
              <p className="font-mono text-[13px] text-foreground">
                {formatDate(user.created_at)}
              </p>
            </div>
          </div>

          {user.display_name && (
            <div className="flex items-center gap-3">
              <User size={16} className="shrink-0 text-muted-foreground" />
              <div>
                <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">
                  {t("profile.displayName")}
                </p>
                <p className="text-[13px] text-foreground">
                  {user.display_name}
                </p>
              </div>
            </div>
          )}
        </div>

        <div className="mt-6 rounded-lg bg-secondary p-3">
          <p className="text-[11px] uppercase tracking-wider text-muted-foreground font-medium">
            ID
          </p>
          <p className="mt-1 font-mono text-[12px] text-muted-foreground">
            {user.id}
          </p>
        </div>
      </div>
    </div>
  );
}
