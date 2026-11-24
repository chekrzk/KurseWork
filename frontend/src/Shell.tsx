import {useState,type ReactNode} from "react";
import {Link,NavLink} from "react-router-dom";
import {LayoutDashboard,Settings,LogOut,ArrowUpRight,Menu} from "lucide-react";
import {Brand,Notice} from "./ui";
import {api,type User} from "./api";
export function Shell({
  user,
  end,
  children,
  demo = false,
}: {
  user?: User;
  end: () => void;
  children: ReactNode;
  demo?: boolean;
}) {
  const [menu, setMenu] = useState(false),
    [msg, setMsg] = useState("");
  return (
    <div className="shell">
      <aside className={menu ? "open" : ""}>
        <Brand />
        <nav onClick={() => setMenu(false)}>
          <NavLink end to={demo ? "/demo" : "/app"}>
            <LayoutDashboard />
            Обзор
          </NavLink>
          {!demo && (
            <>{[["links","Ссылки"],["analytics","Аналитика"],["domains","Домены"],["sites","Сайты"],["monitors","Мониторинг"]].map(([path,name])=><NavLink key={path} to={"/app/"+path}>{name}</NavLink>)}</>
          )}
          {!demo && (
            <NavLink to="/app/settings">
              <Settings />
              Настройки
            </NavLink>
          )}
        </nav>
        <footer>
          {demo ? (
            <Link to="/login">
              Войти <ArrowUpRight />
            </Link>
          ) : (
            <>
              <b>{user?.display_name}</b>
              <button
                className="logout"
                onClick={async () => {
                  try {
                    await api(
                      "/auth/logout",
                      "POST",
                      undefined,
                      user?.csrf_token,
                    );
                    end();
                  } catch (e) {
                    setMsg(String(e));
                  }
                }}
              >
                <LogOut />
                Выйти
              </button>
            </>
          )}
        </footer>
      </aside>
      <main className="workspace">
        <div className="top">
          <button
            className="mobile"
            aria-expanded={menu}
            aria-label="Навигация"
            onClick={() => setMenu(!menu)}
          >
            <Menu />
          </button>
          <span>Рабочее пространство</span>
          <span className="signal">● Pulse</span>
        </div>
        <div className="content">
          <Notice text={msg} />
          {children}
        </div>
      </main>
    </div>
  );
}
