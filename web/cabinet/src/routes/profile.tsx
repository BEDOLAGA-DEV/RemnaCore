import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2, Check, Unlink } from "lucide-react";
import {
  useMe,
  useUpdateProfile,
  useLinkTelegram,
  useUnlinkTelegram,
  LoadingSpinner,
} from "@remnacore/shared";

const profileSchema = z.object({
  display_name: z.string().min(1),
});

type ProfileFormValues = z.infer<typeof profileSchema>;

const telegramSchema = z.object({
  telegram_id: z.coerce.number().int().positive(),
});

type TelegramFormValues = z.infer<typeof telegramSchema>;

export function ProfilePage() {
  const { t } = useTranslation();
  const { data: user, isLoading } = useMe();
  const updateProfile = useUpdateProfile();
  const linkTelegram = useLinkTelegram();
  const unlinkTelegram = useUnlinkTelegram();

  const {
    register: registerProfile,
    handleSubmit: handleProfileSubmit,
    formState: { errors: profileErrors },
  } = useForm<ProfileFormValues>({
    resolver: zodResolver(profileSchema),
    values: {
      display_name: user?.display_name ?? "",
    },
  });

  const {
    register: registerTelegram,
    handleSubmit: handleTelegramSubmit,
    formState: { errors: telegramErrors },
  } = useForm<TelegramFormValues>({
    resolver: zodResolver(telegramSchema),
  });

  const onProfileSubmit = (data: ProfileFormValues) => {
    updateProfile.mutate(data);
  };

  const onTelegramSubmit = (data: TelegramFormValues) => {
    linkTelegram.mutate(data);
  };

  if (isLoading) return <LoadingSpinner />;

  if (!user) {
    return (
      <div className="text-center py-12">
        <p className="text-destructive">{t("common.error")}</p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-lg space-y-6">
      <h1 className="text-2xl font-bold text-foreground">
        {t("profile.title")}
      </h1>

      {/* Profile form */}
      <div className="rounded-xl border border-border bg-card p-6">
        <form
          onSubmit={handleProfileSubmit(onProfileSubmit)}
          className="space-y-4"
        >
          <div>
            <label
              htmlFor="email"
              className="mb-1.5 block text-sm font-medium text-foreground"
            >
              {t("common.email")}
            </label>
            <input
              id="email"
              type="email"
              value={user.email}
              disabled
              className="w-full rounded-lg border border-input bg-muted px-3 py-2 text-sm text-muted-foreground cursor-not-allowed"
            />
          </div>

          <div>
            <label
              htmlFor="display_name"
              className="mb-1.5 block text-sm font-medium text-foreground"
            >
              {t("profile.displayName")}
            </label>
            <input
              id="display_name"
              {...registerProfile("display_name")}
              className="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            />
            {profileErrors.display_name && (
              <p className="mt-1 text-sm text-destructive">
                {profileErrors.display_name.message}
              </p>
            )}
          </div>

          <button
            type="submit"
            disabled={updateProfile.isPending}
            className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50"
          >
            {updateProfile.isPending ? (
              <Loader2 size={14} className="animate-spin" />
            ) : (
              t("common.save")
            )}
          </button>

          {updateProfile.isSuccess && (
            <p className="flex items-center gap-1 text-sm text-green-500">
              <Check size={14} />
              {t("profile.updateSuccess")}
            </p>
          )}
        </form>
      </div>

      {/* Telegram section */}
      <div className="rounded-xl border border-border bg-card p-6">
        <h2 className="mb-4 text-lg font-semibold text-foreground">
          Telegram
        </h2>

        {user.telegram_id ? (
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-green-500">
                {t("profile.telegramLinked")}
              </p>
              <p className="mt-1 font-mono text-sm text-muted-foreground">
                ID: {user.telegram_id}
              </p>
            </div>
            <button
              type="button"
              onClick={() => unlinkTelegram.mutate()}
              disabled={unlinkTelegram.isPending}
              className="flex items-center gap-2 rounded-lg border border-destructive px-3 py-2 text-sm text-destructive hover:bg-destructive/10 transition-colors disabled:opacity-50"
            >
              <Unlink size={14} />
              {t("profile.unlinkTelegram")}
            </button>
          </div>
        ) : (
          <form
            onSubmit={handleTelegramSubmit(onTelegramSubmit)}
            className="space-y-4"
          >
            <div>
              <label
                htmlFor="telegram_id"
                className="mb-1.5 block text-sm font-medium text-foreground"
              >
                {t("profile.telegramId")}
              </label>
              <input
                id="telegram_id"
                type="number"
                {...registerTelegram("telegram_id")}
                className="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
              />
              {telegramErrors.telegram_id && (
                <p className="mt-1 text-sm text-destructive">
                  {telegramErrors.telegram_id.message}
                </p>
              )}
            </div>

            <button
              type="submit"
              disabled={linkTelegram.isPending}
              className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50"
            >
              {linkTelegram.isPending ? (
                <Loader2 size={14} className="animate-spin" />
              ) : (
                t("profile.linkTelegram")
              )}
            </button>
          </form>
        )}
      </div>
    </div>
  );
}
