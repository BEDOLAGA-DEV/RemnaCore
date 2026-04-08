import {
	type CircuitBreakerConfig,
	LoadingSpinner,
	type SettingsUpdate,
	type SystemSettings,
	cn,
	useAdminSettings,
	useUpdateSettings,
} from "@remnacore/shared";
import {
	Activity,
	Check,
	ChevronDown,
	CircleDot,
	Database,
	Flag,
	Gauge,
	Globe2,
	Inbox,
	Key,
	Loader2,
	type LucideIcon,
	MessageSquare,
	Navigation,
	Pencil,
	Puzzle,
	Server,
	Shield,
	Signal,
	X,
} from "lucide-react";
import { useCallback, useState } from "react";
import { useTranslation } from "react-i18next";

// ─── Editable Section Keys ──────────────────────────────────────────────────

type EditableSectionKey =
	| "rate_limit"
	| "feature_flags"
	| "smart_router"
	| "speed_test"
	| "plugins"
	| "outbox"
	| "cors"
	| "circuit_breaker";

// ─── Common Styles ──────────────────────────────────────────────────────────

const INPUT_CLASS =
	"w-20 rounded-lg border border-border bg-background px-2 py-1 font-mono text-[13px] text-foreground text-right focus:outline-none focus:ring-1 focus:ring-primary/50";
const FLOAT_INPUT_CLASS =
	"w-24 rounded-lg border border-border bg-background px-2 py-1 font-mono text-[13px] text-foreground text-right focus:outline-none focus:ring-1 focus:ring-primary/50";
const TEXTAREA_CLASS =
	"w-full rounded-lg border border-border bg-background px-3 py-2 font-mono text-[13px] text-foreground leading-relaxed focus:outline-none focus:ring-1 focus:ring-primary/50 resize-y min-h-[80px]";

// ─── Setting Row Components ──────────────────────────────────────────────────

type SettingRowProps = {
	label: string;
	value: string | number;
	masked?: boolean;
};

function SettingRow({ label, value, masked = false }: SettingRowProps) {
	const displayValue = String(value);

	return (
		<div className="flex items-start justify-between gap-4 py-2">
			<span className="shrink-0 text-[13px] text-foreground">{label}</span>
			<span
				className={cn(
					"text-right font-mono text-[13px]",
					masked ? "italic text-muted-foreground/50" : "text-muted-foreground",
				)}
			>
				{displayValue}
			</span>
		</div>
	);
}

type SettingBoolProps = {
	label: string;
	value: boolean;
};

function SettingBool({ label, value }: SettingBoolProps) {
	return (
		<div className="flex items-center justify-between gap-4 py-2">
			<span className="shrink-0 text-[13px] text-foreground">{label}</span>
			<span
				className={cn(
					"rounded-full px-2.5 py-0.5 font-mono text-[11px] font-medium",
					value
						? "bg-emerald-500/10 text-emerald-500"
						: "bg-red-500/10 text-red-500",
				)}
			>
				{value ? "enabled" : "disabled"}
			</span>
		</div>
	);
}

type SettingArrayProps = {
	label: string;
	values: string[];
};

function SettingArray({ label, values }: SettingArrayProps) {
	return (
		<div className="flex items-start justify-between gap-4 py-2">
			<span className="shrink-0 text-[13px] text-foreground">{label}</span>
			<div className="flex flex-col items-end gap-0.5">
				{values.map((v) => (
					<span key={v} className="font-mono text-[13px] text-muted-foreground">
						{v}
					</span>
				))}
			</div>
		</div>
	);
}

// ─── Editable Input Row ─────────────────────────────────────────────────────

type EditableRowProps = {
	label: string;
	value: number;
	onChange: (v: number) => void;
	step?: string;
	inputClass?: string;
};

function EditableRow({
	label,
	value,
	onChange,
	step,
	inputClass = INPUT_CLASS,
}: EditableRowProps) {
	return (
		<div className="flex items-center justify-between gap-4 py-2">
			<span className="shrink-0 text-[13px] text-foreground">{label}</span>
			<input
				type="number"
				value={value}
				step={step}
				onChange={(e) => onChange(Number(e.target.value))}
				className={inputClass}
			/>
		</div>
	);
}

// ─── Toggle Switch ──────────────────────────────────────────────────────────

type ToggleSwitchProps = {
	label: string;
	checked: boolean;
	onChange: (v: boolean) => void;
	disabled?: boolean;
};

