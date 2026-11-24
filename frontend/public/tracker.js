(() => {
  const script = document.currentScript;
  const site = script?.dataset.siteId;
  if (!site || !script.src || window.Saturnita) return;
  const endpoint = new URL("/api/v1/collect", script.src);
  endpoint.searchParams.set("site_id", site);
  function send(type, name, properties = {}) {
    try {
      const query = new URLSearchParams(location.search);
      const utm = {};
      for (const key of ["source", "medium", "campaign"]) {
        const value = query.get("utm_" + key);
        if (value) utm[key] = value.slice(0, 255);
      }
      const body = JSON.stringify({
        event_id: crypto.randomUUID(),
        site_id: site,
        type,
        name,
        occurred_at: new Date().toISOString(),
        path: location.pathname,
        referrer: document.referrer ? new URL(document.referrer).origin : "",
        utm,
        properties,
      });
      if (body.length <= 32768)
        fetch(endpoint, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body,
          credentials: "omit",
          referrerPolicy: "no-referrer",
          keepalive: true,
        }).catch(() => {});
    } catch {
      /* Analytics must not interrupt the host page. */
    }
  }
  window.Saturnita = {
    track(name, properties) {
      send("custom", name, properties);
    },
  };
  let previous;
  function visit() {
    if (location.href !== previous) {
      previous = location.href;
      send("pageview", "pageview");
    }
  }
  for (const method of ["pushState", "replaceState"]) {
    const original = history[method];
    history[method] = function (...args) {
      const result = original.apply(this, args);
      visit();
      return result;
    };
  }
  addEventListener("popstate", visit);
  addEventListener("hashchange", visit);
  visit();
})();
