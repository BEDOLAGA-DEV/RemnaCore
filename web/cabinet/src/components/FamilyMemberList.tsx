import { useTranslation } from "react-i18next";
import { Trash2, Crown, User } from "lucide-react";
import type { FamilyMember } from "@remnacore/shared";

type FamilyMemberListProps = {
  members: FamilyMember[];
  onRemove: (userId: string) => void;
  isRemoving: boolean;
  currentUserId: string;
};

export function FamilyMemberList({
  members,
  onRemove,
  isRemoving,
  currentUserId,
}: FamilyMemberListProps) {
  const { t } = useTranslation();

  if (members.length === 0) {
    return null;
  }

  return (
    <div className="flex flex-col gap-2">
      {members.map((member) => {
        const isOwner = member.role === "owner";
        const isSelf = member.user_id === currentUserId;

        return (
          <div
            key={member.id}
            className="flex items-center justify-between rounded-lg border border-border bg-card p-3"
          >
            <div className="flex items-center gap-3">
              <div className="flex h-8 w-8 items-center justify-center rounded-full bg-muted">
                {isOwner ? (
                  <Crown size={14} className="text-primary" />
                ) : (
                  <User size={14} className="text-muted-foreground" />
                )}
              </div>
              <div>
                <span className="text-sm font-medium text-foreground">
                  {member.nickname ?? member.user_id.slice(0, 8)}
                </span>
                <span className="ml-2 text-xs text-muted-foreground">
                  {isOwner ? t("family.owner") : t("family.member")}
                </span>
              </div>
            </div>
            {!isOwner && !isSelf && (
              <button
                type="button"
                onClick={() => onRemove(member.user_id)}
                disabled={isRemoving}
                className="rounded-lg p-2 text-muted-foreground hover:bg-destructive/10 hover:text-destructive transition-colors disabled:opacity-50"
                aria-label={t("family.removeMember")}
              >
                <Trash2 size={14} />
              </button>
            )}
          </div>
        );
      })}
    </div>
  );
}
