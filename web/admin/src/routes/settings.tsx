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
import {
	PageHeader,
	Panel,
	PanelHeader,
	StatusPill,
	TermButton,
	TermInput,
} from "../components/ui/index.js";

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

const NUM_INPUT_CLASS = "w-24 text-right tabular-nums";
const TEXTAREA_CLASS =
	"w-full border bg-input px-3 py-2 text-[13px] tracking-[.5px] text-t1 leading-relaxed outline-none placeholder:text-t7 focus:border-accent/45 resize-y min-h-[88px]";
const TEXTAREA_STYLE = { borderColor: "var(--line-strong)" } as const;

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
			<span className="shrink-0 text-[13px] text-t2">{label}</span>
			<span
				className={cn(
					"text-right text-[13px] tabular-nums",
					masked ? "italic text-t7" : "text-t4",
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
			<span className="shrink-0 text-[13px] text-t2">{label}</span>
			<StatusPill
				label={value ? "ENABLED" : "DISABLED"}
				tone={value ? "ok" : "danger"}
			/>
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
			<span className="shrink-0 text-[13px] text-t2">{label}</span>
			<div className="flex flex-col items-end gap-0.5">
				{values.map((v) => (
					<span key={v} className="text-[13px] text-t4">
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
};

function EditableRow({ label, value, onChange, step }: EditableRowProps) {
	return (
		<div className="flex items-center justify-between gap-4 py-2">
			<span className="shrink-0 text-[13px] text-t2">{label}</span>
			<TermInput
				type="number"
				value={value}
				step={step}
				onChange={(e) => onChange(Number(e.target.value))}
				className={NUM_INPUT_CLASS}
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
			<span className="shrink-0 text-[13px] text-t2">{label}</span>
			<button
				type="button"
				role="switch"
				aria-checked={checked}
				disabled={disabled}
				onClick={() => onChange(!checked)}
				className={cn(
					"relative inline-flex h-[23px] w-[42px] shrink-0 cursor-pointer items-center border transition-colors focus:outline-none",
					checked
						? "border-accent bg-accent/20"
						: "border-line bg-input hover:border-accent/40",
					disabled && "cursor-not-allowed opacity-50",
				)}
			>
				<span
					className={cn(
						"pointer-events-none absolute top-[2px] h-[17px] w-[17px] transition-[left] duration-150",
						checked ? "left-[22px] bg-accent" : "left-[2px] bg-t5",
					)}
				/>
			</button>
		</div>
	);
}

// ─── Section Header Actions ─────────────────────────────────────────────────

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
			<span className="flex items-center gap-1 text-[10px] uppercase tracking-[1px] text-accent">
				<Check size={12} />
				SAVED
			</span>
		);
	}

	if (editing) {
		return (
			<div className="flex items-center gap-1.5">
				<TermButton
					type="button"
					variant="ghost"
					onClick={onCancel}
					disabled={isPending}
					className="px-2.5 py-1"
				>
					<X size={11} />
					CANCEL
				</TermButton>
				<TermButton
					type="button"
					variant="primary"
					onClick={onSave}
					disabled={isPending}
					className="px-3 py-1"
				>
					{isPending ? <Loader2 size={11} className="animate-spin" /> : null}
					SAVE
				</TermButton>
			</div>
		);
	}

	return (
		<TermButton
			type="button"
			variant="ghost"
			onClick={(e) => {
				e.stopPropagation();
				onEdit();
			}}
			className="px-2.5 py-1"
		>
			<Pencil size={10} />
			EDIT
		</TermButton>
	);
}

// ─── Settings Section (Panel) ─────────────────────────────────────────────────

type SettingsSectionProps = {
	title: string;
	icon: LucideIcon;
	children: React.ReactNode;
	actions?: React.ReactNode;
};

function SettingsSection({
	title,
	icon: Icon,
	children,
	actions,
}: SettingsSectionProps) {
	return (
		<Panel>
			<PanelHeader
				title={title}
				right={
					<div className="flex items-center gap-3">
						<Icon size={14} className="shrink-0 text-accent" />
						{actions}
					</div>
				}
			/>
			<div className="px-4 py-3">
				<div className="divide-y divide-[var(--line-soft)]">{children}</div>
			</div>
		</Panel>
	);
}

// ─── Mutation Error ───────────────────────────────────────────────────────────

function MutationError({ message }: { message: string | null }) {
	if (!message) {
		return null;
	}
	return (
		<p className="py-1.5 text-[11px] uppercase tracking-[.5px] text-danger">
			{message}
		</p>
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
			<p className="mb-1 text-[10px] uppercase tracking-[1.5px] text-t6">
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
			<p className="mb-1 text-[10px] uppercase tracking-[1.5px] text-t6">
				{name}
			</p>
			<div className="grid grid-cols-2 gap-x-6 gap-y-0.5 pl-3">
				<EditableRow
					label="Max Failures"
					value={config.max_failures}
					onChange={(v) => update("max_failures", v)}
				/>
				<div className="flex items-center justify-between gap-4 py-2">
					<span className="shrink-0 text-[13px] text-t2">Timeout</span>
					<TermInput
						type="text"
						value={config.timeout}
						onChange={(e) => update("timeout", e.target.value)}
						className={NUM_INPUT_CLASS}
					/>
				</div>
				<EditableRow
					label="Max Requests"
					value={config.max_requests}
					onChange={(v) => update("max_requests", v)}
				/>
				<div className="flex items-center justify-between gap-4 py-2">
					<span className="shrink-0 text-[13px] text-t2">Interval</span>
					<TermInput
						type="text"
						value={config.interval}
						onChange={(e) => update("interval", e.target.value)}
						className={NUM_INPUT_CLASS}
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
		<div className="flex flex-col gap-3.5">
			{/* Application (read-only) */}
			<SettingsSection title="APPLICATION" icon={Server}>
				<SettingRow label="Port" value={settings.app.port} />
				<SettingRow label="Version" value={settings.app.version} />
				<SettingRow label="Log Level" value={settings.app.log_level} />
				<SettingRow label="Log Format" value={settings.app.log_format} />
			</SettingsSection>

			{/* Database (read-only) */}
			<SettingsSection title="DATABASE" icon={Database}>
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
			<SettingsSection title="CACHE (VALKEY)" icon={Database}>
				<SettingRow label="URL" value={settings.cache.url} masked />
			</SettingsSection>

			{/* Message Queue (read-only) */}
			<SettingsSection title="MESSAGE QUEUE (NATS)" icon={Inbox}>
				<SettingRow label="URL" value={settings.message_queue.url} masked />
			</SettingsSection>

			{/* JWT (read-only) */}
			<SettingsSection title="JWT AUTHENTICATION" icon={Key}>
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
			<SettingsSection title="TELEGRAM" icon={MessageSquare}>
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
				title="PLUGINS"
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
						<MutationError message={mutationError} />
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
			<SettingsSection title="INFRASTRUCTURE" icon={Activity}>
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
				title="SPEED TEST"
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
						<MutationError message={mutationError} />
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
				title="RATE LIMITING"
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
						<MutationError message={mutationError} />
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
				title="OUTBOX RELAY"
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
						<MutationError message={mutationError} />
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
				title="SMART ROUTER"
				icon={Navigation}
				actions={makeActions("smart_router")}
			>
				{editingSection === "smart_router" ? (
					<>
						<p className="pb-1 pt-2 text-[10px] uppercase tracking-[1.5px] text-t6">
							Default Weights
						</p>
						<EditableRow
							label="Geo"
							value={smartRouterForm.weight_geo}
							onChange={(v) =>
								setSmartRouterForm((prev) => ({ ...prev, weight_geo: v }))
							}
							step="0.01"
						/>
						<EditableRow
							label="Latency"
							value={smartRouterForm.weight_latency}
							onChange={(v) =>
								setSmartRouterForm((prev) => ({ ...prev, weight_latency: v }))
							}
							step="0.01"
						/>
						<EditableRow
							label="Load"
							value={smartRouterForm.weight_load}
							onChange={(v) =>
								setSmartRouterForm((prev) => ({ ...prev, weight_load: v }))
							}
							step="0.01"
						/>

						<p className="pb-1 pt-3 text-[10px] uppercase tracking-[1.5px] text-t6">
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
						/>

						<p className="pb-1 pt-3 text-[10px] uppercase tracking-[1.5px] text-t6">
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
						/>
						<MutationError message={mutationError} />
					</>
				) : (
					<>
						<p className="pb-1 pt-2 text-[10px] uppercase tracking-[1.5px] text-t6">
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
						<p className="pb-1 pt-3 text-[10px] uppercase tracking-[1.5px] text-t6">
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
						<p className="pb-1 pt-3 text-[10px] uppercase tracking-[1.5px] text-t6">
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
				title="FEATURE FLAGS"
				icon={Flag}
				actions={
					successSection === "feature_flags" ? (
						<span className="flex items-center gap-1 text-[10px] uppercase tracking-[1px] text-accent">
							<Check size={12} />
							SAVED
						</span>
					) : updateMutation.isPending && editingSection === null ? (
						<Loader2 size={12} className="animate-spin text-t6" />
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
				title="CIRCUIT BREAKERS"
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
						<MutationError message={mutationError} />
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
							<p className="mb-2 text-[13px] text-t2">Allowed Origins</p>
							<textarea
								value={corsForm}
								onChange={(e) => setCorsForm(e.target.value)}
								placeholder="https://example.com"
								className={TEXTAREA_CLASS}
								style={TEXTAREA_STYLE}
								rows={4}
							/>
							<p className="mt-1 text-[10px] uppercase tracking-[1px] text-t6">
								One origin per line
							</p>
						</div>
						<MutationError message={mutationError} />
					</>
				) : (
					<SettingArray
						label="Allowed Origins"
						values={settings.cors.allowed_origins}
					/>
				)}
			</SettingsSection>

			{/* Tracing (read-only) */}
			<SettingsSection title="TRACING" icon={Signal}>
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
			<Panel className="border-danger">
				<div className="flex flex-col items-center justify-center p-12">
					<p className="text-[12px] uppercase tracking-[1px] text-danger">
						{t("common.errorLoading")}:{" "}
						{error instanceof Error ? error.message : t("common.unknownError")}
					</p>
				</div>
			</Panel>
		);
	}

	if (!settings) {
		return null;
	}

	return (
		<div className="flex flex-col gap-3.5">
			<PageHeader
				title="SETTINGS"
				breadcrumb="REMNAWAVE PROVIDER / SYSTEM / GLOBAL CONFIGURATION"
				right={
					<div className="flex items-center gap-2">
						<StatusPill label={`v${settings.app.version}`} tone="ok" />
						<StatusPill label={`:${settings.app.port}`} tone="muted" />
					</div>
				}
			/>

			<SettingsContent settings={settings} />
		</div>
	);
}
