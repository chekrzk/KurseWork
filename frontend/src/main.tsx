import {useState,useEffect,type ReactNode} from "react";
import {BrowserRouter,Routes,Route,Navigate} from "react-router-dom";
import { createRoot } from "react-dom/client";
import "@fontsource-variable/manrope";
import "@fontsource/space-grotesk/latin-600.css";
import "@fontsource/jetbrains-mono/cyrillic-400.css";
import "./style.css";
import "./product.css";
import {api,ApiError,type User} from "./api";
import {demoEnabled} from "./config";
import {Auth} from "./Auth";
import {Overview} from "./Overview";
import {SettingsPage} from "./SettingsPage";
import {Shell} from "./Shell";
import {Links,LinkReport} from "./Links";
import {Domains} from "./Domains";
import {Sites,SiteReport} from "./Sites";
import {ReportView} from "./Reports";
import {Monitoring,MonitorDetail} from "./Monitoring";
import {Landing} from "./Landing";
function App() {
  const [user, setUser] = useState<User | null>(),
    [error, setError] = useState("");
  async function refresh() {
    setError("");
    try {
      setUser(await api<User>("/me"));
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) setUser(null);
      else {
        setError("Не удалось загрузить профиль");
        throw e;
      }
    }
  }
  useEffect(() => {
    refresh().catch(() => {});
    const end = () => setUser(null);
    window.addEventListener("session-ended", end);
    return () => window.removeEventListener("session-ended", end);
  }, []);
  function protect(child: ReactNode) {
    return error ? (
      <main>
        {error}{" "}
        <button onClick={() => refresh().catch(() => {})}>Повторить</button>
      </main>
    ) : user === undefined ? (
      <main className="skeleton" role="status">
        Загрузка кабинета…
      </main>
    ) : !user ? (
      <Navigate replace to="/login" />
    ) : (
      <Shell user={user} end={() => setUser(null)}>
        {child}
      </Shell>
    );
  }
  return (
    <Routes>
      <Route path="/" element={user?<Navigate replace to="/app"/>:<Landing/>}/>
      <Route path="/app/monitors" element={protect(user&&<Monitoring user={user}/>)} />
      <Route path="/app/monitors/:id" element={protect(user&&<MonitorDetail user={user}/>)} />
      {["login", "register"].map((path) => (
        <Route
          key={path}
          path={"/" + path}
          element={
            user ? (
              <Navigate to="/app" replace />
            ) : (
              <Auth
                key={path}
                register={path === "register"}
                refresh={refresh}
              />
            )
          }
        />
      ))}
      <Route path="/app" element={protect(<Overview />)} />
      <Route path="/app/links" element={protect(user&&<Links user={user}/>)} />
      <Route path="/app/links/:id" element={protect(<LinkReport/>)} />
      <Route path="/app/analytics" element={protect(<ReportView title="Аналитика ссылок" path="/link-analytics"/>)} />
      <Route path="/app/domains" element={protect(user&&<Domains user={user}/>)} />
      <Route path="/app/sites" element={protect(user&&<Sites user={user}/>)} />
      <Route path="/app/sites/:id" element={protect(<SiteReport/>)} />
      <Route
        path="/app/settings"
        element={protect(
          user && (
            <SettingsPage
              user={user}
              refresh={refresh}
              end={() => setUser(null)}
            />
          ),
        )}
      />
      {demoEnabled && (
        <Route
          path="/demo"
          element={
            <Shell demo end={() => {}}>
              <Overview demo />
            </Shell>
          }
        />
      )}
      <Route path="*" element={<Navigate replace to="/app" />} />
    </Routes>
  );
}
createRoot(document.getElementById("root")!).render(
  <BrowserRouter>
    <App />
  </BrowserRouter>,
);
