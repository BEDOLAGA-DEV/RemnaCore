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
      <div className="animate-fade-up">
        <h1 className="text-3xl font-bold tracking-tight text-foreground">
          {t("family.title")}
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {t("family.description")}
        </p>
      </div>

      {isError || !family ? (
        <div
          className="animate-fade-up flex flex-col items-center justify-center rounded-lg border border-dashed border-border bg-card/50 p-12"
          style={{ animationDelay: "100ms", animationFillMode: "backwards" }}
        >
          <Users size={48} className="text-muted-foreground" />
          <p className="mt-4 text-muted-foreground">{t("family.noGroup")}</p>
        </div>
      ) : (
        <>
          <div
            className="animate-fade-up rounded-lg border border-border bg-card p-5"
            style={{ animationDelay: "100ms", animationFillMode: "backwards" }}
          >
            <div className="flex items-center justify-between">
              <h2 className="text-lg font-semibold tracking-tight text-foreground">
                {t("family.title")}
              </h2>
              <span className="rounded-md bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
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
          <div
            className="animate-fade-up rounded-lg border border-border bg-card p-5"
            style={{ animationDelay: "200ms", animationFillMode: "backwards" }}
          >
            <h3 className="mb-4 text-lg font-semibold tracking-tight text-foreground">
              {t("family.addMember")}
            </h3>
            <form
              onSubmit={handleSubmit(onSubmit)}
              className="space-y-4"
            >
              <div>
                <label
                  htmlFor="subscription_id"
                  className="mb-1.5 block text-xs font-medium text-foreground"
                >
                  Subscription ID
                </label>
                <input
                  id="subscription_id"
                  {...register("subscription_id")}
                  className="w-full rounded-[10px] border border-input bg-background px-3 py-2.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
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
                  className="mb-1.5 block text-xs font-medium text-foreground"
                >
                  {t("family.memberEmail")}
                </label>
                <input
                  id="member_user_id"
                  {...register("member_user_id")}
                  className="w-full rounded-[10px] border border-input bg-background px-3 py-2.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
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
                  className="mb-1.5 block text-xs font-medium text-foreground"
                >
                  {t("family.nickname")}
                </label>
                <input
                  id="nickname"
                  {...register("nickname")}
                  className="w-full rounded-[10px] border border-input bg-background px-3 py-2.5 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
                />
              </div>

              <button
                type="submit"
                disabled={addMember.isPending}
                className="w-full rounded-[10px] bg-primary py-2.5 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
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
