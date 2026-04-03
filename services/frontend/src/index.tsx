import { render } from "solid-js/web";
import { Router, Route, Navigate } from "@solidjs/router";
import { Component, JSX } from "solid-js";
import { isLoggedIn } from "./lib/auth";
import "./app.css";

import LoginPage from "./routes/login";
import Overview from "./routes/index";
import QueryMetrics from "./routes/queries";
import CachePage from "./routes/cache";
import DNSSECPage from "./routes/dnssec";
import SystemPage from "./routes/system";
import UpstreamsPage from "./routes/upstreams";
import QueryLogs from "./routes/logs";
import AlertsPage from "./routes/alerts";
import SettingsPage from "./routes/settings";

// Auth guard wrapper
function Protected(props: { component: Component }): JSX.Element {
  if (!isLoggedIn()) {
    return <Navigate href="/login" />;
  }
  return <props.component />;
}

render(
  () => (
    <Router>
      <Route path="/login" component={LoginPage} />
      <Route path="/" component={() => <Protected component={Overview} />} />
      <Route path="/queries" component={() => <Protected component={QueryMetrics} />} />
      <Route path="/cache" component={() => <Protected component={CachePage} />} />
      <Route path="/dnssec" component={() => <Protected component={DNSSECPage} />} />
      <Route path="/system" component={() => <Protected component={SystemPage} />} />
      <Route path="/upstreams" component={() => <Protected component={UpstreamsPage} />} />
      <Route path="/logs" component={() => <Protected component={QueryLogs} />} />
      <Route path="/alerts" component={() => <Protected component={AlertsPage} />} />
      <Route path="/settings" component={() => <Protected component={SettingsPage} />} />
    </Router>
  ),
  document.getElementById("app")!
);
