import { useTranslation } from "react-i18next";
import { Link, useParams } from "@tanstack/react-router";
import { ArrowLeft, Mail, Shield, Calendar } from "lucide-react";
import { useAdminUser, LoadingSpinner, formatDate, cn } from "@remnacore/shared";

export function UserDetailPage() {
  const { t } = useTranslation();
  const { id } = useParams({ strict: false }) as { id: string };
  const { data: user, isLoading } = useAdminUser(id);

  if (isLoading) return <LoadingSpinner />;

  if (!user) {
    return (
      <div className="text-center py-12">
        <p className="text-destructive">{t("common.error")}</p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <Link
        to="/users"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} />
        {t("common.back")}
      </Link>

      <div className="rounded-xl border border-border bg-card p-6">
        <h1 className="text-xl font-bold text-foreground">{user.email}</h1>

        <div className="mt-6 grid gap-4 sm:grid-cols-2">
          <div className="flex items-center gap-3">
            <Mail size={16} className="text-muted-foreground" />
            <div>
              <p className="text-xs text-muted-foreground">
                {t("admin.users.emailVerified")}
              </p>
              <p
                className={cn(
                  "text-sm font-medium",
                  user.email_verified ? "text-green-500" : "text-red-500",
                )}
              >
                {user.email_verified ? t("common.yes") : t("common.no")}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <Shield size={16} className="text-muted-foreground" />
            <div>
              <p className="text-xs text-muted-foreground">
                {t("admin.users.role")}
              </p>
              <p className="text-sm font-medium text-foreground">
                {user.role}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <Calendar size={16} className="text-muted-foreground" />
            <div>
              <p className="text-xs text-muted-foreground">
                {t("common.createdAt")}
              </p>
              <p className="text-sm font-medium text-foreground">
                {formatDate(user.created_at)}
              </p>
            </div>
          </div>

          {user.display_name && (
            <div>
              <p className="text-xs text-muted-foreground">
                {t("profile.displayName")}
              </p>
              <p className="text-sm font-medium text-foreground">
                {user.display_name}
              </p>
            </div>
          )}
        </div>

        <div className="mt-6">
          <p className="text-xs text-muted-foreground">ID</p>
          <p className="font-mono text-sm text-foreground">{user.id}</p>
        </div>
      </div>
    </div>
  );
}
