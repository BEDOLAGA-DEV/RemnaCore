import {
  type CreateCustomRoleRequest,
  type CustomRole,
  useCreateCustomRole,
  useCustomRoles,
  useDeleteCustomRole,
  usePermissionCatalog,
  useUpdateCustomRole,
} from "@remnacore/shared";
import { Loader2, Pencil, Trash2 } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  PageHeader,
  Panel,
  PanelHeader,
  Segmented,
  StatusPill,
  TermButton,
  TermInput,
} from "@/components/ui";

export function RolesPage() {
  const { t } = useTranslation();
  const { data: roles, isLoading } = useCustomRoles();
  const deleteRole = useDeleteCustomRole();
  const [editRole, setEditRole] = useState<CustomRole | null>(null);

  return (
    <div className="space-y-6">
      <PageHeader
        title={t("admin.roles.title")}
        breadcrumb={t("admin.roles.subtitle")}
      />

      <RoleForm editRole={editRole} onDone={() => setEditRole(null)} />

      <Panel>
        <PanelHeader title={t("admin.roles.title")} />
        {isLoading ? (
          <div className="flex justify-center p-8">
            <Loader2 className="animate-spin text-t5" size={18} />
          </div>
        ) : !roles || roles.length === 0 ? (
          <p className="p-4 text-[12px] text-t5">{t("admin.roles.noRoles")}</p>
        ) : (
          <ul className="divide-y divide-line">
            {roles.map((role) => (
              <RoleRow
                key={role.id}
                role={role}
                onEdit={() => setEditRole(role)}
                onDelete={() => {
                  if (window.confirm(t("admin.roles.deleteConfirm"))) {
                    deleteRole.mutate(role.id);
                  }
                }}
                busy={deleteRole.isPending}
              />
            ))}
          </ul>
        )}
      </Panel>
    </div>
  );
}

function RoleRow({
  role,
  onEdit,
  onDelete,
  busy,
}: {
  role: CustomRole;
  onEdit: () => void;
  onDelete: () => void;
  busy: boolean;
}) {
  const { t } = useTranslation();
  return (
    <li className="flex items-start justify-between gap-4 p-4">
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-[13px] text-t2">{role.name}</span>
          <StatusPill
            label={
              role.scope_kind === "global"
                ? t("admin.roles.scopeGlobal")
                : t("admin.roles.scopeShop")
            }
            tone={role.scope_kind === "global" ? "ok" : "muted"}
          />
        </div>
        {role.description && (
          <p className="mt-1 text-[11px] text-t5">{role.description}</p>
        )}
        <p className="mt-1 text-[10px] uppercase tracking-[1px] text-t6">
          {t("admin.roles.permissionCount", { count: role.permissions.length })}
          {role.permissions.length > 0 && ` — ${role.permissions.join(", ")}`}
        </p>
      </div>
      <div className="flex shrink-0 gap-1">
        <TermButton
          variant="ghost"
          onClick={onEdit}
          disabled={busy}
          aria-label={t("common.edit")}
        >
          <Pencil size={14} />
        </TermButton>
        <TermButton
          variant="ghost"
          onClick={onDelete}
          disabled={busy}
          aria-label={t("common.delete")}
        >
          <Trash2 size={14} />
        </TermButton>
      </div>
    </li>
  );
}

function RoleForm({
  editRole,
  onDone,
}: {
  editRole: CustomRole | null;
  onDone: () => void;
}) {
  const { data: catalog } = usePermissionCatalog();
  const createRole = useCreateCustomRole();
  const updateRole = useUpdateCustomRole();

  const isEdit = !!editRole;
  // Re-key the inner form on the edited role so its useState resets cleanly.
  return (
    <RoleFormFields
      key={editRole?.id ?? "new"}
      editRole={editRole}
      catalog={catalog ?? []}
      pending={createRole.isPending || updateRole.isPending}
      success={createRole.isSuccess}
      onCancel={onDone}
      onSubmit={(fields) => {
        if (isEdit && editRole) {
          updateRole.mutate(
            {
              roleId: editRole.id,
              data: {
                name: fields.name,
                description: fields.description,
                permissions: fields.permissions,
              },
            },
            { onSuccess: onDone },
          );
        } else {
          const payload: CreateCustomRoleRequest = {
            name: fields.name,
            description: fields.description,
            scope_kind: fields.scope,
            tenant_id: fields.scope === "shop" ? fields.tenantId || null : null,
            permissions: fields.permissions,
          };
          createRole.mutate(payload);
        }
      }}
    />
  );
}

