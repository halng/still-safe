import type { Reason } from "./types";

export const reasonLabel: Record<Reason, string> = {
  not_responding: "Not responding",
  unwell: "Seems unwell",
  self_harm: "Worried about self-harm",
  missing: "Missing",
};

export const reasonDetail: Record<Reason, string> = {
  not_responding: "They have not heard from you.",
  unwell: "They think you may be unwell.",
  self_harm: "They are worried you may harm yourself.",
  missing: "They think you are missing.",
};

export function ageLabel(iso?: string): string {
  if (!iso) return "No location yet";
  const minutes = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 60000));
  if (minutes < 1) return "Just now";
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} hr ago`;
  return `${Math.round(hours / 24)} days ago`;
}

export function accuracyLabel(meters?: number): string {
  if (meters === undefined || meters === null) return "";
  return `±${Math.round(meters)} m`;
}

export function clock(iso?: string | null): string {
  if (!iso) return "";
  return new Date(iso).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

export function countdown(respondBy: string, now = Date.now()): string {
  const seconds = Math.max(0, Math.ceil((new Date(respondBy).getTime() - now) / 1000));
  const minutes = Math.floor(seconds / 60);
  const rest = seconds % 60;
  return `${minutes}:${rest.toString().padStart(2, "0")}`;
}
