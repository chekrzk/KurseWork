import {
  ResponsiveContainer,
  LineChart,
  Line,
  CartesianGrid,
  XAxis,
  YAxis,
  Tooltip,
  BarChart,
  Bar,
  PieChart,
  Pie,
  Cell,
} from "recharts";
import type { Report, Group } from "./product";
export function Trend({ report }: { report: Report }) {
  const points = report.series.map((p) => ({
    ...p,
    label: new Date(p.bucket).toLocaleString("ru", {
      timeZone: report.timezone,
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    }),
  }));
  return (
    <ResponsiveContainer width="100%" height={230}>
      <LineChart data={points}>
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis dataKey="label" minTickGap={32} />
        <YAxis allowDecimals={false} />
        <Tooltip />
        <Line
          type="linear"
          dataKey="value"
          name="Количество"
          stroke="#5945bb"
          strokeWidth={3}
          isAnimationActive={false}
        />
      </LineChart>
    </ResponsiveContainer>
  );
}
export function Distribution({
  rows,
  donut,
}: {
  rows: Group[];
  donut?: boolean;
}) {
  if (!rows.length) return null;
  return (
    <ResponsiveContainer width="100%" height={160}>
      {donut ? (
        <PieChart>
          <Pie
            data={rows}
            dataKey="value"
            nameKey="key"
            innerRadius={35}
            outerRadius={65}
            isAnimationActive={false}
          >
            {rows.map((r, i) => (
              <Cell
                key={r.key}
                fill={["#5945bb", "#549555", "#487ab7", "#bd8040"][i % 4]}
              />
            ))}
          </Pie>
          <Tooltip />
        </PieChart>
      ) : (
        <BarChart data={rows.slice(0, 8)}>
          <XAxis dataKey="key" hide />
          <YAxis allowDecimals={false} />
          <Bar
            dataKey="value"
            name="Переходы"
            fill="#7868ea"
            isAnimationActive={false}
          />
          <Tooltip />
        </BarChart>
      )}
    </ResponsiveContainer>
  );
}