function ToggleSwitch({
	label,
	checked,
	onChange,
	disabled = false,
}: ToggleSwitchProps) {
	return (
		<div className="flex items-center justify-between gap-4 py-2">
			<span className="shrink-0 text-[13px] text-foreground">{label}</span>
			<button
				type="button"
				role="switch"
				aria-checked={checked}
				disabled={disabled}
				onClick={() => onChange(!checked)}
				className={cn(
					"relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full transition-colors duration-200 focus:outline-none focus:ring-1 focus:ring-primary/50",
					checked ? "bg-emerald-500" : "bg-muted-foreground/30",
					disabled && "cursor-not-allowed opacity-50",
				)}
			>
				<span
					className={cn(
						"pointer-events-none inline-block h-3.5 w-3.5 rounded-full bg-white shadow-sm transition-transform duration-200",
						checked ? "translate-x-[18px]" : "translate-x-[3px]",
					)}
				/>
			</button>
		</div>
	);
}

// ─── Section Header Buttons ─────────────────────────────────────────────────

type SectionActionsProps = {
	editing: boolean;
	isPending: boolean;
	showSuccess: boolean;
	onEdit: () => void;
	onSave: () => void;
	onCancel: () => void;
};

function SectionActions({
	editing,
	isPending,
	showSuccess,
	onEdit,
	onSave,
	onCancel,
}: SectionActionsProps) {
	if (showSuccess) {
		return (
			<span className="flex items-center gap-1 text-[11px] font-mono text-emerald-500">
				<Check size={12} />
				saved
			</span>
		);
	}

	if (editing) {
		return (
			<div className="flex items-center gap-1.5">
				<button
					type="button"
					onClick={onCancel}
					disabled={isPending}
					className="flex items-center gap-1 rounded-lg border border-border px-2.5 py-1 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground disabled:opacity-50"
				>
					<X size={11} />
					Cancel
				</button>
				<button
					type="button"
					onClick={onSave}
					disabled={isPending}
					className="flex items-center gap-1 rounded-lg bg-primary px-3 py-1 text-[11px] font-medium text-background transition-colors hover:bg-primary/90 disabled:opacity-50"
				>
					{isPending ? <Loader2 size={11} className="animate-spin" /> : null}
					Save
				</button>
			</div>
		);
	}

	return (
		<button
			type="button"
			onClick={(e) => {
				e.stopPropagation();
				onEdit();
			}}
			className="flex items-center gap-1 text-[11px] font-mono text-primary transition-colors hover:text-primary/80"
		>
			<Pencil size={10} />
			Edit
		</button>
	);
}

// ─── Collapsible Section ─────────────────────────────────────────────────────

type SettingsSectionProps = {
	title: string;
	icon: LucideIcon;
	children: React.ReactNode;
	defaultOpen?: boolean;
	actions?: React.ReactNode;
};

function SettingsSection({
	title,
	icon: Icon,
	children,
	defaultOpen = true,
	actions,
}: SettingsSectionProps) {
	const [open, setOpen] = useState(defaultOpen);

	return (
		<div className="rounded-xl border border-border bg-card">
			<div className="flex w-full items-center gap-3 px-5 py-4">
				<button
					type="button"
					onClick={() => setOpen((prev) => !prev)}
					className="flex flex-1 items-center gap-3 text-left transition-colors hover:opacity-80"
				>
					<Icon size={16} className="shrink-0 text-primary" />
					<span className="flex-1 text-[13px] font-semibold text-foreground">
						{title}
					</span>
					<ChevronDown
						size={14}
						className={cn(
							"shrink-0 text-muted-foreground transition-transform duration-200",
							open && "rotate-180",
						)}
					/>
				</button>
				{actions ? <div className="shrink-0 pl-2">{actions}</div> : null}
			</div>
			{open ? (
				<div className="border-t border-border px-5 pb-4 pt-2">
					<div className="divide-y divide-border/50">{children}</div>
				</div>
			) : null}
		</div>
	);
}

// ─── Circuit Breaker Sub-Section ─────────────────────────────────────────────

type CircuitBreakerBlockProps = {
	name: string;
	config: CircuitBreakerConfig;
};

function CircuitBreakerBlock({ name, config }: CircuitBreakerBlockProps) {
	return (
		<div className="py-2">
			<p className="mb-1 text-[12px] font-medium uppercase tracking-wider text-muted-foreground">
				{name}
			</p>
			<div className="grid grid-cols-2 gap-x-6 gap-y-0.5 pl-3">
				<SettingRow label="Max Failures" value={config.max_failures} />
				<SettingRow label="Timeout" value={config.timeout} />
				<SettingRow label="Max Requests" value={config.max_requests} />
				<SettingRow label="Interval" value={config.interval} />
			</div>
		</div>
	);
}

