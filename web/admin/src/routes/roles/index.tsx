import {
  type CreateCustomRoleRequest,
  type CustomRole,
  useCreateCustomRole,
  useCustomRoles,
  useDeleteCustomRole,
  usePermissionCatalog,
} from "@remnacore/shared";
import { Loader2, Trash2 } from "lucide-react";
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

  return (
    <div className="space-y-6">
      <PageHeader
        title={t("admin.roles.title")}
        breadcrumb={t("admin.roles.subtitle")}
      />

      <CreateRoleForm />

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
                onDelete={() => {
                  if (window.confirm(t("admin.roles.deleteConfirm"))) {
                    deleteRole.mutate(role.id);
                  }
                }}
                deleting={deleteRole.isPending}
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
  onDelete,
  deleting,
}: {
  role: CustomRole;
  onDelete: () => void;
  deleting: boolean;
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
          {t("admin.roles.permissionCount", {
            count: role.permissions.length,
          })}
          {role.permissions.length > 0 && ` — ${role.permissions.join(", ")}`}
        </p>
      </div>
      <TermButton
        variant="ghost"
        onClick={onDelete}
        disabled={deleting}
        aria-label={t("common.delete")}
      >
        <Trash2 size={14} />
      </TermButton>
    </li>
  );
}

function CreateRoleForm() {
  const { t } = useTranslation();
  const { data: catalog } = usePermissionCatalog();
  const createRole = useCreateCustomRole();

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [scope, setScope] = useState<"global" | "shop">("global");
  const [tenantId, setTenantId] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const togglePerm = (key: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  const submit = () => {
    const payload: CreateCustomRoleRequest = {
      name: name.trim(),
      description: description.trim(),
      scope_kind: scope,
      tenant_id: scope === "shop" ? tenantId.trim() || null : null,
      permissions: [...selected],
    };
    createRole.mutate(payload, {
      onSuccess: () => {
        setName("");
        setDescription("");
        setTenantId("");
        setSelected(new Set());
      },
    });
  };

  const canSubmit =
    name.trim().length > 0 &&
    (scope === "global" || tenantId.trim().length > 0) &&
    !createRole.isPending;

  return (
    <Panel>
      <PanelHeader title={t("admin.roles.create")} />
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
          {scope === "shop" && (
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
            {(catalog ?? []).map((p) => (
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

        <TermButton
          type="button"
          variant="primary"
          onClick={submit}
          disabled={!canSubmit}
        >
          {createRole.isPending ? (
            <Loader2 size={14} className="animate-spin" />
          ) : (
            t("admin.roles.create")
          )}
        </TermButton>
        {createRole.isSuccess && (
          <p className="text-[10px] uppercase tracking-[1px] text-accent">
            {t("admin.roles.createSuccess")}
          </p>
        )}
      </div>
    </Panel>
  );
}