type FieldValues = {
  name: string;
  description: string;
  scope: "global" | "shop";
  tenantId: string;
  permissions: string[];
};

function RoleFormFields({
  editRole,
  catalog,
  pending,
  success,
  onSubmit,
  onCancel,
}: {
  editRole: CustomRole | null;
  catalog: { key: string; description: string }[];
  pending: boolean;
  success: boolean;
  onSubmit: (v: FieldValues) => void;
  onCancel: () => void;
}) {
  const { t } = useTranslation();
  const isEdit = !!editRole;
  const [name, setName] = useState(editRole?.name ?? "");
  const [description, setDescription] = useState(editRole?.description ?? "");
  const [scope, setScope] = useState<"global" | "shop">(
    editRole?.scope_kind ?? "global",
  );
  const [tenantId, setTenantId] = useState(editRole?.tenant_id ?? "");
  const [selected, setSelected] = useState<Set<string>>(
    new Set(editRole?.permissions ?? []),
  );

  const togglePerm = (key: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const canSubmit =
    name.trim().length > 0 &&
    (isEdit || scope === "global" || tenantId.trim().length > 0) &&
    !pending;

  return (
    <Panel>
      <PanelHeader
        title={isEdit ? t("common.edit") : t("admin.roles.create")}
      />
      <div className="space-y-4 p-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <TermInput
            id="role-name"
            label={t("admin.roles.name")}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <TermInput
            id="role-desc"
            label={t("admin.roles.description")}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
          {!isEdit && (
            <div>
              <div className="mb-2 text-[9px] uppercase tracking-[1.5px] text-t6">
                {t("admin.roles.scope")}
              </div>
              <Segmented<"global" | "shop">
                value={scope}
                options={[
                  { value: "global", label: t("admin.roles.scopeGlobal") },
                  { value: "shop", label: t("admin.roles.scopeShop") },
                ]}
                onChange={setScope}
              />
            </div>
          )}
          {!isEdit && scope === "shop" && (
            <TermInput
              id="role-tenant"
              label={t("admin.roles.tenantId")}
              placeholder={t("admin.roles.tenantIdHint")}
              value={tenantId}
              onChange={(e) => setTenantId(e.target.value)}
            />
          )}
        </div>

        <div>
          <div className="mb-2 text-[9px] uppercase tracking-[1.5px] text-t6">
            {t("admin.roles.permissions")}
          </div>
          <div className="grid max-h-64 gap-1.5 overflow-y-auto sm:grid-cols-2">
            {catalog.map((p) => (
              <label
                key={p.key}
                className="flex cursor-pointer items-start gap-2 border border-line bg-input px-2.5 py-2 text-[11px]"
              >
                <input
                  type="checkbox"
                  checked={selected.has(p.key)}
                  onChange={() => togglePerm(p.key)}
                  className="mt-0.5"
                />
                <span>
                  <span className="text-t2">{p.key}</span>
                  <span className="block text-[10px] text-t6">
                    {p.description}
                  </span>
                </span>
              </label>
            ))}
          </div>
        </div>

        <div className="flex items-center gap-2">
          <TermButton
            type="button"
            variant="primary"
            onClick={() =>
              onSubmit({
                name: name.trim(),
                description: description.trim(),
                scope,
                tenantId: tenantId.trim(),
                permissions: [...selected],
              })
            }
            disabled={!canSubmit}
          >
            {pending ? (
              <Loader2 size={14} className="animate-spin" />
            ) : isEdit ? (
              t("common.save")
            ) : (
              t("admin.roles.create")
            )}
          </TermButton>
          {isEdit && (
            <TermButton type="button" variant="ghost" onClick={onCancel}>
              {t("common.cancel")}
            </TermButton>
          )}
          {!isEdit && success && (
            <span className="text-[10px] uppercase tracking-[1px] text-accent">
              {t("admin.roles.createSuccess")}
            </span>
          )}
        </div>
      </div>
    </Panel>
  );
}
