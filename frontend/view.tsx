import { useT } from "@margince/frontend/app";
import { Button, SegmentedControl } from "@margince/frontend/design-system";
import { createContext, type ReactNode, useContext, useState } from "react";

// How the tables on one request's page are laid out: in the order things
// started or slowest first, and whether each row's start offset shows. One
// choice for every tab, so switching tabs keeps the reader's view.

export const ORDERS = ["time", "duration"] as const;
export type Order = (typeof ORDERS)[number];

type View = Readonly<{ order: Order; showAt: boolean }>;

const ViewContext = createContext<View>({ order: "time", showAt: false });

export function useView(): View {
  return useContext(ViewContext);
}

/** Rows in the chosen order; the slowest first when ordered by duration. */
export function ordered<Row extends { at_ms: number; duration_ms?: number }>(
  rows: readonly Row[],
  order: Order,
): Row[] {
  const sorted = [...rows];
  if (order === "duration") {
    return sorted.sort((a, b) => (b.duration_ms ?? 0) - (a.duration_ms ?? 0));
  }
  return sorted.sort((a, b) => a.at_ms - b.at_ms);
}

/** Leaves the start-offset column out unless the reader asked for it. */
export function withAt<Column extends { key: string }>(
  columns: Column[],
  showAt: boolean,
): Column[] {
  return showAt ? columns : columns.filter((c) => c.key !== "at");
}

export function ViewControls({ children }: Readonly<{ children: ReactNode }>) {
  const t = useT();
  const [order, setOrder] = useState<Order>("time");
  const [showAt, setShowAt] = useState(false);
  return (
    <ViewContext.Provider value={{ order, showAt }}>
      <div
        style={{
          display: "flex",
          gap: "1rem",
          alignItems: "center",
          flexWrap: "wrap",
        }}
      >
        <SegmentedControl<Order>
          label={t("extProfiler.order")}
          options={ORDERS}
          value={order}
          onChange={setOrder}
          labels={{
            time: t("extProfiler.order.time"),
            duration: t("extProfiler.order.duration"),
          }}
        />
        <Button variant="link" onClick={() => setShowAt(!showAt)}>
          {showAt ? t("extProfiler.at.hide") : t("extProfiler.at.show")}
        </Button>
      </div>
      {children}
    </ViewContext.Provider>
  );
}
