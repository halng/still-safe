import { createElement } from "react";
import { StyleSheet, View } from "react-native";

export function MapPanel({ html }: { html: string }) {
  return (
    <View style={styles.wrap}>
      {createElement("iframe", {
        srcDoc: html,
        title: "Location map",
        style: { border: "0", width: "100%", height: "100%", background: "#fff" },
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { flex: 1, backgroundColor: "#FFFFFF" },
});
