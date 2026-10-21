import * as Location from "expo-location";
import { createContext, ReactNode, useContext, useEffect } from "react";
import { Platform } from "react-native";
import { api } from "../api";
import { useAuth } from "../auth";
import { LOCATION_TASK } from "./task";

const BackgroundNoteContext = createContext(false);

export function BackgroundNoteProvider({ value, children }: { value: boolean; children: ReactNode }) {
  return <BackgroundNoteContext.Provider value={value}>{children}</BackgroundNoteContext.Provider>;
}

export function useBackgroundLimited(): boolean {
  return useContext(BackgroundNoteContext);
}

export function LocationBridge({ onBackground }: { onBackground: (limited: boolean) => void }) {
  const { token, me } = useAuth();
  const sharing = Boolean(me?.sharingEnabled && me.idOnFile);

  useEffect(() => {
    if (!token || !sharing) {
      onBackground(false);
      if (Platform.OS !== "web") {
        Location.hasStartedLocationUpdatesAsync(LOCATION_TASK)
          .then((started) => (started ? Location.stopLocationUpdatesAsync(LOCATION_TASK) : undefined))
          .catch(() => undefined);
      }
      return;
    }

    let subscription: Location.LocationSubscription | null = null;
    let cancelled = false;

    (async () => {
      const foreground = await Location.requestForegroundPermissionsAsync();
      if (foreground.status !== "granted" || cancelled) {
        onBackground(false);
        return;
      }
      subscription = await Location.watchPositionAsync(
        { accuracy: Location.Accuracy.High, timeInterval: 60000, distanceInterval: 50 },
        (position) => {
          api.postLocation(token, position).catch(() => undefined);
        },
      );
      try {
        const current = await Location.getCurrentPositionAsync({ accuracy: Location.Accuracy.High });
        if (!cancelled) await api.postLocation(token, current);
      } catch {
        // Permission can be granted before a fix is available.
      }
      if (Platform.OS === "web") {
        onBackground(true);
        return;
      }
      try {
        const background = await Location.requestBackgroundPermissionsAsync();
        if (cancelled) return;
        if (background.status !== "granted") {
          onBackground(true);
          return;
        }
        const started = await Location.hasStartedLocationUpdatesAsync(LOCATION_TASK);
        if (!started) {
          await Location.startLocationUpdatesAsync(LOCATION_TASK, {
            accuracy: Location.Accuracy.High,
            timeInterval: 60000,
            distanceInterval: 50,
            showsBackgroundLocationIndicator: true,
            foregroundService: {
              notificationTitle: "Circle is sharing your location",
              notificationBody: "Your supervisors can see your precise location.",
            },
          });
        }
        onBackground(false);
      } catch {
        onBackground(true);
      }
    })();

    return () => {
      cancelled = true;
      subscription?.remove();
    };
  }, [token, sharing, onBackground]);

  return null;
}
