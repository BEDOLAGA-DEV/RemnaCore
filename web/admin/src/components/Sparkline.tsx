import { useMemo } from "react";

type SparklineProps = {
  data: number[];
  color?: string;
  width?: number;
  height?: number;
};

const SPARKLINE_PADDING = 2;

export function Sparkline({
  data,
  color = "hsl(var(--primary))",
  width = 80,
  height = 28,
}: SparklineProps) {
  const { polylinePoints, areaPoints, gradientId } = useMemo(() => {
    if (data.length < 2) {
      return { polylinePoints: "", areaPoints: "", gradientId: "" };
    }

    const min = Math.min(...data);
    const max = Math.max(...data);
    const range = max - min || 1;

    const stepX = (width - SPARKLINE_PADDING * 2) / (data.length - 1);
    const usableHeight = height - SPARKLINE_PADDING * 2;

    const points = data.map((value, index) => {
      const x = SPARKLINE_PADDING + index * stepX;
      const y =
        height - SPARKLINE_PADDING - ((value - min) / range) * usableHeight;
      return `${x},${y}`;
    });

    const polyline = points.join(" ");
    const firstX = SPARKLINE_PADDING;
    const lastX = SPARKLINE_PADDING + (data.length - 1) * stepX;
    const area = `${firstX},${height} ${polyline} ${lastX},${height}`;

    const id = `sparkline-grad-${Math.random().toString(36).slice(2, 8)}`;

    return { polylinePoints: polyline, areaPoints: area, gradientId: id };
  }, [data, width, height]);

  if (data.length < 2) {
    return null;
  }

  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      className="overflow-visible"
    >
      <defs>
        <linearGradient id={gradientId} x1="0" x2="0" y1="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity={0.3} />
          <stop offset="100%" stopColor={color} stopOpacity={0} />
        </linearGradient>
      </defs>
      <polygon points={areaPoints} fill={`url(#${gradientId})`} />
      <polyline
        points={polylinePoints}
        fill="none"
        stroke={color}
        strokeWidth={1.5}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
