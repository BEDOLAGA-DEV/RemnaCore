import { cn } from "@remnacore/shared";
import {
  type ColumnDef,
  type RowData,
  tableFeatures,
  useTable,
} from "@tanstack/react-table";
import { useTranslation } from "react-i18next";

// react-table v9 requires the feature set to be declared explicitly. This table
// only renders rows — sorting, filtering and pagination are done server-side —
// so the set is empty. The core row model is wired up automatically in v9 and is
// no longer passed as an option.
const features = tableFeatures({});

// Column type for DataTable. Callers should use this instead of ColumnDef so the
// feature set stays in one place.
export type DataTableColumn<TData extends RowData> = ColumnDef<
  typeof features,
  TData
>;

type DataTableProps<TData extends RowData> = {
  data: TData[];
  columns: DataTableColumn<TData>[];
  isLoading?: boolean;
};

export function DataTable<TData extends RowData>({
  data,
  columns,
  isLoading,
}: DataTableProps<TData>) {
  const { t } = useTranslation();

  const table = useTable({
    features,
    data,
    columns,
  });

  return (
    <div className="rounded-xl border border-border bg-card overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            {table.getHeaderGroups().map((headerGroup) => (
              <tr
                key={headerGroup.id}
                className="border-b border-border bg-card"
              >
                {headerGroup.headers.map((header) => (
                  <th
                    key={header.id}
                    className="whitespace-nowrap px-3 py-2.5 text-left text-[11px] font-medium uppercase tracking-wider text-muted-foreground"
                  >
                    {header.isPlaceholder ? null : (
                      <table.FlexRender header={header} />
                    )}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {isLoading ? (
              <tr>
                <td
                  colSpan={columns.length}
                  className="px-4 py-8 text-center text-sm font-mono text-muted-foreground"
                >
                  {t("common.loading")}
                </td>
              </tr>
            ) : table.getRowModel().rows.length === 0 ? (
              <tr>
                <td
                  colSpan={columns.length}
                  className="px-4 py-8 text-center text-sm text-muted-foreground"
                >
                  {t("common.noResults")}
                </td>
              </tr>
            ) : (
              table.getRowModel().rows.map((row) => (
                <tr
                  key={row.id}
                  className={cn(
                    "border-b border-border/50 transition-colors hover:bg-secondary",
                    "last:border-0",
                  )}
                >
                  {row.getAllCells().map((cell) => (
                    <td
                      key={cell.id}
                      className="max-w-[220px] truncate px-3 py-2.5 text-sm text-foreground"
                    >
                      <table.FlexRender cell={cell} />
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
