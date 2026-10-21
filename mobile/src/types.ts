export type Reason = "not_responding" | "unwell" | "self_harm" | "missing";

export type LocationPoint = {
  latitude: number;
  longitude: number;
  accuracyM: number;
  recordedAt: string;
  stale: boolean;
};

export type Place = {
  id: string;
  label: string;
  latitude: number;
  longitude: number;
};

export type SafetyCheck = {
  id: string;
  personId: string;
  personName: string;
  startedBy: string;
  startedByName: string;
  reason: Reason;
  status: "pending" | "confirmed_safe" | "unanswered";
  startedAt: string;
  respondBy: string;
  resolvedAt?: string | null;
  confirmedByName?: string;
};

export type Identity = {
  legalName: string;
  dateOfBirth: string;
  documentType: "passport" | "national_id" | "drivers_license";
  documentLast4: string;
  status: "id_on_file";
};

export type Me = {
  id: string;
  displayName: string;
  idOnFile: boolean;
  identity?: Identity;
  supervisorLimit: number;
  activeSupervisors: number;
  sharingEnabled: boolean;
  lastLocation?: LocationPoint;
};

export type Supervisor = {
  linkId: string;
  userId: string;
  displayName: string;
};

export type Watched = {
  personId: string;
  displayName: string;
  sharingEnabled: boolean;
  lastLocation?: LocationPoint;
  check?: SafetyCheck;
};

export type PersonDetail = Watched & {
  trail: LocationPoint[];
  places: Place[];
};

export type LiveEvent = {
  type: "location" | "sharing" | "check";
  personId: string;
  latitude?: number;
  longitude?: number;
  accuracyM?: number;
  recordedAt?: string;
  enabled?: boolean;
  check?: SafetyCheck;
};

export type RootStackParamList = {
  Welcome: undefined;
  Register: undefined;
  Login: undefined;
  Id: undefined;
  Main: undefined;
  Person: { personId: string };
};

export type TabParamList = {
  Circle: undefined;
  Watch: undefined;
};
