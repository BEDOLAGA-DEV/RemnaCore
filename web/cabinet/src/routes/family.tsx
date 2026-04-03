import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2, Users } from "lucide-react";
import {
  useFamily,
  useAddFamilyMember,
  useRemoveFamilyMember,
  useAuthStore,
  LoadingSpinner,
} from "@remnacore/shared";
import { FamilyMemberList } from "../components/FamilyMemberList.js";

const addMemberSchema = z.object({
  subscription_id: z.string().min(1),
  member_user_id: z.string().min(1),
  nickname: z.string().optional(),
});

type AddMemberValues = z.infer<typeof addMemberSchema>;

export function FamilyPage() {
  const { t } = useTranslation();
  const { user } = useAuthStore();
  const { data: family, isLoading, isError } = useFamily();
  const addMember = useAddFamilyMember();
  const removeMember = useRemoveFamilyMember();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<AddMemberValues>({
    resolver: zodResolver(addMemberSchema),
  });

  const onSubmit = (data: AddMemberValues) => {
    addMember.mutate(data, {
      onSuccess: () => reset(),
    });
  };

  const handleRemove = (userId: string) => {
    if (!family) return;
    // The subscription_id is needed for the remove call
    removeMember.mutate({
      userId,
      subscriptionId: "", // TODO: from family group context
    });
  };

  if (isLoading) return <LoadingSpinner />;

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-foreground">
          {t("family.title")}
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {t("family.description")}
        </p>
      </div>

      {isError || !family ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
          <Users size={48} className="text-muted-foreground" />
          <p className="mt-4 text-muted-foreground">{t("family.noGroup")}</p>
        </div>
      ) : (
        <>
          <div className="rounded-xl border border-border bg-card p-5">
            <div className="flex items-center justify-between">
              <h2 className="text-lg font-semibold text-foreground">
                {t("family.title")}
              </h2>
              <span className="text-sm text-muted-foreground">
                {t("family.maxMembers", { count: family.max_members })}
              </span>
            </div>

            <div className="mt-4">
              <FamilyMemberList
                members={family.members ?? []}
                onRemove={handleRemove}
                isRemoving={removeMember.isPending}
                currentUserId={user?.id ?? ""}
              />
            </div>
          </div>

          {/* Add member form */}
          <div className="rounded-xl border border-border bg-card p-5">
            <h3 className="mb-4 text-lg font-semibold text-foreground">
              {t("family.addMember")}
            </h3>
            <form
              onSubmit={handleSubmit(onSubmit)}
              className="space-y-4"
            >
              <div>
                <label
                  htmlFor="subscription_id"
                  className="mb-1.5 block text-sm font-medium text-foreground"
                >
                  Subscription ID
                </label>
                <input
                  id="subscription_id"
                  {...register("subscription_id")}
                  className="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
                />
                {errors.subscription_id && (
                  <p className="mt-1 text-sm text-destructive">
                    {errors.subscription_id.message}
                  </p>
                )}
              </div>

              <div>
                <label
                  htmlFor="member_user_id"
                  className="mb-1.5 block text-sm font-medium text-foreground"
                >
                  {t("family.memberEmail")}
                </label>
                <input
                  id="member_user_id"
                  {...register("member_user_id")}
                  className="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
                />
                {errors.member_user_id && (
                  <p className="mt-1 text-sm text-destructive">
                    {errors.member_user_id.message}
                  </p>
                )}
              </div>

              <div>
                <label
                  htmlFor="nickname"
                  className="mb-1.5 block text-sm font-medium text-foreground"
                >
                  {t("family.nickname")}
                </label>
                <input
                  id="nickname"
                  {...register("nickname")}
                  className="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
                />
              </div>

              <button
                type="submit"
                disabled={addMember.isPending}
                className="w-full rounded-lg bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50"
              >
                {addMember.isPending ? (
                  <Loader2 size={16} className="mx-auto animate-spin" />
                ) : (
                  t("family.addMember")
                )}
              </button>
            </form>
          </div>
        </>
      )}
    </div>
  );
}
