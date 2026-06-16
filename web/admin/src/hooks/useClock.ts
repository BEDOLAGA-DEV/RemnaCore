import { useEffect, useState } from "react";

function utcNow(): string {
  const d = new Date();
  const h = String(d.getUTCHours()).padStart(2, "0");
  const m = String(d.getUTCMinutes()).padStart(2, "0");
  const s = String(d.getUTCSeconds()).padStart(2, "0");
  return `${h}:${m}:${s}`;
}

/** Live UTC clock as HH:MM:SS, ticking every second. */
export function useClock(): string {
  const [time, setTime] = useState(utcNow);
  useEffect(() => {
    const id = setInterval(() => setTime(utcNow()), 1000);
    return () => clearInterval(id);
  }, []);
  return time;
}
