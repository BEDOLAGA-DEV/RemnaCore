import {
  apiDelete,
  apiGet,
  apiPatch,
  apiPost,
  cn,
  ENDPOINTS,
  LoadingSpinner,
} from "@remnacore/shared";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, Pencil, Plus, Server, Trash2 } from "lucide-react";
import { useState } from "react";

// ─── Types ──────────────────────────────────────────────────────────────────

type HostEntry = {
  panel_id: string;
  host: {
    uuid: string;
    remark: string;
    address: string;
    port: number;
    isDisabled: boolean;
    securityLayer: string;
    sni: string;
    fingerprint: string;
    alpn: string;
    path: string;
    host: string;
  };
};

type PanelEntry = { id: string; slug: string; url: string; status: string };

type InboundEntry = {
  uuid: string;
  tag: string;
  type: string;
  profileUuid?: string;
};

type FormMode = "closed" | "create" | "edit";

// ─── Constants ──────────────────────────────────────────────────────────────

const QUERY_KEY = ["remnawave", "hosts"] as const;
const REMARK_MAX = 40;
const SECURITY_OPTIONS = ["DEFAULT", "TLS", "NONE"] as const;
const ALPN_OPTIONS = [
  "",
  "h2",
  "h3",
  "http/1.1",
  "h2,http/1.1",
  "h3,h2,http/1.1",
  "h3,h2",
] as const;
const FP_OPTIONS = [
  "",
  "chrome",
  "firefox",
  "safari",
  "ios",
  "android",
  "edge",
  "random",
  "randomized",
] as const;

// ─── Component ──────────────────────────────────────────────────────────────

