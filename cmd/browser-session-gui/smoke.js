// Exercises the real native WebView, Wails binding, and local Go store.
// No browser accounts, remote content, or user profile data are involved.
(async () => {
  const get = (id) => document.getElementById(id);
  const until = async (test, label) => {
    for (let n = 0; n < 150; n++) {
      if (test()) return;
      await new Promise((r) => setTimeout(r, 100));
    }
    throw Error("Timed out: " + label);
  };
  try {
    await until(
      () => window.go?.main?.Smoke && get("total").textContent === "0",
      "native binding and initial empty state",
    );
    const exe = await window.go.main.Smoke.Executable();
    get("new-button").click();
    await until(() => get("create-dialog").open, "create dialog");
    get("session-name").value = "ui-smoke";
    get("browser-choice").value = "custom";
    get("browser-choice").dispatchEvent(new Event("change"));
    get("browser-path").value = exe;
    get("launch-now").checked = false;
    get("launch-now").dispatchEvent(new Event("change"));
    get("create-form").requestSubmit();
    await until(
      () => !get("create-dialog").open && get("total").textContent === "1",
      "create through real binding",
    );
    const state = await window.go.desktop.App.Snapshot();
    if (state.rows[0].name !== "ui-smoke" || state.rows[0].status !== "stopped")
      throw Error("Incorrect native store state");
    get("search").value = "missing";
    get("search").dispatchEvent(new Event("input"));
    if (get("empty").hidden || get("rows").children.length)
      throw Error("Search did not filter");
    get("search").value = "";
    get("search").dispatchEvent(new Event("input"));
    get("rows").querySelector(".session-identity").click();
    if (get("detail-title").textContent !== "ui-smoke")
      throw Error("Detail mismatch");
    get("delete-button").click();
    get("delete-confirmation").value = "wrong";
    get("delete-confirmation").dispatchEvent(new Event("input"));
    if (!get("delete-submit").disabled)
      throw Error("Unsafe delete confirmation");
    get("delete-confirmation").value = "ui-smoke";
    get("delete-confirmation").dispatchEvent(new Event("input"));
    get("delete-form").requestSubmit();
    await until(
      () => !get("delete-dialog").open && get("total").textContent === "0",
      "delete confirmed test session",
    );
    if (!get("error-banner").hidden) throw Error(get("error-text").textContent);
    await window.go.main.Smoke.Finish(
      "PASS: native WebView, bridge, create, search, details, delete confirmation, delete, empty state",
    );
  } catch (err) {
    if (window.go?.main?.Smoke)
      await window.go.main.Smoke.Finish("FAIL: " + err.message);
  }
})();
