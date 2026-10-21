import type { LocationObject } from "expo-location";
import type { Identity, Me, PersonDetail, Place, Reason, SafetyCheck, Supervisor, Watched } from "./types";

export const API_URL = process.env.EXPO_PUBLIC_API_URL ?? "http://127.0.0.1:43123";

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

type Options = {
  token?: string;
  method?: string;
  body?: unknown;
  form?: FormData;
};

async function request<T>(path: string, options: Options = {}): Promise<T> {
  const headers: Record<string, string> = {};
  if (options.token) headers.Authorization = `Bearer ${options.token}`;
  let body: BodyInit | undefined;
  if (options.form) {
    body = options.form;
  } else if (options.body !== undefined) {
    headers["Content-Type"] = "application/json";
    body = JSON.stringify(options.body);
  }
  let res: Response;
  try {
    res = await fetch(`${API_URL}${path}`, { method: options.method ?? "GET", headers, body });
  } catch {
    throw new ApiError("Can't reach Circle.", 0);
  }
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  const data = text ? JSON.parse(text) : {};
  if (!res.ok) throw new ApiError(data.error || "Something went wrong.", res.status);
  return data as T;
}

export const api = {
  register: (displayName: string, passphrase: string) =>
    request<{ token: string }>("/v1/auth/register", { method: "POST", body: { displayName, passphrase } }),
  login: (displayName: string, passphrase: string) =>
    request<{ token: string }>("/v1/auth/login", { method: "POST", body: { displayName, passphrase } }),
  logout: (token: string) => request<void>("/v1/auth/logout", { method: "POST", token }),
  me: (token: string) => request<Me>("/v1/me", { token }),
  submitId: (token: string, form: FormData) =>
    request<{ idOnFile: boolean; identity: Identity }>("/v1/me/id", { method: "POST", token, form }),
  setLimit: (token: string, supervisorLimit: number) =>
    request<{ supervisorLimit: number; activeSupervisors: number }>("/v1/circle/limit", {
      method: "PUT",
      token,
      body: { supervisorLimit },
    }),
  createInvite: (token: string) => request<{ code: string }>("/v1/circle/invites", { method: "POST", token }),
  supervisors: (token: string) => request<{ supervisors: Supervisor[] }>("/v1/circle/supervisors", { token }),
  revoke: (token: string, linkId: string) =>
    request<void>(`/v1/circle/supervisors/${linkId}`, { method: "DELETE", token }),
  acceptInvite: (token: string, code: string) =>
    request<{ personId: string }>("/v1/invites/accept", { method: "POST", token, body: { code } }),
  setSharing: (token: string, enabled: boolean) =>
    request<{ enabled: boolean }>("/v1/circle/sharing", { method: "PUT", token, body: { enabled } }),
  postLocation: (token: string, loc: LocationObject) =>
    request<{ stored: boolean; live: boolean }>("/v1/location", {
      method: "POST",
      token,
      body: {
        latitude: loc.coords.latitude,
        longitude: loc.coords.longitude,
        accuracyM: loc.coords.accuracy ?? 0,
      },
    }),
  places: (token: string) => request<{ places: Place[] }>("/v1/places", { token }),
  createPlace: (token: string, place: { label: string; latitude: number; longitude: number }) =>
    request<Place>("/v1/places", { method: "POST", token, body: place }),
  deletePlace: (token: string, placeId: string) => request<void>(`/v1/places/${placeId}`, { method: "DELETE", token }),
  watching: (token: string) => request<{ people: Watched[] }>("/v1/watching", { token }),
  person: (token: string, personId: string) => request<PersonDetail>(`/v1/people/${personId}`, { token }),
  startCheck: (token: string, personId: string, reason: Reason) =>
    request<{ check: SafetyCheck }>(`/v1/people/${personId}/checks`, { method: "POST", token, body: { reason } }),
  pendingCheck: (token: string) => request<{ check: SafetyCheck | null }>("/v1/checks/pending", { token }),
  confirmCheck: (token: string, checkId: string) =>
    request<{ check: SafetyCheck }>(`/v1/checks/${checkId}/confirm`, { method: "POST", token, body: {} }),
};

export function socketUrl(token: string): string {
  const base = API_URL.replace(/^http/, "ws");
  return `${base}/v1/ws?access_token=${encodeURIComponent(token)}`;
}