export default function RemnawaveHosts() {
  const queryClient = useQueryClient();

  const {
    data: hosts,
    isLoading,
    isError,
  } = useQuery({
    queryKey: QUERY_KEY,
    queryFn: () => apiGet<HostEntry[]>(ENDPOINTS.remnawave.hosts),
  });

  const { data: panelsRaw } = useQuery({
    queryKey: ["remnawave", "panels"],
    queryFn: () => apiGet<PanelEntry[]>(ENDPOINTS.remnawave.panels),
  });
  const panels = Array.isArray(panelsRaw) ? panelsRaw : [];

  const [formMode, setFormMode] = useState<FormMode>("closed");
  const [editingItem, setEditingItem] = useState<HostEntry | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<HostEntry | null>(null);

  // Form fields
  const [panelID, setPanelID] = useState("");
  const [inboundUUID, setInboundUUID] = useState("");
  const [remark, setRemark] = useState("");
  const [address, setAddress] = useState("");
  const [port, setPort] = useState(443);
  const [sni, setSni] = useState("");
  const [hostField, setHostField] = useState("");
  const [path, setPath] = useState("");
  const [securityLayer, setSecurityLayer] = useState("DEFAULT");
  const [alpn, setAlpn] = useState("");
  const [fingerprint, setFingerprint] = useState("");
  const [isDisabled, setIsDisabled] = useState(false);

  // Fetch inbounds for selected panel
  const { data: inboundsRaw } = useQuery({
    queryKey: ["remnawave", "inbounds", panelID],
    queryFn: () =>
      apiGet<{ panel_id: string; inbounds: InboundEntry[] }>(
        ENDPOINTS.remnawave.configProfileInbounds(panelID),
      ),
    enabled: !!panelID && formMode === "create",
  });
  const inbounds: InboundEntry[] = Array.isArray(inboundsRaw)
    ? inboundsRaw
    : ((inboundsRaw as any)?.inbounds ?? []);

  // ─── Mutations ──────────────────────────────────────────────────────────

  const createMutation = useMutation({
    mutationFn: () => {
      const selectedInbound = inbounds.find((ib) => ib.uuid === inboundUUID);
      return apiPost(`/api/remnawave/hosts/${panelID}`, {
        inbound: {
          configProfileInboundUuid: inboundUUID,
          configProfileUuid: selectedInbound?.profileUuid ?? "",
        },
        remark,
        address,
        port,
        sni,
        host: hostField,
        path,
        securityLayer,
        alpn: alpn || null,
        fingerprint: fingerprint || null,
        isDisabled,
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
      closeForm();
    },
  });

  const updateMutation = useMutation({
    mutationFn: (uuid: string) =>
      apiPatch(`/api/remnawave/hosts/${panelID}`, {
        uuid,
        remark,
        address,
        port,
        sni,
        host: hostField,
        path,
        securityLayer,
        alpn: alpn || null,
        fingerprint: fingerprint || null,
        isDisabled,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
      closeForm();
    },
  });

  const deleteMutation = useMutation({
    mutationFn: ({ pid, uuid }: { pid: string; uuid: string }) =>
      apiDelete(`/api/remnawave/hosts/${pid}/${uuid}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
      setDeleteTarget(null);
    },
  });

  // ─── Handlers ───────────────────────────────────────────────────────────

  function closeForm() {
    setFormMode("closed");
    setEditingItem(null);
    setPanelID("");
    setInboundUUID("");
    setRemark("");
    setAddress("");
    setPort(443);
    setSni("");
    setHostField("");
    setPath("");
    setSecurityLayer("DEFAULT");
    setAlpn("");
    setFingerprint("");
    setIsDisabled(false);
  }

  function openCreate() {
    closeForm();
    setFormMode("create");
  }

  function openEdit(entry: HostEntry) {
    setFormMode("edit");
    setEditingItem(entry);
    setPanelID(entry.panel_id);
    setRemark(entry.host.remark);
    setAddress(entry.host.address);
    setPort(entry.host.port);
    setSni(entry.host.sni);
    setHostField(entry.host.host);
    setPath(entry.host.path);
    setSecurityLayer(entry.host.securityLayer || "DEFAULT");
    setAlpn(entry.host.alpn || "");
    setFingerprint(entry.host.fingerprint || "");
    setIsDisabled(entry.host.isDisabled);
  }

  function handleSubmit() {
    if (formMode === "create") createMutation.mutate();
    else if (editingItem) updateMutation.mutate(editingItem.host.uuid);
  }

  const isSaving = createMutation.isPending || updateMutation.isPending;

  if (isLoading) return <LoadingSpinner />;
  if (isError) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 py-16">
        <AlertTriangle size={32} className="text-destructive" />
        <p className="text-[13px] text-destructive">Failed to load hosts</p>
      </div>
    );
  }

  const list: HostEntry[] = Array.isArray(hosts) ? hosts : [];
  const ic =
    "w-full rounded-lg border border-border bg-background px-3 py-2 text-[13px] text-foreground placeholder:text-muted-foreground/50 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary disabled:opacity-50";

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Server size={18} className="text-muted-foreground" />
          <div>
            <h1 className="text-[15px] font-semibold text-foreground">Hosts</h1>
            <p className="mt-0.5 font-mono text-[11px] text-muted-foreground">
              {list.length} hosts
            </p>
          </div>
        </div>
        <button
          type="button"
          onClick={openCreate}
          className="flex items-center gap-1.5 rounded-lg bg-primary px-3 py-1.5 text-[13px] font-medium text-background hover:bg-primary/90"
        >
          <Plus size={14} /> Create Host
        </button>
      </div>

      {/* Form */}
      {formMode !== "closed" && (
        <div className="rounded-xl border border-border bg-card p-6">
          <h2 className="mb-4 text-[14px] font-semibold text-foreground">
            {formMode === "create" ? "Create Host" : "Edit Host"}
          </h2>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {/* Panel */}
            <div>
              <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                Panel
              </label>
              <select
                value={panelID}
                onChange={(e) => {
                  setPanelID(e.target.value);
                  setInboundUUID("");
                }}
                disabled={formMode === "edit"}
                className={ic}
              >
                <option value="">Select panel...</option>
                {panels.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.slug || p.id} — {p.url}
                  </option>
                ))}
              </select>
            </div>

            {/* Inbound (create only) */}
            {formMode === "create" && (
              <div>
                <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                  Inbound
                </label>
                <select
                  value={inboundUUID}
                  onChange={(e) => setInboundUUID(e.target.value)}
                  className={ic}
                >
                  <option value="">
                    {panelID
                      ? inbounds.length
                        ? "Select inbound..."
                        : "Loading..."
                      : "Select panel first"}
                  </option>
                  {inbounds.map((ib) => (
                    <option key={ib.uuid} value={ib.uuid}>
                      {ib.tag} ({ib.type})
                    </option>
                  ))}
                </select>
              </div>
            )}

            {/* Remark */}
            <div>
              <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                Remark
              </label>
              <input
                type="text"
                value={remark}
                onChange={(e) => setRemark(e.target.value.slice(0, REMARK_MAX))}
                maxLength={REMARK_MAX}
                placeholder="Host name"
                className={ic}
              />
            </div>

            {/* Address */}
            <div>
              <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                Address
              </label>
              <input
                type="text"
                value={address}
                onChange={(e) => setAddress(e.target.value)}
                placeholder="example.com"
                className={ic}
              />
            </div>

            {/* Port */}
            <div>
              <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                Port
              </label>
              <input
                type="number"
                value={port}
                onChange={(e) => setPort(Number(e.target.value))}
                className={ic}
              />
            </div>

            {/* SNI */}
            <div>
              <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                SNI
              </label>
              <input
                type="text"
                value={sni}
                onChange={(e) => setSni(e.target.value)}
                placeholder="sni.example.com"
                className={ic}
              />
            </div>

            {/* Host */}
            <div>
              <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                Host
              </label>
              <input
                type="text"
                value={hostField}
                onChange={(e) => setHostField(e.target.value)}
                className={ic}
              />
            </div>

            {/* Path */}
            <div>
              <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                Path
              </label>
              <input
                type="text"
                value={path}
                onChange={(e) => setPath(e.target.value)}
                placeholder="/"
                className={ic}
              />
            </div>

            {/* Security */}
            <div>
              <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                Security Layer
              </label>
              <select
                value={securityLayer}
                onChange={(e) => setSecurityLayer(e.target.value)}
                className={ic}
              >
                {SECURITY_OPTIONS.map((o) => (
                  <option key={o} value={o}>
                    {o}
                  </option>
                ))}
              </select>
            </div>

            {/* ALPN */}
            <div>
              <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                ALPN
              </label>
              <select
                value={alpn}
                onChange={(e) => setAlpn(e.target.value)}
                className={ic}
              >
                {ALPN_OPTIONS.map((o) => (
                  <option key={o} value={o}>
                    {o || "— none —"}
                  </option>
                ))}
              </select>
            </div>

            {/* Fingerprint */}
            <div>
              <label className="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                Fingerprint
              </label>
              <select
                value={fingerprint}
                onChange={(e) => setFingerprint(e.target.value)}
                className={ic}
              >
                {FP_OPTIONS.map((o) => (
                  <option key={o} value={o}>
                    {o || "— none —"}
                  </option>
                ))}
              </select>
            </div>

            {/* Disabled */}
            <div className="flex items-center gap-3 pt-5">
              <button
                type="button"
                role="switch"
                aria-checked={isDisabled}
                onClick={() => setIsDisabled(!isDisabled)}
                className={cn(
                  "relative h-5 w-9 rounded-full transition-colors",
                  isDisabled ? "bg-destructive" : "bg-muted",
                )}
              >
                <span
                  className={cn(
                    "absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-background transition-transform",
                    isDisabled && "translate-x-4",
                  )}
                />
              </button>
              <span className="text-[13px] text-foreground">Disabled</span>
            </div>
          </div>

          <div className="flex gap-3 pt-4">
            <button
              type="button"
              onClick={handleSubmit}
              disabled={
                isSaving ||
                !panelID ||
                !remark ||
                !address ||
                (formMode === "create" && !inboundUUID)
              }
              className="rounded-lg bg-primary px-4 py-2 text-[13px] font-medium text-background hover:bg-primary/90 disabled:opacity-50"
            >
              {isSaving ? "Saving..." : "Save"}
            </button>
            <button
              type="button"
              onClick={closeForm}
              className="rounded-lg border border-border px-4 py-2 text-[13px] text-muted-foreground hover:bg-secondary"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Delete */}
      {deleteTarget && (
        <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-4">
          <p className="text-[13px]">
            Delete host <strong>{deleteTarget.host.remark}</strong>?
          </p>
          <div className="mt-3 flex gap-2">
            <button
              type="button"
              onClick={() =>
                deleteMutation.mutate({
                  pid: deleteTarget.panel_id,
                  uuid: deleteTarget.host.uuid,
                })
              }
              disabled={deleteMutation.isPending}
              className="rounded-lg bg-destructive px-3 py-1.5 text-[12px] font-medium text-background"
            >
              {deleteMutation.isPending ? "Deleting..." : "Delete"}
            </button>
            <button
              type="button"
              onClick={() => setDeleteTarget(null)}
              className="rounded-lg border border-border px-3 py-1.5 text-[12px] text-muted-foreground"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Table */}
      {list.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
          <Server size={32} className="mb-3 text-muted-foreground/50" />
          <p className="text-[13px] text-muted-foreground">No hosts found</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-border bg-card">
          <table className="w-full">
            <thead>
              <tr className="border-b border-border">
                <th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                  Remark
                </th>
                <th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                  Address
                </th>
                <th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                  Security
                </th>
                <th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                  SNI
                </th>
                <th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                  Status
                </th>
                <th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                  Panel
                </th>
                <th className="px-4 py-3 text-right text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody>
              {list.map((entry) => {
                const h = entry.host;
                return (
                  <tr
                    key={`${entry.panel_id}-${h.uuid}`}
                    className="border-b border-border/50 last:border-0 hover:bg-secondary/30"
                  >
                    <td className="px-4 py-3 text-[13px] font-semibold text-foreground">
                      {h.remark || "—"}
                    </td>
                    <td className="px-4 py-3 font-mono text-[12px]">
                      {h.address}:{h.port}
                    </td>
                    <td className="px-4 py-3 text-[12px]">
                      {h.securityLayer || "—"}
                    </td>
                    <td className="px-4 py-3 font-mono text-[11px] text-muted-foreground">
                      {h.sni || "—"}
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={cn(
                          "inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium",
                          h.isDisabled
                            ? "bg-red-500/10 text-red-500"
                            : "bg-emerald-500/10 text-emerald-500",
                        )}
                      >
                        {h.isDisabled ? "disabled" : "active"}
                      </span>
                    </td>
                    <td className="px-4 py-3 font-mono text-[11px] text-muted-foreground">
                      {entry.panel_id}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-1">
                        <button
                          type="button"
                          onClick={() => openEdit(entry)}
                          className="rounded-lg p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground"
                        >
                          <Pencil size={14} />
                        </button>
                        <button
                          type="button"
                          onClick={() => setDeleteTarget(entry)}
                          className="rounded-lg p-1.5 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                        >
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
