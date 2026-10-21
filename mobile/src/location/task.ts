import * as Location from "expo-location";
import * as TaskManager from "expo-task-manager";
import { api } from "../api";
import { getToken } from "../session";

export const LOCATION_TASK = "circle-location";

try {
  TaskManager.defineTask(LOCATION_TASK, async ({ data, error }) => {
    if (error || !data) return;
    const token = await getToken();
    if (!token) return;
    const locations = (data as { locations?: Location.LocationObject[] }).locations ?? [];
    const latest = locations[locations.length - 1];
    if (!latest) return;
    try {
      await api.postLocation(token, latest);
    } catch {
      // The next fix will try again.
    }
  });
} catch {
  // Web and some Expo Go sessions cannot register a background task.
}
