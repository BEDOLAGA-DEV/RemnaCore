import { apiGet, apiPost, cn, LoadingSpinner } from "@remnacore/shared";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
	AlertTriangle,
	Globe,
	Loader2,
	MonitorX,
	RefreshCw,
	Search,
	Unplug,
	Wifi,
} from "lucide-react";
import { useCallback, useEffect, useState } from "react";

// ─── Types ──────────────────────────────────────────────────────────────────

type UserEntry = {
	panel_id: string;
	user: {
		uuid: string;
		username: string;
		status: string;
	};
};

type IPFetchJob = {
	panel_id: string;
	user_uuid: string;
	job_id: string;
	status: string;
};

type IPFetchResult = {
	panel_id: string;
	result: {
		jobId: string;
		status: string;
		ips?: string[];
	};
};

type ActiveJob = {
	panelID: string;
	userUUID: string;
	username: string;
	jobID: string;
	status: "fetching" | "done" | "error";
	ips: string[];
	error?: string;
};

// ─── Helpers ────────────────────────────────────────────────────────────────

const QUERY_KEY = ["remnawave", "users-for-ip"];
const POLL_INTERVAL_MS = 2000;

// ─── Component ──────────────────────────────────────────────────────────────

export default function RemnawaveIPManagement() {
	const queryClient = useQueryClient();
	const [search, setSearch] = useState("");
	const [debouncedSearch, setDebouncedSearch] = useState("");
	const [activeJobs, setActiveJobs] = useState<ActiveJob[]>([]);

	// Debounce search
	useEffect(() => {
		const timer = setTimeout(() => setDebouncedSearch(search), 300);
		return () => clearTimeout(timer);
	}, [search]);

	// Fetch users
	const {
		data: users,
		isLoading,
		isError,
	} = useQuery({
		queryKey: [...QUERY_KEY, debouncedSearch],
		queryFn: () => {
			const params = debouncedSearch ? `?q=${encodeURIComponent(debouncedSearch)}` : "";
			return apiGet<UserEntry[]>(`/api/remnawave/users${params}`);
		},
	});

	// Fetch IPs for a user
	const fetchIPsMutation = useMutation({
		mutationFn: async ({
			panelID,
			userUUID,
			username,
		}: {
			panelID: string;
			userUUID: string;
			username: string;
		}) => {
			const result = await apiPost<IPFetchJob>(
				`/api/remnawave/users/${panelID}/${userUUID}/sessions`,
				{},
			);
			return { ...result, username };
		},
		onSuccess: (data) => {
			setActiveJobs((prev) => [
				...prev,
				{
					panelID: data.panel_id,
					userUUID: data.user_uuid,
					username: data.username,
					jobID: data.job_id,
					status: "fetching",
					ips: [],
				},
			]);
		},
	});

	// Drop connections for a user
	const dropMutation = useMutation({
		mutationFn: async ({
			panelID,
			userUUID,
		}: {
			panelID: string;
			userUUID: string;
		}) => {
			return apiPost<{ action: string }>(
				`/api/remnawave/users/${panelID}/${userUUID}/disconnect`,
				{},
			);
		},
	});

	// Poll for job results
	useEffect(() => {
		const pendingJobs = activeJobs.filter((j) => j.status === "fetching");
		if (pendingJobs.length === 0) return;

		const interval = setInterval(async () => {
			for (const job of pendingJobs) {
				try {
					const result = await apiGet<IPFetchResult>(
						`/api/remnawave/users/${job.panelID}/ip-fetch-result/${job.jobID}`,
					);
					if (result.result?.ips) {
						setActiveJobs((prev) =>
							prev.map((j) =>
								j.jobID === job.jobID
									? { ...j, status: "done", ips: result.result.ips ?? [] }
									: j,
							),
						);
					}
				} catch {
					setActiveJobs((prev) =>
						prev.map((j) =>
							j.jobID === job.jobID
								? { ...j, status: "error", error: "Failed to fetch result" }
								: j,
						),
					);
				}
			}
		}, POLL_INTERVAL_MS);

		return () => clearInterval(interval);
	}, [activeJobs]);

	const handleFetchIPs = useCallback(
		(panelID: string, userUUID: string, username: string) => {
			// Don't start duplicate jobs
			if (activeJobs.some((j) => j.userUUID === userUUID && j.status === "fetching")) return;
			fetchIPsMutation.mutate({ panelID, userUUID, username });
		},
		[activeJobs, fetchIPsMutation],
	);

	const handleDropConnections = useCallback(
		(panelID: string, userUUID: string) => {
			if (!window.confirm("Drop all active connections for this user?")) return;
			dropMutation.mutate({ panelID, userUUID });
		},
		[dropMutation],
	);

	const clearJob = useCallback((jobID: string) => {
		setActiveJobs((prev) => prev.filter((j) => j.jobID !== jobID));
	}, []);

	if (isLoading) return <LoadingSpinner />;

	return (
		<div className="space-y-6">
			{/* Header */}
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-[15px] font-semibold text-foreground">
						IP Management
					</h1>
					<p className="mt-0.5 text-[11px] text-muted-foreground">
						Fetch user IPs, view active connections, and drop sessions
					</p>
				</div>
			</div>

			{/* Active IP fetch results */}
			{activeJobs.length > 0 && (
				<div className="space-y-3">
					<h2 className="text-[13px] font-medium text-foreground">
						IP Fetch Results
					</h2>
					{activeJobs.map((job) => (
						<div
							key={job.jobID}
							className="rounded-xl border border-border bg-card p-4"
						>
							<div className="flex items-center justify-between">
								<div className="flex items-center gap-2">
									{job.status === "fetching" ? (
										<Loader2 size={14} className="animate-spin text-primary" />
									) : job.status === "done" ? (
										<Wifi size={14} className="text-emerald-500" />
									) : (
										<AlertTriangle size={14} className="text-destructive" />
									)}
									<span className="font-mono text-[13px] font-medium text-foreground">
										{job.username}
									</span>
									<span className="text-[11px] text-muted-foreground">
										{job.status === "fetching"
											? "Fetching IPs..."
											: job.status === "done"
												? `${job.ips.length} IP(s) found`
												: job.error ?? "Error"}
									</span>
								</div>
								<div className="flex items-center gap-1">
									{job.status === "done" && job.ips.length > 0 && (
										<button
											type="button"
											onClick={() => handleDropConnections(job.panelID, job.userUUID)}
											disabled={dropMutation.isPending}
											className="flex items-center gap-1 rounded-lg px-2 py-1 text-[11px] font-medium text-destructive transition-colors hover:bg-destructive/10"
											title="Drop all connections"
										>
											{dropMutation.isPending ? (
												<Loader2 size={12} className="animate-spin" />
											) : (
												<Unplug size={12} />
											)}
											Drop
										</button>
									)}
									<button
										type="button"
										onClick={() => clearJob(job.jobID)}
										className="rounded-lg p-1 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
										title="Dismiss"
									>
										<MonitorX size={14} />
									</button>
								</div>
							</div>

							{/* IP list */}
							{job.status === "done" && job.ips.length > 0 && (
								<div className="mt-3 flex flex-wrap gap-2">
									{job.ips.map((ip) => (
										<span
											key={ip}
											className="rounded-md bg-secondary px-2 py-0.5 font-mono text-[11px] text-foreground"
										>
											{ip}
										</span>
									))}
								</div>
							)}
							{job.status === "done" && job.ips.length === 0 && (
								<p className="mt-2 text-[11px] text-muted-foreground">
									No active connections found
								</p>
							)}
						</div>
					))}
				</div>
			)}

			{/* Search */}
			<div className="flex items-center gap-2 rounded-lg border border-border bg-background px-3 py-2">
				<Search size={14} className="shrink-0 text-muted-foreground" />
				<input
					type="text"
					value={search}
					onChange={(e) => setSearch(e.target.value)}
					placeholder="Search by username..."
					className="w-full bg-transparent text-[13px] text-foreground placeholder:text-muted-foreground/50 outline-none"
				/>
			</div>

			{/* Error */}
			{isError && (
				<div className="rounded-xl border border-destructive/20 bg-destructive/5 p-6 text-center">
					<AlertTriangle size={24} className="mx-auto mb-2 text-destructive" />
					<p className="text-[13px] text-destructive">Failed to load users</p>
				</div>
			)}

			{/* User table */}
			{users && users.length > 0 ? (
				<div className="rounded-xl border border-border bg-card">
					<table className="w-full">
						<thead>
							<tr className="border-b border-border">
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
									Username
								</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
									Panel
								</th>
								<th className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
									Status
								</th>
								<th className="px-4 py-3 text-right text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
									Actions
								</th>
							</tr>
						</thead>
						<tbody>
							{users.map((entry) => {
								const user = entry.user;
								const isJobPending = activeJobs.some(
									(j) => j.userUUID === user.uuid && j.status === "fetching",
								);

								return (
									<tr
										key={`${entry.panel_id}-${user.uuid}`}
										className="border-b border-border/50 last:border-0 transition-colors hover:bg-secondary/30"
									>
										<td className="px-4 py-3">
											<span className="font-mono text-[13px] font-medium text-foreground">
												{user.username}
											</span>
										</td>
										<td className="px-4 py-3">
											<span className="font-mono text-[11px] text-muted-foreground">
												{entry.panel_id}
											</span>
										</td>
										<td className="px-4 py-3">
											<StatusBadge status={user.status} />
										</td>
										<td className="px-4 py-3">
											<div className="flex items-center justify-end gap-1">
												<button
													type="button"
													onClick={() =>
														handleFetchIPs(entry.panel_id, user.uuid, user.username)
													}
													disabled={isJobPending}
													className="flex items-center gap-1 rounded-lg px-2 py-1 text-[11px] font-medium text-primary transition-colors hover:bg-primary/10"
													title="Fetch user IPs"
												>
													{isJobPending ? (
														<Loader2 size={12} className="animate-spin" />
													) : (
														<Globe size={12} />
													)}
													Fetch IPs
												</button>
												<button
													type="button"
													onClick={() =>
														handleDropConnections(entry.panel_id, user.uuid)
													}
													disabled={dropMutation.isPending}
													className="flex items-center gap-1 rounded-lg px-2 py-1 text-[11px] font-medium text-destructive transition-colors hover:bg-destructive/10"
													title="Drop all connections"
												>
													<Unplug size={12} />
													Drop
												</button>
											</div>
										</td>
									</tr>
								);
							})}
						</tbody>
					</table>
				</div>
			) : !isLoading && !isError ? (
				<div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
					<Globe size={32} className="mb-3 text-muted-foreground/50" />
					<p className="text-[13px] text-muted-foreground">
						{debouncedSearch
							? "No users found matching your search"
							: "No users available"}
					</p>
				</div>
			) : null}
		</div>
	);
}

// ─── Status Badge ───────────────────────────────────────────────────────────

const STATUS_COLORS: Record<string, string> = {
	active: "bg-emerald-500/10 text-emerald-500",
	disabled: "bg-red-500/10 text-red-500",
	expired: "bg-yellow-500/10 text-yellow-500",
	limited: "bg-orange-500/10 text-orange-500",
};

function StatusBadge({ status }: { status: string }) {
	const colorClass = STATUS_COLORS[status] ?? "bg-muted text-muted-foreground";
	return (
		<span
			className={cn(
				"inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium",
				colorClass,
			)}
		>
			{status}
		</span>
	);
}