type CircuitBreakerEditBlockProps = {
	name: string;
	config: CircuitBreakerConfig;
	onChange: (config: CircuitBreakerConfig) => void;
};

function CircuitBreakerEditBlock({
	name,
	config,
	onChange,
}: CircuitBreakerEditBlockProps) {
	const update = (
		field: keyof CircuitBreakerConfig,
		value: string | number,
	) => {
		onChange({ ...config, [field]: value });
	};

	return (
		<div className="py-2">
			<p className="mb-1 text-[12px] font-medium uppercase tracking-wider text-muted-foreground">
				{name}
			</p>
			<div className="grid grid-cols-2 gap-x-6 gap-y-0.5 pl-3">
				<EditableRow
					label="Max Failures"
					value={config.max_failures}
					onChange={(v) => update("max_failures", v)}
				/>
				<div className="flex items-center justify-between gap-4 py-2">
					<span className="shrink-0 text-[13px] text-foreground">Timeout</span>
					<input
						type="text"
						value={config.timeout}
						onChange={(e) => update("timeout", e.target.value)}
						className={cn(INPUT_CLASS, "w-24 text-left")}
					/>
				</div>
				<EditableRow
					label="Max Requests"
					value={config.max_requests}
					onChange={(v) => update("max_requests", v)}
				/>
				<div className="flex items-center justify-between gap-4 py-2">
					<span className="shrink-0 text-[13px] text-foreground">Interval</span>
					<input
						type="text"
						value={config.interval}
						onChange={(e) => update("interval", e.target.value)}
						className={cn(INPUT_CLASS, "w-24 text-left")}
					/>
				</div>
			</div>
		</div>
	);
}

// ─── Settings Content ────────────────────────────────────────────────────────

type SettingsContentProps = {
	settings: SystemSettings;
};

