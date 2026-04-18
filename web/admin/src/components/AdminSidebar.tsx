import { cn, useAuthStore, usePluginPages } from "@remnacore/shared";
import { Link } from "@tanstack/react-router";
import {
	ArrowUpCircle,
	BarChart3,
	Boxes,
	Building2,
	Calculator,
	ChevronDown,
	Clock,
	CreditCard,
	FileText,
	Globe,
	LayoutDashboard,
	type LucideIcon,
	Network,
	Package,
	Plus,
	Puzzle,
	Search,
	Server,
	Settings,
	ShoppingCart,
	Smartphone,
	Tag,
	TrendingUp,
	Users,
	Wallet,
	BookOpen,
	List,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

const NAV_ITEMS = [
	{
		to: "/" as const,
		icon: LayoutDashboard,
		labelKey: "admin.dashboard.title",
	},
	{ to: "/users" as const, icon: Users, labelKey: "admin.users.title" },
	{
		to: "/subscriptions" as const,
		icon: CreditCard,
		labelKey: "admin.subscriptions.title",
	},
	{
		to: "/invoices" as const,
		icon: FileText,
		labelKey: "admin.invoices.title",
	},
	{ to: "/plugins" as const, icon: Puzzle, labelKey: "admin.plugins.title" },
	{ to: "/nodes" as const, icon: Globe, labelKey: "admin.nodes.title" },
	{ to: "/tenants" as const, icon: Building2, labelKey: "admin.tenants.title" },
	{
		to: "/settings" as const,
		icon: Settings,
		labelKey: "admin.settings.title",
	},
] as const;

const ICON_MAP: Record<string, LucideIcon> = {
	ArrowUpCircle,
	BarChart3,
	BookOpen,
	Boxes,
	Building2,
	Calculator,
	Clock,
	CreditCard,
	FileText,
	Globe,
	LayoutDashboard,
	List,
	Network,
	Package,
	Plus,
	Puzzle,
	Search,
	Server,
	Settings,
	ShoppingCart,
	Smartphone,
	Tag,
	TrendingUp,
	Users,
	Wallet,
};

// Human-readable plugin names for sidebar groups.
const PLUGIN_LABELS: Record<string, string> = {
	"tariff-manager": "Tariffs",
	"remnawave-provider": "Remnawave",
	"user-balance": "Balance",
	checkout: "Checkout",
};

// Primary icon per plugin group.
const PLUGIN_ICONS: Record<string, LucideIcon> = {
	"tariff-manager": CreditCard,
	"remnawave-provider": Server,
	"user-balance": Wallet,
	checkout: ShoppingCart,
};

const NAV_LINK_CLASSES = cn(
	"flex items-center gap-2.5 rounded-lg px-2.5 py-[7px] text-[13px] text-muted-foreground transition-colors",
	"hover:bg-secondary hover:text-foreground",
	"[&.active]:bg-primary/20 [&.active]:font-medium [&.active]:text-primary",
);

const SUB_LINK_CLASSES = cn(
	"flex items-center gap-2 rounded-lg py-[5px] pl-8 pr-2.5 text-[12px] text-muted-foreground transition-colors",
	"hover:bg-secondary hover:text-foreground",
	"[&.active]:bg-primary/20 [&.active]:font-medium [&.active]:text-primary",
);

type DynamicIconProps = {
	name: string;
} & React.ComponentProps<LucideIcon>;

function DynamicIcon({ name, ...props }: DynamicIconProps) {
	const Icon = ICON_MAP[name] ?? Puzzle;
	return <Icon size={16} className="shrink-0" {...props} />;
}

function formatUtcTime(date: Date): string {
	const h = String(date.getUTCHours()).padStart(2, "0");
	const m = String(date.getUTCMinutes()).padStart(2, "0");
	const s = String(date.getUTCSeconds()).padStart(2, "0");
	return `${h}:${m}:${s}`;
}

function getUserInitials(user: {
	email: string;
	display_name: string | null;
}): string {
	if (user.display_name) {
		const parts = user.display_name.trim().split(/\s+/);
		if (parts.length >= 2) {
			const first = parts[0]?.[0] ?? "";
			const second = parts[1]?.[0] ?? "";
			return `${first}${second}`.toUpperCase();
		}
		return user.display_name.slice(0, 2).toUpperCase();
	}
	return user.email.slice(0, 2).toUpperCase();
}

function getUserDisplayName(user: {
	email: string;
	display_name: string | null;
}): string {
	return user.display_name ?? user.email.split("@")[0] ?? user.email;
}

function getRoleLabel(role: string): string {
	const ROLE_LABELS: Record<string, string> = {
		admin: "Super admin",
		reseller: "Reseller",
		customer: "Customer",
	};
	return ROLE_LABELS[role] ?? role;
}

type PluginGroup = {
	slug: string;
	label: string;
	icon: LucideIcon;
	pages: Array<{ path: string; title: string; icon: string }>;
};

export function AdminSidebar() {
	const { t } = useTranslation();
	const { user } = useAuthStore();
	const [utcTime, setUtcTime] = useState(() => formatUtcTime(new Date()));
	const { data: pluginPages } = usePluginPages();
	const [expandedPlugins, setExpandedPlugins] = useState<
		Record<string, boolean>
	>({});

	const adminPages = pluginPages?.filter((p) => p.menu === "admin") ?? [];

	// Group pages by plugin_slug for collapsible sections.
	const pluginGroups = useMemo<PluginGroup[]>(() => {
		const groups = new Map<string, PluginGroup>();
		for (const page of adminPages) {
			if (!groups.has(page.plugin_slug)) {
				groups.set(page.plugin_slug, {
					slug: page.plugin_slug,
					label:
						PLUGIN_LABELS[page.plugin_slug] ?? page.plugin_slug,
					icon: PLUGIN_ICONS[page.plugin_slug] ?? Puzzle,
					pages: [],
				});
			}
			groups.get(page.plugin_slug)!.pages.push({
				path: page.path,
				title: page.title,
				icon: page.icon,
			});
		}
		return Array.from(groups.values());
	}, [adminPages]);

	const togglePlugin = (slug: string) => {
		setExpandedPlugins((prev) => ({ ...prev, [slug]: !prev[slug] }));
	};

	useEffect(() => {
		const interval = setInterval(() => {
			setUtcTime(formatUtcTime(new Date()));
		}, 1000);
		return () => clearInterval(interval);
	}, []);

	return (
		<aside className="flex w-[220px] shrink-0 flex-col border-r border-border bg-card">
			{/* Logo */}
			<div className="px-4 pb-3 pt-5">
				<div className="flex items-center gap-2.5">
					<svg
						width="20"
						height="20"
						viewBox="0 0 20 20"
						fill="none"
						aria-hidden="true"
					>
						<defs>
							<linearGradient
								id="logo-grad"
								x1="0"
								y1="0"
								x2="20"
								y2="20"
								gradientUnits="userSpaceOnUse"
							>
								<stop stopColor="hsl(187 86% 53%)" />
								<stop offset="1" stopColor="hsl(210 80% 60%)" />
							</linearGradient>
						</defs>
						<path
							d="M10 1L18.66 10L10 19L1.34 10L10 1Z"
							fill="url(#logo-grad)"
						/>
					</svg>
					<span className="text-[15px] font-bold tracking-tight text-foreground">
						Remna
					</span>
				</div>
				<span className="mt-0.5 block pl-[30px] font-mono text-[11px] text-dim">
					{t("admin.nerveCenter")}
				</span>
			</div>

			{/* Search placeholder */}
			<div className="px-3 pb-2 pt-1">
				<div className="flex items-center gap-2 rounded-lg bg-secondary px-2.5 py-[7px]">
					<Search size={14} className="shrink-0 text-muted-foreground" />
					<span className="text-[12px] text-muted-foreground">
						{t("admin.searchPlaceholder")}
					</span>
				</div>
			</div>

			{/* Navigation */}
			<nav className="flex flex-1 flex-col gap-0.5 overflow-y-auto px-2 pt-2">
				{NAV_ITEMS.map((item) => (
					<Link key={item.to} to={item.to} className={NAV_LINK_CLASSES}>
						<item.icon size={16} className="shrink-0" />
						<span>{t(item.labelKey)}</span>
					</Link>
				))}

				{/* Plugin groups — collapsible sections */}
				{pluginGroups.length > 0 && (
					<div className="mx-2.5 my-2 border-t border-border-subtle" />
				)}

				{pluginGroups.map((group) => {
					const isExpanded = expandedPlugins[group.slug] ?? false;
					const GroupIcon = group.icon;

					// Single-page plugin — render as direct link, no accordion.
					if (group.pages.length === 1) {
						const page = group.pages[0]!;
						return (
							<Link
								key={group.slug}
								to="/plugins/$slug/page/$pagePath"
								params={{ slug: group.slug, pagePath: page.path }}
								className={NAV_LINK_CLASSES}
							>
								<GroupIcon size={16} className="shrink-0" />
								<span>{group.label}</span>
							</Link>
						);
					}

					return (
						<div key={group.slug}>
							{/* Group header — click to expand/collapse */}
							<button
								type="button"
								onClick={() => togglePlugin(group.slug)}
								className={cn(
									NAV_LINK_CLASSES,
									"w-full justify-between",
								)}
							>
								<span className="flex items-center gap-2.5">
									<GroupIcon size={16} className="shrink-0" />
									<span>{group.label}</span>
								</span>
								<ChevronDown
									size={14}
									className={cn(
										"shrink-0 text-muted-foreground transition-transform duration-200",
										isExpanded && "rotate-180",
									)}
								/>
							</button>

							{/* Nested pages */}
							{isExpanded && (
								<div className="flex flex-col gap-0.5 pb-0.5">
									{group.pages.map((page) => (
										<Link
											key={`${group.slug}-${page.path}`}
											to="/plugins/$slug/page/$pagePath"
											params={{
												slug: group.slug,
												pagePath: page.path,
											}}
											className={SUB_LINK_CLASSES}
										>
											<DynamicIcon name={page.icon} size={14} />
											<span>{page.title}</span>
										</Link>
									))}
								</div>
							)}
						</div>
					);
				})}
			</nav>

			{/* Bottom section */}
			<div className="mt-auto border-t border-border-subtle px-3 pb-4 pt-3">
				{/* System status */}
				<div className="flex items-center gap-1.5 pb-1">
					<span className="inline-block h-1.5 w-1.5 rounded-full bg-success" />
					<span className="text-[11px] text-muted-foreground">
						{t("admin.allSystemsNominal")}
					</span>
				</div>
				<span className="block pb-3 pl-3 font-mono text-[11px] text-dim">
					{utcTime} UTC
				</span>

				{/* User section */}
				{user ? (
					<div className="flex items-center gap-2.5">
						<div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-primary/20 text-[11px] font-semibold text-primary">
							{getUserInitials(user)}
						</div>
						<div className="min-w-0">
							<p className="truncate text-[12px] font-medium text-foreground">
								{getUserDisplayName(user)}
							</p>
							<p className="text-[11px] text-dim">{getRoleLabel(user.role)}</p>
						</div>
					</div>
				) : null}
			</div>
		</aside>
	);
}