function SettingsContent({ settings }: SettingsContentProps) {
	const { t } = useTranslation();
	const updateMutation = useUpdateSettings();

	const [editingSection, setEditingSection] =
		useState<EditableSectionKey | null>(null);
	const [successSection, setSuccessSection] =
		useState<EditableSectionKey | null>(null);
	const [mutationError, setMutationError] = useState<string | null>(null);

	// Per-section form state — only populated when editing
	const [rateLimitForm, setRateLimitForm] = useState({
		...settings.rate_limit,
	});
	const [smartRouterForm, setSmartRouterForm] = useState({
		...settings.smart_router,
	});
	const [speedTestForm, setSpeedTestForm] = useState({
		...settings.speed_test,
	});
	const [pluginsForm, setPluginsForm] = useState({
		max_plugins: settings.plugins.max_plugins,
		enable_hot_reload: settings.plugins.enable_hot_reload,
	});
	const [outboxForm, setOutboxForm] = useState({ ...settings.outbox });
	const [corsForm, setCorsForm] = useState(
		settings.cors.allowed_origins.join("\n"),
	);
	const [cbForm, setCbForm] = useState({ ...settings.circuit_breaker });

	const startEditing = useCallback(
		(section: EditableSectionKey) => {
			// Reset form state from current settings when entering edit mode
			setMutationError(null);
			switch (section) {
				case "rate_limit":
					setRateLimitForm({ ...settings.rate_limit });
					break;
				case "smart_router":
					setSmartRouterForm({ ...settings.smart_router });
					break;
				case "speed_test":
					setSpeedTestForm({ ...settings.speed_test });
					break;
				case "plugins":
					setPluginsForm({
						max_plugins: settings.plugins.max_plugins,
						enable_hot_reload: settings.plugins.enable_hot_reload,
					});
					break;
				case "outbox":
					setOutboxForm({ ...settings.outbox });
					break;
				case "cors":
					setCorsForm(settings.cors.allowed_origins.join("\n"));
					break;
				case "circuit_breaker":
					setCbForm({ ...settings.circuit_breaker });
					break;
			}
			setEditingSection(section);
		},
		[settings],
	);

	const cancelEditing = useCallback(() => {
		setEditingSection(null);
		setMutationError(null);
	}, []);

	const SUCCESS_FLASH_MS = 1500;

	const saveSection = useCallback(
		(section: EditableSectionKey) => {
			let payload: SettingsUpdate;

			switch (section) {
				case "rate_limit":
					payload = { rate_limit: rateLimitForm };
					break;
				case "smart_router":
					payload = { smart_router: smartRouterForm };
					break;
				case "speed_test":
					payload = { speed_test: speedTestForm };
					break;
				case "plugins":
					payload = { plugins: pluginsForm };
					break;
				case "outbox":
					payload = { outbox: outboxForm };
					break;
				case "cors": {
					const origins = corsForm
						.split("\n")
						.map((s) => s.trim())
						.filter((s) => s.length > 0);
					payload = { cors: { allowed_origins: origins } };
					break;
				}
				case "circuit_breaker":
					payload = { circuit_breaker: cbForm };
					break;
				default:
					return;
			}

			setMutationError(null);
			updateMutation.mutate(payload, {
				onSuccess: () => {
					setEditingSection(null);
					setSuccessSection(section);
					setTimeout(() => setSuccessSection(null), SUCCESS_FLASH_MS);
				},
				onError: (err: unknown) => {
					setMutationError(
						err instanceof Error ? err.message : t("common.unknownError"),
					);
				},
			});
		},
		[
			rateLimitForm,
			smartRouterForm,
			speedTestForm,
			pluginsForm,
			outboxForm,
			corsForm,
			cbForm,
			updateMutation,
			t,
		],
	);

	// Feature flags use instant-save toggles (no edit/save flow)
	const toggleFeatureFlag = useCallback(
		(flag: keyof SystemSettings["feature_flags"], currentValue: boolean) => {
			updateMutation.mutate(
				{ feature_flags: { [flag]: !currentValue } },
				{
					onSuccess: () => {
						setSuccessSection("feature_flags");
						setTimeout(() => setSuccessSection(null), SUCCESS_FLASH_MS);
					},
				},
			);
		},
		[updateMutation],
	);

	const makeActions = (section: EditableSectionKey) => (
		<SectionActions
			editing={editingSection === section}
			isPending={updateMutation.isPending && editingSection === section}
			showSuccess={successSection === section}
			onEdit={() => startEditing(section)}
			onSave={() => saveSection(section)}
			onCancel={cancelEditing}
		/>
	);

	return (
		<div className="space-y-3">
			{/* Application (read-only) */}
			<SettingsSection title="Application" icon={Server}>
				<SettingRow label="Port" value={settings.app.port} />
				<SettingRow label="Version" value={settings.app.version} />
				<SettingRow label="Log Level" value={settings.app.log_level} />
				<SettingRow label="Log Format" value={settings.app.log_format} />
			</SettingsSection>

			{/* Database (read-only) */}
			<SettingsSection title="Database" icon={Database}>
				<SettingRow label="URL" value={settings.database.url} masked />
				<SettingRow
					label="Max Connections"
					value={settings.database.max_conns}
				/>
				<SettingRow
					label="Min Connections"
					value={settings.database.min_conns}
				/>
				<SettingRow
					label="Max Connection Lifetime"
					value={settings.database.max_conn_lifetime}
				/>
				<SettingRow
					label="Max Connection Idle Time"
					value={settings.database.max_conn_idle_time}
				/>
				<SettingRow
					label="Health Check Period"
					value={settings.database.health_check_period}
				/>
			</SettingsSection>

			{/* Cache (read-only) */}
			<SettingsSection title="Cache (Valkey)" icon={Database}>
				<SettingRow label="URL" value={settings.cache.url} masked />
			</SettingsSection>

			{/* Message Queue (read-only) */}
			<SettingsSection title="Message Queue (NATS)" icon={Inbox}>
				<SettingRow label="URL" value={settings.message_queue.url} masked />
			</SettingsSection>

			{/* JWT (read-only) */}
			<SettingsSection title="JWT Authentication" icon={Key}>
				<SettingRow
					label="Private Key Path"
					value={settings.jwt.private_key_path}
				/>
				<SettingRow
					label="Public Key Path"
					value={settings.jwt.public_key_path}
				/>
				<SettingRow
					label="Access Token TTL"
					value={settings.jwt.access_token_ttl}
				/>
				<SettingRow
					label="Refresh Token TTL"
					value={settings.jwt.refresh_token_ttl}
				/>
			</SettingsSection>

			{/* Telegram (read-only) */}
			<SettingsSection title="Telegram" icon={MessageSquare}>
				<SettingRow
					label="Bot Token"
					value={settings.telegram.bot_token}
					masked
				/>
				<SettingRow label="Webhook URL" value={settings.telegram.webhook_url} />
				<SettingRow label="Cabinet URL" value={settings.telegram.cabinet_url} />
			</SettingsSection>

			{/* Plugins (editable) */}
			<SettingsSection
				title="Plugins"
				icon={Puzzle}
				actions={makeActions("plugins")}
			>
				{editingSection === "plugins" ? (
					<>
						<SettingRow
							label="Directory"
							value={settings.plugins.plugins_dir}
						/>
						<EditableRow
							label="Max Plugins"
							value={pluginsForm.max_plugins}
							onChange={(v) =>
								setPluginsForm((prev) => ({ ...prev, max_plugins: v }))
							}
						/>
						<ToggleSwitch
							label="Hot Reload"
							checked={pluginsForm.enable_hot_reload}
							onChange={(v) =>
								setPluginsForm((prev) => ({ ...prev, enable_hot_reload: v }))
							}
						/>
						{mutationError ? (
							<p className="py-1 text-[12px] text-destructive">
								{mutationError}
							</p>
						) : null}
					</>
				) : (
					<>
						<SettingRow
							label="Directory"
							value={settings.plugins.plugins_dir}
						/>
						<SettingRow
							label="Max Plugins"
							value={settings.plugins.max_plugins}
						/>
						<SettingBool
							label="Hot Reload"
							value={settings.plugins.enable_hot_reload}
						/>
					</>
				)}
			</SettingsSection>

			{/* Infrastructure (read-only) */}
			<SettingsSection title="Infrastructure" icon={Activity}>
				<SettingRow
					label="Health Check Interval"
					value={settings.infrastructure.health_check_interval}
				/>
				<SettingRow
					label="Max Concurrent Checks"
					value={settings.infrastructure.max_concurrent_checks}
				/>
				<SettingRow
					label="Speed Test Port"
					value={settings.infrastructure.speed_test_port}
				/>
				<SettingRow
					label="Subscription Proxy Port"
					value={settings.infrastructure.subscription_proxy_port}
				/>
			</SettingsSection>

			{/* Speed Test (editable) */}
			<SettingsSection
				title="Speed Test"
				icon={Gauge}
				actions={makeActions("speed_test")}
			>
				{editingSection === "speed_test" ? (
					<>
						<EditableRow
							label="Max Concurrent"
							value={speedTestForm.max_concurrent}
							onChange={(v) =>
								setSpeedTestForm((prev) => ({ ...prev, max_concurrent: v }))
							}
						/>
						<EditableRow
							label="Per-IP Rate Limit"
							value={speedTestForm.per_ip_rate_limit}
							onChange={(v) =>
								setSpeedTestForm((prev) => ({ ...prev, per_ip_rate_limit: v }))
							}
						/>
						<EditableRow
							label="Max Upload Bytes"
							value={speedTestForm.max_upload_bytes}
							onChange={(v) =>
								setSpeedTestForm((prev) => ({ ...prev, max_upload_bytes: v }))
							}
						/>
						{mutationError ? (
							<p className="py-1 text-[12px] text-destructive">
								{mutationError}
							</p>
						) : null}
					</>
				) : (
					<>
						<SettingRow
							label="Max Concurrent"
							value={settings.speed_test.max_concurrent}
						/>
						<SettingRow
							label="Per-IP Rate Limit"
							value={settings.speed_test.per_ip_rate_limit}
						/>
						<SettingRow
							label="Max Upload Bytes"
							value={settings.speed_test.max_upload_bytes.toLocaleString()}
						/>
					</>
				)}
			</SettingsSection>

			{/* Rate Limiting (editable) */}
			<SettingsSection
				title="Rate Limiting"
				icon={Shield}
				actions={makeActions("rate_limit")}
			>
				{editingSection === "rate_limit" ? (
					<>
						<EditableRow
							label="Checkout Max / Hour"
							value={rateLimitForm.checkout_max_per_hour}
							onChange={(v) =>
								setRateLimitForm((prev) => ({
									...prev,
									checkout_max_per_hour: v,
								}))
							}
						/>
						<EditableRow
							label="Subscription Max / Day"
							value={rateLimitForm.subscription_max_per_day}
							onChange={(v) =>
								setRateLimitForm((prev) => ({
									...prev,
									subscription_max_per_day: v,
								}))
							}
						/>
						<EditableRow
							label="Login Max / Window"
							value={rateLimitForm.login_max_per_window}
							onChange={(v) =>
								setRateLimitForm((prev) => ({
									...prev,
									login_max_per_window: v,
								}))
							}
						/>
						<EditableRow
							label="Login Window (min)"
							value={rateLimitForm.login_window_minutes}
							onChange={(v) =>
								setRateLimitForm((prev) => ({
									...prev,
									login_window_minutes: v,
								}))
							}
						/>
						<EditableRow
							label="Forgot Password Max / Window"
							value={rateLimitForm.forgot_pwd_max_per_window}
							onChange={(v) =>
								setRateLimitForm((prev) => ({
									...prev,
									forgot_pwd_max_per_window: v,
								}))
							}
						/>
						<EditableRow
							label="Forgot Password Window (min)"
							value={rateLimitForm.forgot_pwd_window_minutes}
							onChange={(v) =>
								setRateLimitForm((prev) => ({
									...prev,
									forgot_pwd_window_minutes: v,
								}))
							}
						/>
						{mutationError ? (
							<p className="py-1 text-[12px] text-destructive">
								{mutationError}
							</p>
						) : null}
					</>
				) : (
					<>
						<SettingRow
							label="Checkout Max / Hour"
							value={settings.rate_limit.checkout_max_per_hour}
						/>
						<SettingRow
							label="Subscription Max / Day"
							value={settings.rate_limit.subscription_max_per_day}
						/>
						<SettingRow
							label="Login Max / Window"
							value={settings.rate_limit.login_max_per_window}
						/>
						<SettingRow
							label="Login Window (min)"
							value={settings.rate_limit.login_window_minutes}
						/>
						<SettingRow
							label="Forgot Password Max / Window"
							value={settings.rate_limit.forgot_pwd_max_per_window}
						/>
						<SettingRow
							label="Forgot Password Window (min)"
							value={settings.rate_limit.forgot_pwd_window_minutes}
						/>
					</>
				)}
			</SettingsSection>

			{/* Outbox (editable) */}
			<SettingsSection
				title="Outbox Relay"
				icon={Inbox}
				actions={makeActions("outbox")}
			>
				{editingSection === "outbox" ? (
					<>
						<EditableRow
							label="Relay Workers"
							value={outboxForm.relay_workers}
							onChange={(v) =>
								setOutboxForm((prev) => ({ ...prev, relay_workers: v }))
							}
						/>
						<EditableRow
							label="Partition Lookahead"
							value={outboxForm.partition_lookahead}
							onChange={(v) =>
								setOutboxForm((prev) => ({ ...prev, partition_lookahead: v }))
							}
						/>
						<EditableRow
							label="Retention Days"
							value={outboxForm.retention_days}
							onChange={(v) =>
								setOutboxForm((prev) => ({ ...prev, retention_days: v }))
							}
						/>
						{mutationError ? (
							<p className="py-1 text-[12px] text-destructive">
								{mutationError}
							</p>
						) : null}
					</>
				) : (
					<>
						<SettingRow
							label="Relay Workers"
							value={settings.outbox.relay_workers}
						/>
						<SettingRow
							label="Partition Lookahead"
							value={settings.outbox.partition_lookahead}
						/>
						<SettingRow
							label="Retention Days"
							value={settings.outbox.retention_days}
						/>
					</>
				)}
			</SettingsSection>

			{/* Smart Router (editable) */}
			<SettingsSection
				title="Smart Router"
				icon={Navigation}
				actions={makeActions("smart_router")}
			>
				{editingSection === "smart_router" ? (
					<>
						<p className="pb-1 pt-2 text-[12px] font-medium uppercase tracking-wider text-muted-foreground">
							Default Weights
						</p>
						<EditableRow
							label="Geo"
							value={smartRouterForm.weight_geo}
							onChange={(v) =>
								setSmartRouterForm((prev) => ({ ...prev, weight_geo: v }))
							}
							step="0.01"
							inputClass={FLOAT_INPUT_CLASS}
						/>
						<EditableRow
							label="Latency"
							value={smartRouterForm.weight_latency}
							onChange={(v) =>
								setSmartRouterForm((prev) => ({ ...prev, weight_latency: v }))
							}
							step="0.01"
							inputClass={FLOAT_INPUT_CLASS}
						/>
						<EditableRow
							label="Load"
							value={smartRouterForm.weight_load}
							onChange={(v) =>
								setSmartRouterForm((prev) => ({ ...prev, weight_load: v }))
							}
							step="0.01"
							inputClass={FLOAT_INPUT_CLASS}
						/>

						<p className="pb-1 pt-3 text-[12px] font-medium uppercase tracking-wider text-muted-foreground">
							Gaming Weights
						</p>
						<EditableRow
							label="Geo"
							value={smartRouterForm.weight_gaming_geo}
							onChange={(v) =>
								setSmartRouterForm((prev) => ({
									...prev,
									weight_gaming_geo: v,
								}))
							}
							step="0.01"
							inputClass={FLOAT_INPUT_CLASS}
						/>
						<EditableRow
							label="Latency"
							value={smartRouterForm.weight_gaming_latency}
							onChange={(v) =>
								setSmartRouterForm((prev) => ({
									...prev,
									weight_gaming_latency: v,
								}))
							}
							step="0.01"
							inputClass={FLOAT_INPUT_CLASS}
						/>
						<EditableRow
							label="Load"
							value={smartRouterForm.weight_gaming_load}
							onChange={(v) =>
								setSmartRouterForm((prev) => ({
									...prev,
									weight_gaming_load: v,
								}))
							}
							step="0.01"
							inputClass={FLOAT_INPUT_CLASS}
						/>

						<p className="pb-1 pt-3 text-[12px] font-medium uppercase tracking-wider text-muted-foreground">
							Streaming Weights
						</p>
						<EditableRow
							label="Geo"
							value={smartRouterForm.weight_streaming_geo}
							onChange={(v) =>
								setSmartRouterForm((prev) => ({
									...prev,
									weight_streaming_geo: v,
								}))
							}
							step="0.01"
							inputClass={FLOAT_INPUT_CLASS}
						/>
						<EditableRow
							label="Latency"
							value={smartRouterForm.weight_streaming_latency}
							onChange={(v) =>
								setSmartRouterForm((prev) => ({
									...prev,
									weight_streaming_latency: v,
								}))
							}
							step="0.01"
							inputClass={FLOAT_INPUT_CLASS}
						/>
						<EditableRow
							label="Load"
							value={smartRouterForm.weight_streaming_load}
							onChange={(v) =>
								setSmartRouterForm((prev) => ({
									...prev,
									weight_streaming_load: v,
								}))
							}
							step="0.01"
							inputClass={FLOAT_INPUT_CLASS}
						/>
						{mutationError ? (
							<p className="py-1 text-[12px] text-destructive">
								{mutationError}
							</p>
						) : null}
					</>
				) : (
					<>
						<p className="pb-1 pt-2 text-[12px] font-medium uppercase tracking-wider text-muted-foreground">
							Default Weights
						</p>
						<SettingRow label="Geo" value={settings.smart_router.weight_geo} />
						<SettingRow
							label="Latency"
							value={settings.smart_router.weight_latency}
						/>
						<SettingRow
							label="Load"
							value={settings.smart_router.weight_load}
						/>
						<p className="pb-1 pt-3 text-[12px] font-medium uppercase tracking-wider text-muted-foreground">
							Gaming Weights
						</p>
						<SettingRow
							label="Geo"
							value={settings.smart_router.weight_gaming_geo}
						/>
						<SettingRow
							label="Latency"
							value={settings.smart_router.weight_gaming_latency}
						/>
						<SettingRow
							label="Load"
							value={settings.smart_router.weight_gaming_load}
						/>
						<p className="pb-1 pt-3 text-[12px] font-medium uppercase tracking-wider text-muted-foreground">
							Streaming Weights
						</p>
						<SettingRow
							label="Geo"
							value={settings.smart_router.weight_streaming_geo}
						/>
						<SettingRow
							label="Latency"
							value={settings.smart_router.weight_streaming_latency}
						/>
						<SettingRow
							label="Load"
							value={settings.smart_router.weight_streaming_load}
						/>
					</>
				)}
			</SettingsSection>

			{/* Feature Flags (editable — instant toggle) */}
			<SettingsSection
				title="Feature Flags"
				icon={Flag}
				actions={
					successSection === "feature_flags" ? (
						<span className="flex items-center gap-1 text-[11px] font-mono text-emerald-500">
							<Check size={12} />
							saved
						</span>
					) : updateMutation.isPending && editingSection === null ? (
						<Loader2 size={12} className="animate-spin text-muted-foreground" />
					) : null
				}
			>
				<ToggleSwitch
					label="Subscription Hooks"
					checked={settings.feature_flags.hooks_subscription_enabled}
					onChange={() =>
						toggleFeatureFlag(
							"hooks_subscription_enabled",
							settings.feature_flags.hooks_subscription_enabled,
						)
					}
					disabled={updateMutation.isPending}
				/>
				<ToggleSwitch
					label="VPN Provider Hooks"
					checked={settings.feature_flags.hooks_vpn_provider_enabled}
					onChange={() =>
						toggleFeatureFlag(
							"hooks_vpn_provider_enabled",
							settings.feature_flags.hooks_vpn_provider_enabled,
						)
					}
					disabled={updateMutation.isPending}
				/>
			</SettingsSection>

			{/* Circuit Breakers (editable) */}
			<SettingsSection
				title="Circuit Breakers"
				icon={CircleDot}
				actions={makeActions("circuit_breaker")}
			>
				{editingSection === "circuit_breaker" ? (
					<>
						<CircuitBreakerEditBlock
							name="Remnawave"
							config={cbForm.remnawave}
							onChange={(c) => setCbForm((prev) => ({ ...prev, remnawave: c }))}
						/>
						<CircuitBreakerEditBlock
							name="Outbox NATS"
							config={cbForm.outbox_nats}
							onChange={(c) =>
								setCbForm((prev) => ({ ...prev, outbox_nats: c }))
							}
						/>
						<CircuitBreakerEditBlock
							name="Valkey"
							config={cbForm.valkey}
							onChange={(c) => setCbForm((prev) => ({ ...prev, valkey: c }))}
						/>
						<CircuitBreakerEditBlock
							name="VPN Provider"
							config={cbForm.vpn_provider}
							onChange={(c) =>
								setCbForm((prev) => ({ ...prev, vpn_provider: c }))
							}
						/>
						{mutationError ? (
							<p className="py-1 text-[12px] text-destructive">
								{mutationError}
							</p>
						) : null}
					</>
				) : (
					<>
						<CircuitBreakerBlock
							name="Remnawave"
							config={settings.circuit_breaker.remnawave}
						/>
						<CircuitBreakerBlock
							name="Outbox NATS"
							config={settings.circuit_breaker.outbox_nats}
						/>
						<CircuitBreakerBlock
							name="Valkey"
							config={settings.circuit_breaker.valkey}
						/>
						<CircuitBreakerBlock
							name="VPN Provider"
							config={settings.circuit_breaker.vpn_provider}
						/>
					</>
				)}
			</SettingsSection>

			{/* CORS (editable) */}
			<SettingsSection title="CORS" icon={Globe2} actions={makeActions("cors")}>
				{editingSection === "cors" ? (
					<>
						<div className="py-2">
							<p className="mb-2 text-[13px] text-foreground">
								Allowed Origins
							</p>
							<textarea
								value={corsForm}
								onChange={(e) => setCorsForm(e.target.value)}
								placeholder="https://example.com"
								className={TEXTAREA_CLASS}
								rows={4}
							/>
							<p className="mt-1 text-[11px] text-muted-foreground">
								One origin per line
							</p>
						</div>
						{mutationError ? (
							<p className="py-1 text-[12px] text-destructive">
								{mutationError}
							</p>
						) : null}
					</>
				) : (
					<SettingArray
						label="Allowed Origins"
						values={settings.cors.allowed_origins}
					/>
				)}
			</SettingsSection>

			{/* Tracing (read-only) */}
			<SettingsSection title="Tracing" icon={Signal}>
				<SettingRow label="Endpoint" value={settings.tracing.endpoint} />
			</SettingsSection>
		</div>
	);
}

// ─── Page ────────────────────────────────────────────────────────────────────

export function SettingsPage() {
	const { t } = useTranslation();
	const { data: settings, isLoading, isError, error } = useAdminSettings();

	if (isLoading) {
		return <LoadingSpinner />;
	}

	if (isError) {
		return (
			<div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
				<p className="text-[13px] text-destructive">
					{t("common.errorLoading")}:{" "}
					{error instanceof Error ? error.message : t("common.unknownError")}
				</p>
			</div>
		);
	}

	if (!settings) {
		return null;
	}

	return (
		<div className="space-y-6">
			{/* Header */}
			<div className="flex items-center justify-between">
				<h1 className="text-[15px] font-semibold text-foreground">
					System Settings
				</h1>
				<div className="flex items-center gap-2">
					<span className="rounded-full bg-primary/10 px-2.5 py-0.5 font-mono text-[11px] font-medium text-primary">
						v{settings.app.version}
					</span>
					<span className="rounded-full bg-secondary px-2.5 py-0.5 font-mono text-[11px] font-medium text-muted-foreground">
						:{settings.app.port}
					</span>
				</div>
			</div>

			<SettingsContent settings={settings} />
		</div>
	);
}
